// The gate commands the lead and teammates drive: gate status/approve/reject,
// hold/unhold, and the quest-level `fellowship init`.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/justinjdev/fellowship/cli/internal/datadir"
	"github.com/justinjdev/fellowship/cli/internal/db"
	"github.com/justinjdev/fellowship/cli/internal/events"
	"github.com/justinjdev/fellowship/cli/internal/gate"
	"github.com/justinjdev/fellowship/cli/internal/gitutil"
	"github.com/justinjdev/fellowship/cli/internal/history"
	"github.com/justinjdev/fellowship/cli/internal/hooks"
	"github.com/justinjdev/fellowship/cli/internal/state"
)

// gateArgs is the parsed form of a `fellowship gate ...` invocation.
type gateArgs struct {
	sub string
	dir string
}

// parseGateArgs parses the gate subcommand and its flags. The FlagSet uses
// ContinueOnError so an unknown flag returns a usage error to the caller
// instead of terminating the process.
func parseGateArgs(args []string) (gateArgs, error) {
	usage := "usage: fellowship gate <status|approve|reject> [--dir <worktree>]"
	if len(args) == 0 {
		return gateArgs{}, errors.New(usage)
	}
	sub := args[0]
	switch sub {
	case "status", "approve", "reject":
	default:
		return gateArgs{}, fmt.Errorf("unknown gate command: %s\n%s", sub, usage)
	}

	fs := flag.NewFlagSet("gate "+sub, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dir := fs.String("dir", "", "Worktree directory (default: current directory)")
	if err := fs.Parse(args[1:]); err != nil {
		return gateArgs{}, fmt.Errorf("gate %s: %v\n%s", sub, err, usage)
	}
	if fs.NArg() > 0 {
		return gateArgs{}, fmt.Errorf("gate %s: unexpected argument %q\n%s", sub, fs.Arg(0), usage)
	}
	return gateArgs{sub: sub, dir: *dir}, nil
}

func runGate(d *db.DB, args []string) int {
	ctx := context.Background()

	ga, err := parseGateArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := checkDir(ga.dir); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	questName := resolveDirQuest(d, ga.dir)
	if questName == "" {
		fmt.Fprintln(os.Stderr, "fellowship: no quest state found")
		return 1
	}

	switch ga.sub {
	case "status":
		var s *state.State
		if err := d.WithConn(ctx, func(conn *db.Conn) error {
			var err error
			s, err = state.Load(conn, questName)
			return err
		}); err != nil {
			fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
			return 1
		}
		fmt.Printf("Phase:    %s\n", s.Phase)
		fmt.Printf("Pending:  %v\n", s.GatePending)
		fmt.Printf("Held:     %v\n", s.Held)
		if s.HeldReason != nil {
			fmt.Printf("Reason:   %s\n", *s.HeldReason)
		}
		fmt.Printf("Lembas:   %v\n", s.LembasCompleted)
		fmt.Printf("Metadata: %v\n", s.MetadataUpdated)
		if s.GateID != nil {
			fmt.Printf("Gate ID:  %s\n", *s.GateID)
		}
		return 0

	case "approve":
		var prevPhase, nextPhase string
		if err := d.WithTx(ctx, func(conn *db.Conn) error {
			s, err := state.Load(conn, questName)
			if err != nil {
				return err
			}
			prev, next, err := state.Approve(s)
			if err != nil {
				return err
			}
			prevPhase, nextPhase = prev, next
			if err := state.Upsert(conn, s); err != nil {
				return err
			}
			return gate.RecordApproval(conn, questName, prevPhase, nextPhase, "")
		}); err != nil {
			fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
			return 1
		}
		fmt.Printf("Gate approved. Phase advanced to %s.\n", nextPhase)
		return 0

	case "reject":
		var phase string
		if err := d.WithTx(ctx, func(conn *db.Conn) error {
			s, err := state.Load(conn, questName)
			if err != nil {
				return err
			}
			if err := state.Reject(s); err != nil {
				return err
			}
			phase = s.Phase
			if err := state.Upsert(conn, s); err != nil {
				return err
			}
			return gate.RecordRejection(conn, questName, phase, "")
		}); err != nil {
			fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
			return 1
		}
		fmt.Println("Gate rejected. Teammate unblocked to address feedback.")
		return 0

	default:
		fmt.Fprintf(os.Stderr, "unknown gate command: %s\n", ga.sub)
		return 1
	}
}

func runHold(d *db.DB, args []string) int {
	ctx := context.Background()
	fs := flag.NewFlagSet("hold", flag.ExitOnError)
	dir := fs.String("dir", "", "Worktree directory (required)")
	reason := fs.String("reason", "", "Reason for holding the quest")
	fs.Parse(args)

	if *dir == "" {
		fmt.Fprintln(os.Stderr, "usage: fellowship hold --dir <worktree> [--reason \"message\"]")
		return 1
	}

	questName, err := resolveHoldQuest(d, *dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	var phase string
	if err := d.WithTx(ctx, func(conn *db.Conn) error {
		s, err := state.Load(conn, questName)
		if err != nil {
			return err
		}
		if s.Held {
			return fmt.Errorf("quest is already held")
		}
		s.Held = true
		if *reason != "" {
			s.HeldReason = reason
		}
		phase = s.Phase
		if err := state.Upsert(conn, s); err != nil {
			return err
		}

		detail := "Quest held"
		if *reason != "" {
			detail += ": " + *reason
		}
		return events.Record(conn, events.Event{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Quest:     questName,
			Type:      events.QuestHeld,
			Phase:     phase,
			Detail:    detail,
		})
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	fmt.Printf("Quest held.%s\n", func() string {
		if *reason != "" {
			return " Reason: " + *reason
		}
		return ""
	}())
	return 0
}

func runUnhold(d *db.DB, args []string) int {
	ctx := context.Background()
	fs := flag.NewFlagSet("unhold", flag.ExitOnError)
	dir := fs.String("dir", "", "Worktree directory (required)")
	fs.Parse(args)

	if *dir == "" {
		fmt.Fprintln(os.Stderr, "usage: fellowship unhold --dir <worktree>")
		return 1
	}

	questName, err := resolveHoldQuest(d, *dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	if err := d.WithTx(ctx, func(conn *db.Conn) error {
		s, err := state.Load(conn, questName)
		if err != nil {
			return err
		}
		if !s.Held {
			return fmt.Errorf("quest is not held")
		}
		s.Held = false
		s.HeldReason = nil
		if err := state.Upsert(conn, s); err != nil {
			return err
		}

		return events.Record(conn, events.Event{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Quest:     questName,
			Type:      events.QuestUnheld,
			Phase:     s.Phase,
			Detail:    "Quest unheld — resumed",
		})
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	fmt.Println("Quest unheld.")
	return 0
}

// resolveHoldQuest finds the quest registered for the --dir of hold/unhold.
// It used to fall back to the directory's basename, which quietly held a quest
// that does not exist (state.Load then failed with a confusing "quest not
// found" naming a directory) or, worse, a same-named quest registered
// elsewhere. An unregistered directory is a user error and now says so.
func resolveHoldQuest(d *db.DB, dir string) (string, error) {
	ctx := context.Background()
	var questName string
	if err := d.WithConn(ctx, func(conn *db.Conn) error {
		var err error
		questName, err = state.FindQuest(conn, dir)
		if err != nil || questName != "" {
			return err
		}
		// Accept a subdirectory or a differently-spelled path by resolving the
		// worktree root, exactly as the hooks do.
		questName, err = state.FindQuest(conn, gitRootFrom(ctx, dir))
		return err
	}); err != nil {
		return "", fmt.Errorf("looking up the quest for %q: %w", dir, err)
	}
	if questName == "" {
		return "", fmt.Errorf("no quest is registered for %q — register it with %q first",
			dir, "fellowship state add-quest --name <quest> --worktree "+dir)
	}
	return questName, nil
}

func runInit(d *db.DB, args []string) int {
	ctx := context.Background()
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	phase := fs.String("phase", "", "Initial phase (default: Research)")
	planSkip := fs.Bool("plan-skip", false, "Record Research/Plan as skipped in history")
	questName := fs.String("quest", "", "Quest name (default: the name registered for this worktree)")
	initDir := fs.String("dir", "", "Worktree or repo root (default: auto-detect via git)")
	fs.Parse(args)

	if err := checkDir(*initDir); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	if *phase != "" && !state.IsValidPhase(*phase) {
		fmt.Fprintf(os.Stderr, "fellowship: invalid phase %q (valid: %s)\n", *phase, strings.Join(state.Phases(), ", "))
		return 1
	}

	if *planSkip && *phase == "" {
		*phase = "Implement"
	}
	if *planSkip && *phase != "Implement" {
		fmt.Fprintln(os.Stderr, "fellowship: --plan-skip requires --phase Implement")
		return 1
	}

	root := *initDir
	if root == "" {
		root = gitRootOrCwd()
	}

	// The quest's own working directory for checkpoints and notes. Its name
	// comes from the MAIN repo's config (that is where the project config
	// lives), so a worktree and the main tree always agree on it.
	mainRoot := root
	if mr, err := gitutil.MainRepoRoot(root); err == nil {
		mainRoot = mr
	}
	dataDir := filepath.Join(root, datadir.Resolve(mainRoot))
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: creating data directory: %v\n", err)
		return 1
	}

	qn := resolveInitQuestName(d, *questName, root)

	initPhase := state.Phases()[0]
	if *phase != "" {
		initPhase = *phase
	}

	// Auto-approved gates come from the merged fellowship config: the main
	// repo's .fellowship/config.json, overridden by ~/.claude/fellowship.json.
	autoApprove := datadir.AutoApproveGates(mainRoot)
	if err := validateAutoApproveGates(autoApprove); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}
	if autoApprove == nil {
		autoApprove = []string{}
	}

	// Who is running this? On a quest row that already exists, --phase and
	// --plan-skip are a phase move, and a phase move is a gate decision: it can
	// take a quest from Research straight to Implement without any gate ever
	// being submitted, and gate-guard waves it through because nothing is
	// pending. Only the lead may do that. On a row being created for the first
	// time the flags are ordinary bootstrap and anyone may pass them.
	callerIsLead, leadKnown := initCallerIsLead(d, mainRoot)
	// Whether --plan-skip's history entry is written: only alongside a phase
	// move that actually happened.
	recordSkipped := true

	if err := d.WithTx(ctx, func(conn *db.Conn) error {
		// Try to load existing state to reset it.
		existing, loadErr := state.Load(conn, qn)
		if loadErr != nil && !errors.Is(loadErr, state.ErrNotFound) {
			return fmt.Errorf("loading quest state: %w", loadErr)
		}
		if loadErr == nil {
			movePhase := true
			if *phase != "" && *phase != existing.Phase && !callerIsLead {
				if leadKnown {
					return fmt.Errorf("only the lead may change the phase of an existing quest (%s is in %s, --phase asked for %s); submit this phase's gate and let the lead approve it",
						qn, existing.Phase, *phase)
				}
				// The lead cannot be identified, so this caller cannot be
				// called a teammate either. Refuse the move, keep going: a
				// reset of the gate flags is always safe.
				fmt.Fprintf(os.Stderr, "fellowship: warning: ignoring --phase/--plan-skip on the existing quest %q — its phase (%s) only moves through a gate\n", qn, existing.Phase)
				movePhase = false
			}
			// Reset existing state: the gate and prerequisite flags go back to
			// their starting values, the phase is kept unless the lead's
			// --phase moves it.
			state.Reset(existing)
			if movePhase && *phase != "" {
				existing.Phase = *phase
			}
			// --plan-skip only ever records history alongside a phase move, so
			// a refused move must not leave the history claiming the phases
			// were skipped.
			recordSkipped = movePhase
			existing.AutoApproveGates = autoApprove
			if err := state.Upsert(conn, existing); err != nil {
				return err
			}
			fmt.Printf("State reset (gate_pending cleared, phase: %s).\n", existing.Phase)
		} else {
			// Create new state.
			s := &state.State{
				QuestName:        qn,
				Phase:            initPhase,
				AutoApproveGates: autoApprove,
			}
			if err := state.Upsert(conn, s); err != nil {
				return err
			}
			fmt.Printf("Quest state created (quest: %s, phase: %s)\n", qn, initPhase)
		}

		if *planSkip && recordSkipped {
			if err := history.RecordSkippedPhases(conn, qn, []string{"Research", "Plan"}, "pre-existing plan"); err != nil {
				return err
			}
			fmt.Println("Recorded Research/Plan as skipped (pre-existing plan).")
		}
		return nil
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: %v\n", err)
		return 1
	}

	return 0
}

// initCallerIsLead answers, for the repo rooted at mainRoot, whether the
// session running this command is the fellowship's recorded lead, and whether a
// lead is recorded at all. It is the same identity check the worktree-guard
// makes at hook time: the session id `fellowship state init` recorded against
// the id Claude Code exports to the commands it runs.
//
// leadKnown is what separates "you are not the lead" (a teammate — refuse) from
// "nobody knows who the lead is" (an old fellowship, or a plain shell — refuse
// the phase move, but do not accuse anyone).
func initCallerIsLead(d *db.DB, mainRoot string) (callerIsLead, leadKnown bool) {
	lead := ""
	if err := d.WithConn(context.Background(), func(conn *db.Conn) error {
		lead = state.LeadSessionID(conn, mainRoot, datadir.Resolve(mainRoot))
		return nil
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fellowship: warning: could not read the recorded lead: %v\n", err)
	}
	return hooks.IsLeadSession(state.CurrentSessionID(), lead), lead != ""
}

// resolveInitQuestName picks the quest name `fellowship init` records: the
// explicit --quest flag, else the name the lead registered for this worktree
// with `state add-quest`, else the directory basename.
func resolveInitQuestName(d *db.DB, flagName, root string) string {
	if flagName != "" {
		return flagName
	}
	if registered := resolveDirQuest(d, root); registered != "" {
		return registered
	}
	return filepath.Base(root)
}

// autoApprovablePhases lists the phases a gate can be auto-approved for: every
// quest phase except the terminal one, which no gate leaves.
func autoApprovablePhases() []string {
	return state.GatePhases()
}

// validateAutoApproveGates rejects gates.autoApprove entries that do not name a
// gate-bearing phase.
func validateAutoApproveGates(gates []string) error {
	valid := autoApprovablePhases()
	for _, g := range gates {
		ok := false
		for _, p := range valid {
			if p == g {
				ok = true
				break
			}
		}
		if !ok {
			return fmt.Errorf("invalid gates.autoApprove entry %q (valid: %s)", g, strings.Join(valid, ", "))
		}
	}
	return nil
}
