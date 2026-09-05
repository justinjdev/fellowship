// Hook dispatch: the subcommands Claude Code calls, and the fail-open
// worktree-guard backstop that runs alongside them.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/justinjdev/fellowship/cli/internal/datadir"
	"github.com/justinjdev/fellowship/cli/internal/db"
	"github.com/justinjdev/fellowship/cli/internal/events"
	"github.com/justinjdev/fellowship/cli/internal/fellowship"
	"github.com/justinjdev/fellowship/cli/internal/gate"
	"github.com/justinjdev/fellowship/cli/internal/gitutil"
	"github.com/justinjdev/fellowship/cli/internal/hooks"
	"github.com/justinjdev/fellowship/cli/internal/state"
)

// gateHooks are the hooks that carry enforcement. They fail CLOSED: if the
// store exists but cannot be read, the safe answer is to block, because the
// alternative is silently skipping gate, hold, and phase checks. worktree-guard
// is deliberately absent — it is defense-in-depth behind lead-provisioned
// isolation and fails open.
var gateHooks = map[string]bool{
	"gate-guard":       true,
	"gate-submit":      true,
	"gate-prereq":      true,
	"completion-guard": true,
	"metadata-track":   true,
	"file-track":       true,
}

func isGateHook(name string) bool { return gateHooks[name] }

// hookDBTimeout bounds every hook's database work. Claude Code kills a hook at
// 5s with no exit code of its own; deadlining well inside that turns lock
// contention into a decision we control (fail closed for gate hooks) instead of
// a killed process.
const hookDBTimeout = 2 * time.Second

func runHook(d *db.DB, name string) int {
	cwd, _ := os.Getwd()
	return runHookWith(name, os.Stdin, cwd, d)
}

// runHookWith is runHook's testable core: it takes the hook name, the payload
// reader, the working directory, and an open store, and returns the exit code.
func runHookWith(name string, stdin io.Reader, cwd string, d *db.DB) int {
	ctx, cancel := context.WithTimeout(context.Background(), hookDBTimeout)
	defer cancel()
	gitRoot := gitRootFrom(cwd)

	// Worktree isolation guard: self-contained, independent of quest state.
	// Runs in teammate sessions (inherited via project settings), so it must
	// not depend on quest lookup keyed to this worktree.
	if name == "worktree-guard" {
		return runWorktreeGuard(ctx, d, cwd, stdin)
	}

	// Find quest name for this worktree.
	var questName string
	var lookupErr error
	if err := d.WithConn(ctx, func(conn *db.Conn) error {
		var err error
		questName, err = state.FindQuest(conn, gitRoot)
		return err
	}); err != nil {
		lookupErr = err
	}
	if questName == "" {
		if lookupErr != nil {
			// DB error — fail closed for safety.
			fmt.Fprintf(os.Stderr, "fellowship: quest lookup failed: %v\n", lookupErr)
			return 2
		}
		// "No quest for this worktree" means one of two very different things.
		// In the main repo root it is the lead session, and only the CWD guard
		// applies. In some OTHER worktree of a repo running a fellowship it
		// means a teammate is somewhere no quest is registered — enforcement
		// cannot be evaluated there, so gate hooks block instead of waving it
		// through as if it were the lead.
		if isGateHook(name) && unregisteredQuestWorktree(ctx, d, cwd, gitRoot) {
			fmt.Fprintf(os.Stderr, "fellowship: worktree %s has no registered quest while a fellowship is running — blocking for safety. The lead can register it with \"fellowship state update-quest --name <quest> --worktree %s\".\n", gitRoot, gitRoot)
			return 2
		}
		if name == "gate-guard" {
			input, err := hooks.ParseInput(stdin)
			if err != nil {
				input = &hooks.HookInput{}
			}
			// No quest row here, but the store is nobody's to hand-edit —
			// not the lead's either.
			if result := hooks.StoreWriteGuard(input); result.Block {
				fmt.Fprintln(os.Stderr, result.Message)
				return 2
			}
			// Only enumerate worktrees when the command looks like a cd/pushd,
			// so the extra git call is off the common hot path.
			var worktrees []string
			if c := strings.TrimSpace(input.ToolInput.Command); strings.HasPrefix(c, "cd ") || strings.HasPrefix(c, "pushd ") {
				worktrees = listQuestWorktrees(cwd)
			}
			if result := hooks.WorktreeGuard(input, hooks.CanonicalPath(cwd), worktrees); result.Block {
				fmt.Fprintln(os.Stderr, result.Message)
				return 2
			}
		}
		return 0
	}

	input, err := hooks.ParseInput(stdin)
	if err != nil {
		switch name {
		case "gate-guard":
			input = &hooks.HookInput{}
		default:
			fmt.Fprintln(os.Stderr, "fellowship: malformed hook input — blocking for safety")
			return 2
		}
	}

	// Dispatch. Hooks that only decide take a plain connection; hooks that
	// record state do their read and write in one transaction. Every branch
	// funnels database errors through hookDBExit so the bootstrap window (no
	// quest row yet) is allowed and everything else fails closed.
	switch name {
	case "gate-guard":
		var result hooks.HookResult
		if err := d.WithConn(ctx, func(conn *db.Conn) error {
			s, err := loadQuestState(conn, questName)
			if err != nil {
				return err
			}
			result = hooks.GateGuard(s, input, hooks.GuardParams{
				LeadSessionID: hookLeadSessionID(conn, cwd),
			})
			return nil
		}); err != nil {
			return hookDBExit(err)
		}
		if result.Block {
			fmt.Fprintln(os.Stderr, result.Message)
			return 2
		}
		return 0

	case "gate-prereq":
		// Recording-only: GatePrereq never blocks, it just marks lembas done.
		if err := d.WithTx(ctx, func(conn *db.Conn) error {
			s, err := loadQuestState(conn, questName)
			if err != nil {
				return err
			}
			if !hooks.GatePrereq(s, input) {
				return nil
			}
			if err := state.Upsert(conn, s); err != nil {
				return err
			}
			return events.Record(conn, events.Event{
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Quest:     questName,
				Type:      events.LembasCompleted,
				Phase:     s.Phase,
				Detail:    "Lembas skill completed",
			})
		}); err != nil {
			return hookDBExit(err)
		}
		return 0

	case "completion-guard":
		var result hooks.HookResult
		if err := d.WithTx(ctx, func(conn *db.Conn) error {
			s, err := loadQuestState(conn, questName)
			if err != nil {
				return err
			}
			result = hooks.CompletionGuard(s, input)
			if !result.Block && input.ToolInput.Status == "completed" {
				if err := hooks.MarkHistoryCompleted(conn, questName); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return hookDBExit(err)
		}
		if result.Block {
			fmt.Fprintln(os.Stderr, result.Message)
			return 2
		}
		return 0

	case "file-track":
		if err := d.WithTx(ctx, func(conn *db.Conn) error {
			s, err := loadQuestState(conn, questName)
			if err != nil {
				return err
			}
			hooks.FileTrack(conn, s, input, questName)
			return nil
		}); err != nil {
			return hookDBExit(err)
		}
		return 0

	case "gate-submit":
		var result hooks.HookResult
		var gateSubmitEnrich bool
		if err := d.WithTx(ctx, func(conn *db.Conn) error {
			s, err := loadQuestState(conn, questName)
			if err != nil {
				return err
			}
			sr := hooks.GateSubmit(s, input)
			result = hooks.HookResult{Block: sr.Block, Message: sr.Message}
			if sr.StateChanged {
				if err := state.Upsert(conn, s); err != nil {
					return err
				}
				if !sr.Block {
					gateSubmitEnrich = true
					if err := hooks.RecordGateSubmitted(conn, questName, sr.PrevPhase); err != nil {
						return err
					}
					if err := events.Record(conn, events.Event{
						Timestamp: time.Now().UTC().Format(time.RFC3339),
						Quest:     questName,
						Type:      events.GateSubmitted,
						Phase:     s.Phase,
						Detail:    "Gate submitted for review",
					}); err != nil {
						return err
					}
					// An auto-approved gate is recorded exactly as the lead's
					// `gate approve` records one, so the history and events
					// look the same whoever (or whatever) approved it.
					if sr.AutoApproved {
						if err := gate.RecordApproval(conn, questName, sr.PrevPhase, sr.NextPhase, "Auto-approved by gates.autoApprove"); err != nil {
							return err
						}
					}
				}
			}
			return nil
		}); err != nil {
			return hookDBExit(err)
		}
		if result.Block {
			// The deny travels in the JSON, so a failed encode would let the
			// gate through silently. Fall back to the plain block channel.
			if err := json.NewEncoder(os.Stdout).Encode(hooks.NewDenyOutput(result.Message)); err != nil {
				fmt.Fprintf(os.Stderr, "fellowship: could not emit the gate denial (%v) — blocking\n%s\n", err, result.Message)
				return 2
			}
			return 0 // exit 0 with JSON deny — Claude Code reads the JSON
		}
		if gateSubmitEnrich {
			// Enrichment is a nicety on an allowed gate: report failures, but
			// never turn one into a block.
			var enrichment string
			if err := d.WithConn(ctx, func(conn *db.Conn) error {
				enrichment = hooks.GatherEnrichment(conn, questName, gitRoot)
				return nil
			}); err != nil {
				fmt.Fprintf(os.Stderr, "fellowship: gate enrichment unavailable: %v\n", err)
			}
			if enrichment != "" {
				enrichedContent := input.ToolInput.Content + enrichment
				out := hooks.NewAllowOutput(map[string]string{"content": enrichedContent})
				if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {
					fmt.Fprintf(os.Stderr, "fellowship: could not emit gate enrichment: %v\n", err)
				}
			}
		}
		return 0

	case "metadata-track":
		if err := d.WithTx(ctx, func(conn *db.Conn) error {
			s, err := loadQuestState(conn, questName)
			if err != nil {
				return err
			}
			if !hooks.MetadataTrack(s, input) {
				return nil
			}
			if err := state.Upsert(conn, s); err != nil {
				return err
			}
			return events.Record(conn, events.Event{
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Quest:     questName,
				Type:      events.MetadataUpdated,
				Phase:     s.Phase,
				Detail:    "Task metadata updated",
			})
		}); err != nil {
			return hookDBExit(err)
		}
		return 0

	default:
		fmt.Fprintf(os.Stderr, "fellowship: unknown hook %q\n", name)
		return 2
	}
}

// errQuestRowMissing reports a quest_state row that is gone from under a quest
// that has already done something. Unlike the bootstrap window this fails
// closed: see loadQuestState.
var errQuestRowMissing = errors.New("quest state row missing")

// loadQuestState loads a quest's row, telling the bootstrap window apart from a
// row that was deleted.
//
// A missing row has to allow during bootstrap — the lead has registered the
// worktree, the teammate has not run `fellowship init` yet, and blocking would
// deadlock the quest before it starts. But "no row" was also the state a
// teammate could manufacture to shake off a pending gate. A quest that has
// submitted a gate or logged an event is past its bootstrap, so a row missing
// under it is a destroyed row and blocks.
func loadQuestState(conn *db.Conn, questName string) (*state.State, error) {
	s, err := state.Load(conn, questName)
	if !errors.Is(err, state.ErrNotFound) {
		return s, err
	}
	hasHistory, histErr := hooks.QuestHasHistory(conn, questName)
	if histErr != nil {
		return nil, histErr // unknown — fail closed
	}
	if hasHistory {
		return nil, fmt.Errorf("%w: %s", errQuestRowMissing, questName)
	}
	return nil, err
}

// hookDBExit maps a hook's database error to an exit code.
//
// state.ErrNotFound is the bootstrap window (see loadQuestState): a missing
// quest row on a quest with no history allows, so the teammate can run the very
// command that creates the row. Every other error is a real failure and fails
// closed.
func hookDBExit(err error) int {
	if errors.Is(err, errQuestRowMissing) {
		fmt.Fprintln(os.Stderr, `fellowship: quest state row missing — run "fellowship init --quest <name>" from the lead. Blocking for safety.`)
		return 2
	}
	if errors.Is(err, state.ErrNotFound) {
		fmt.Fprintln(os.Stderr, `fellowship: no quest state for this worktree yet — allowing so the quest can be started with "fellowship init".`)
		return 0
	}
	fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
	return 2
}

// hookLeadSessionID resolves the session id recorded for the fellowship's lead
// in the repo containing cwd. The store is the authority; the legacy marker
// file is a one-release fallback consulted only when the store names no lead.
// Any failure to resolve it reads as "unknown", which every guard treats as
// "the writer cannot be identified" rather than as a licence to act.
func hookLeadSessionID(conn *db.Conn, cwd string) string {
	mainRoot, err := gitutil.MainRepoRoot(cwd)
	if err != nil {
		return ""
	}
	return state.LeadSessionID(conn, mainRoot, datadir.Resolve(mainRoot))
}

// unregisteredQuestWorktree reports whether cwd sits in a git worktree that is
// NOT the main repo root while a fellowship is actually RUNNING in that repo's
// store. That combination means a teammate is running somewhere no quest was
// ever registered (a stale or mistyped path, a tree created outside the
// fellowship), so gate hooks must not treat it as a lead session. Anything it
// cannot determine reads as false: the lead session in the main tree must
// never be blocked by this.
//
// "Running" is fellowshipRunning's definition, the same one worktree-guard
// arms itself with — a live quest with a worktree on disk. The fellowship row
// is never deleted, so "a row exists" is sticky: it made every linked worktree
// of the repo unusable forever after one `state init`, long after the last
// quest had merged.
func unregisteredQuestWorktree(ctx context.Context, d *db.DB, cwd, gitRoot string) bool {
	mainRoot, err := gitutil.MainRepoRoot(cwd)
	if err != nil {
		return false
	}
	if hooks.CanonicalPath(mainRoot) == hooks.CanonicalPath(gitRoot) {
		return false // the main tree — this really is the lead session
	}
	running := false
	if err := d.WithConn(ctx, func(conn *db.Conn) error {
		fs, err := fellowship.LoadFellowship(conn)
		if err != nil {
			return nil
		}
		running = fellowshipRunning(fs)
		return nil
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: could not check for a running fellowship: %v\n", err)
		return false
	}
	return running
}

// runWorktreeGuard is the fail-OPEN backstop that keeps quest teammates from
// writing source into the MAIN working tree during an active fellowship. It is
// defense-in-depth behind lead-created `isolation: "worktree"`, so any internal
// resolution failure allows the action (exit 0) rather than hard-blocking
// unrelated work. Only a positive mis-placement detection blocks (exit 2).
func runWorktreeGuard(ctx context.Context, d *db.DB, cwd string, stdin io.Reader) int {
	input, err := hooks.ParseInput(stdin)
	if err != nil {
		return 0 // malformed input — allow (defense-in-depth, not primary gate)
	}

	mainRoot, err := gitutil.MainRepoRoot(cwd)
	if err != nil {
		return 0
	}

	sessionTop := hooks.CanonicalPath(gitRootFrom(cwd))

	// Inert unless a fellowship is actually running in the main repo's store.
	// The same read answers whether this session's top-level is registered as
	// a quest worktree, which is what tells a quest provisioned into the main
	// tree apart from the lead standing in it.
	dataDirName := datadir.Resolve(mainRoot)
	active := false
	sessionIsQuest := false
	leadSessionID := ""
	if err := d.WithConn(ctx, func(conn *db.Conn) error {
		// The lead is read from the store, whatever else this read finds: a
		// fellowship that is not running still has a lead, and reading it here
		// costs nothing extra.
		leadSessionID = state.LeadSessionID(conn, mainRoot, dataDirName)
		fs, err := fellowship.LoadFellowship(conn)
		if err != nil {
			return nil
		}
		active = fellowshipRunning(fs)
		name, err := state.FindQuest(conn, sessionTop)
		if err != nil {
			return err
		}
		sessionIsQuest = name != ""
		return nil
	}); err != nil {
		// Fail open, but say so: the guard is a backstop, not the gate.
		fmt.Fprintf(os.Stderr, "fellowship: worktree-guard: %v — allowing\n", err)
		return 0
	}

	filePath := hooks.TargetPath(input)
	if filePath != "" && !filepath.IsAbs(filePath) {
		filePath = filepath.Join(cwd, filePath)
	}
	filePath = hooks.CanonicalPath(filePath)

	// Canonicalize all paths so symlinked repo roots (e.g. macOS /tmp ->
	// /private/tmp) don't defeat the main-root comparison. `git --show-toplevel`
	// returns a resolved path; the cwd-derived main root does not.
	result := hooks.IsolationGuard(hooks.IsolationParams{
		FellowshipActive:         active,
		MainRoot:                 hooks.CanonicalPath(mainRoot),
		SessionTopLevel:          sessionTop,
		TargetTopLevel:           targetTopLevel(filePath),
		ToolName:                 input.ToolName,
		FilePath:                 filePath,
		DataDirName:              dataDirName,
		SessionID:                input.SessionID,
		LeadSessionID:            leadSessionID,
		SessionIsRegisteredQuest: sessionIsQuest,
	})
	if result.Block {
		fmt.Fprintln(os.Stderr, result.Message)
		return 2
	}
	return 0
}

// targetTopLevel resolves the git working tree a target file belongs to, from
// the file's own path rather than the session's working directory — that is
// what makes a write into the main tree visible when the session sits in a
// worktree. Returns "" when git cannot answer, which the guard reads as
// "unknown" and falls back to the session's own top-level.
//
// The target may not exist yet (a Write creating a file, in a directory that
// does not exist either), so the lookup runs in its nearest existing ancestor.
func targetTopLevel(path string) string {
	if path == "" {
		return ""
	}
	dir := nearestExistingDir(filepath.Dir(path))
	if dir == "" {
		return ""
	}
	root, err := gitutil.TopLevel(dir)
	if err != nil {
		return ""
	}
	return hooks.CanonicalPath(root)
}

// nearestExistingDir walks up from dir to the first directory that exists, or
// "" if it reaches the filesystem root without finding one.
func nearestExistingDir(dir string) string {
	for {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// fellowshipRunning reports whether a fellowship is actually in progress, as
// opposed to merely initialized once. The fellowship DB row is never deleted,
// so "a row exists" is a sticky signal that would block ordinary main-tree
// edits forever after a single `state init`. Quest worktrees, by contrast, are
// created when teammates spawn and removed when their work merges — so a live
// quest worktree on disk is the signal that teammates may currently be running
// and the guard should be armed. A finished (or never-started) fellowship whose
// row lingers has no live worktree and reads as inert.
func fellowshipRunning(fs *fellowship.FellowshipState) bool {
	for _, q := range fs.Quests {
		switch fellowship.QuestEntryStatus(q) {
		case "completed", "cancelled":
			continue
		}
		if q.Worktree == "" {
			continue
		}
		if info, err := os.Stat(q.Worktree); err == nil && info.IsDir() {
			return true
		}
	}
	return false
}
