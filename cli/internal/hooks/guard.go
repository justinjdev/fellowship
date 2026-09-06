package hooks

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/justinjdev/fellowship/cli/internal/datadir"
	"github.com/justinjdev/fellowship/cli/internal/state"
)

type HookResult struct {
	Block   bool
	Message string
}

// GuardParams carries the session-level facts gate-guard needs beyond the
// quest row itself. Resolving them costs a git call and a store read, so the
// caller does it once and hands the result to the pure decision function.
type GuardParams struct {
	// LeadSessionID is the session id recorded for the fellowship's lead, or
	// "" when no lead was recorded. Only the lead may move a quest's phase.
	LeadSessionID string
	// DataDirName is the data directory name configured for the MAIN repo
	// (datadir.Resolve(mainRoot)), whose writes the early-phase rule exempts.
	// It must be resolved from the main root, exactly as the store path is:
	// the project config lives there, and a hook that resolved it from its own
	// worktree read ".fellowship" for a fellowship configured otherwise —
	// exempting the wrong directory and blocking the right one. Empty falls
	// back to the process-wide lookup.
	DataDirName string
}

// dataDirName returns the data directory name to enforce with, falling back to
// the process-wide lookup when the caller did not resolve one.
func (p GuardParams) dataDirName() string {
	if p.DataDirName != "" {
		return p.DataDirName
	}
	return datadir.Name()
}

// IsLeadSession reports whether a session id identifies the fellowship's
// lead. Both ids must be known: an empty id on either side means
// "unidentifiable", never "the lead".
//
// It answers for a SESSION, which is all a CLI command can see (the
// CLAUDE_CODE_SESSION_ID in its environment). A hook payload knows more —
// see IsLeadPayload — and hooks must use that instead: a teammate spawned with
// the Agent tool runs in-process and shares the lead's session id.
func IsLeadSession(sessionID, leadSessionID string) bool {
	return sessionID != "" && sessionID == leadSessionID
}

// IsLeadPayload reports whether a hook payload comes from the lead's own
// conversation: its session id is the recorded lead's AND it carries no agent
// id. A payload with an agent id fired inside a subagent — a quest teammate, a
// scout, palantir, or any other agent the lead spawned — and a subagent is
// never the lead, whatever session id it shares.
func IsLeadPayload(input *HookInput, leadSessionID string) bool {
	if input == nil {
		return false
	}
	return input.AgentID == "" && IsLeadSession(input.SessionID, leadSessionID)
}

func GateGuard(s *state.State, input *HookInput, p GuardParams) HookResult {
	// The escape allowlist is read-only reporting and side-channel bookkeeping:
	// nothing on it can clear a hold or a gate. A held teammate that cannot even
	// run `fellowship status` or record a failure has no way to see why it is
	// stopped or to leave a note about it, so the hold check reads the allowlist
	// too — it is checked before both blocks rather than only before the gate.
	escape := isFellowshipEscapeCommand(input.ToolInput.Command)

	if s.Held && !escape {
		msg := "Quest is held — paused by the lead."
		if s.HeldReason != nil {
			msg += " Reason: " + *s.HeldReason
		}
		msg += " Wait for the lead to unhold before taking any action."
		return HookResult{
			Block:   true,
			Message: msg,
		}
	}

	if s.GatePending && !escape {
		return HookResult{
			Block:   true,
			Message: "Gate pending — waiting for lead approval. Do not take any action until the lead approves your gate.",
		}
	}

	// Lead-only CLI commands. GateGuard runs only where a quest row was
	// resolved — that is, inside a registered quest worktree — so any
	// `fellowship state ...` here is a teammate reaching for the lead's own
	// command set, and `fellowship init --phase/--plan-skip` is a phase move,
	// which is a gate decision. Both are refused before they reach the CLI.
	//
	// Every invocation on the line is judged, not just the first: a no-op
	// `init --phase <current phase>` is waved through below, and stopping at
	// it would hand a teammate everything chained behind it.
	if !IsLeadPayload(input, p.LeadSessionID) {
		for _, inv := range LeadOnlyCommands(input.ToolInput.Command) {
			switch inv.Subcommand {
			case "state":
				return HookResult{
					Block: true,
					Message: fmt.Sprintf(
						"\"fellowship state %s\" is a lead command and this is a quest worktree. `state init` records which session is the lead, so running it here would make this teammate the lead and lock the real one out. Ask the lead to run it.",
						strings.TrimSpace(inv.Detail)),
				}
			case "init":
				// Re-running init for the phase the quest is already in moves
				// nothing, so it is not a gate decision.
				if inv.Detail != s.Phase {
					return HookResult{
						Block: true,
						Message: fmt.Sprintf(
							"Only the lead may move a quest's phase: \"fellowship init\" with --phase/--plan-skip would take this quest from %s to %s without a gate. Submit this phase's gate and wait for the lead instead.",
							s.Phase, inv.Detail),
					}
				}
			}
		}
	}

	// `fellowship complete` ends the quest. The command refuses to run outside
	// Review, or with a gate pending, or while held — and so does the guard,
	// so the invariant holds structurally even if the binary the teammate
	// reaches is not this one. Judged against the same rule the command uses.
	if HasCompleteCommand(input.ToolInput.Command) {
		if result := CompletionCheck(s); result.Block {
			return result
		}
	}

	if result := StoreWriteGuard(input); result.Block {
		return result
	}

	filePath := TargetPath(input)
	if state.IsEarlyPhase(s.Phase) {
		dataDir := p.dataDirName()
		if filePath != "" && !datadir.IsPathIn(filePath, dataDir) {
			return HookResult{
				Block:   true,
				Message: fmt.Sprintf("Phase '%s' does not allow file modifications outside %s/. Submit this phase's gate to advance toward Implement.", s.Phase, dataDir),
			}
		}
	}

	return HookResult{}
}

// StoreWriteGuard refuses an Edit/Write/NotebookEdit aimed at the SQLite store.
//
// The data directory is exempt from the phase write rule — teammates keep
// coordination files there — but the store inside it is not a coordination
// file: it IS the enforcement state, and hand-editing it rewrites a phase,
// clears a gate, or renames the lead. Blocked in every phase, for every
// session; the CLI is the only writer.
func StoreWriteGuard(input *HookInput) HookResult {
	filePath := TargetPath(input)
	if filePath == "" || !datadir.IsStorePath(filePath) {
		return HookResult{}
	}
	return HookResult{
		Block:   true,
		Message: "The fellowship store is not editable by hand — it is the enforcement state itself. Use the fellowship CLI (gate, init, todo, notes) to change it.",
	}
}

// TargetPath returns the file a tool call writes: file_path, or notebook_path
// for NotebookEdit. Empty for tool calls that write no file (Bash, Task, ...).
func TargetPath(input *HookInput) string {
	if input == nil {
		return ""
	}
	if p := input.ToolInput.FilePath; p != "" {
		return p
	}
	return input.ToolInput.NotebookPath
}

// WorktreeGuard blocks the lead session from cd'ing into a quest worktree.
// It is called when no quest state exists for the cwd (indicating the lead
// session). A bare "cd <worktree>"/"pushd <worktree>" would move the lead onto
// quest state and subject it to that quest's gate/hold blocks; scoped commands
// ("cd <path> && <cmd>") are allowed since CWD doesn't persist between Bash
// tool calls.
//
// worktreePaths is the set of live git worktree roots (absolute, main root
// excluded) that the caller enumerates; a cd target equal to or under any of
// them is blocked. This covers lead-provisioned worktrees created OUTSIDE the
// main tree. The legacy ".claude/worktrees" location is always recognized even
// when worktreePaths is empty. cwd resolves relative cd targets; pass "" to
// match on the raw target only.
func WorktreeGuard(input *HookInput, cwd string, worktreePaths []string) HookResult {
	if input == nil {
		return HookResult{}
	}
	cmd := strings.TrimSpace(input.ToolInput.Command)
	if cmd == "" {
		return HookResult{}
	}
	target, ok := bareCDTarget(cmd)
	if !ok {
		return HookResult{}
	}
	if isWorktreeTarget(target, cwd, worktreePaths) {
		return HookResult{
			Block:   true,
			Message: "Gandalf must not cd into quest worktrees. Use --dir <path> for fellowship commands, or absolute paths for reading files.",
		}
	}
	return HookResult{}
}

// bareCDTarget returns the target of an unscoped "cd"/"pushd" command. ok is
// false for non-cd commands and for scoped commands like "cd path && cmd"
// (safe — CWD does not persist between Bash tool calls).
func bareCDTarget(command string) (string, bool) {
	fields := strings.Fields(command)
	if len(fields) != 2 {
		return "", false
	}
	if fields[0] != "cd" && fields[0] != "pushd" {
		return "", false
	}
	target := strings.Trim(fields[1], `"'`)
	target = strings.TrimSuffix(target, ";")
	return target, true
}

// isWorktreeTarget reports whether a bare cd target points into a quest
// worktree — either the legacy ".claude/worktrees" location, or (resolved
// against cwd) a path equal to or under one of the supplied worktree roots.
func isWorktreeTarget(target, cwd string, worktreePaths []string) bool {
	if isLegacyWorktreePath(target) {
		return true
	}
	if len(worktreePaths) == 0 {
		return false
	}
	abs := target
	if !filepath.IsAbs(abs) && cwd != "" {
		abs = filepath.Join(cwd, abs)
	}
	abs = CanonicalPath(abs)
	for _, wt := range worktreePaths {
		wt = CanonicalPath(wt)
		if abs == wt || strings.HasPrefix(abs, wt+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// isLegacyWorktreePath checks the historical ".claude/worktrees" location,
// which predates lead-provisioned out-of-tree worktrees.
func isLegacyWorktreePath(path string) bool {
	normalized := strings.TrimSuffix(filepath.ToSlash(filepath.Clean(path)), "/")
	return normalized == ".claude/worktrees" ||
		strings.HasPrefix(normalized, ".claude/worktrees/") ||
		strings.HasSuffix(normalized, "/.claude/worktrees") ||
		strings.Contains(normalized, "/.claude/worktrees/")
}

// IsStoreUpgradeCommand reports whether a Bash command is one that can bring an
// out-of-date store up to date without being able to change quest state.
//
// Hooks refuse to migrate, so an out-of-date store makes every gate hook block
// until some other invocation runs the schema ladder. gate-guard gates Bash, so
// a blanket refusal would deny the only way out of itself and freeze every
// session in the repo. Every non-hook command opens the store through
// db.OpenExisting, which migrates, so the allowance does not need to name a
// mutating command — and must not. `fellowship init` also RESETS an existing
// quest row, clearing gate_pending; letting it through here would hand a
// gate-blocked teammate its own release the first time a binary upgrade made
// the store stale, at exactly the moment gate-guard cannot read the gate flag
// to refuse it. The escape allowlist is the right set: read-only reporting and
// side-channel bookkeeping, which upgrade the schema on open and can advance
// nothing. Shell metacharacters are rejected there for the same reason —
// nothing may be chained onto the allowance.
func IsStoreUpgradeCommand(command string) bool {
	return isFellowshipEscapeCommand(command)
}

// isFellowshipBinary reports whether a command-line token names the fellowship
// CLI: the bare name or any path ending in it, and the wrapper script the
// plugin ships (fellowship.sh), which execs the same binary.
func isFellowshipBinary(token string) bool {
	base := token
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	return base == "fellowship" || base == "fellowship.sh"
}

// shellFields splits a command line on whitespace that is OUTSIDE quotes,
// keeping a quoted run as part of the token it appears in and dropping the
// quote characters themselves.
//
// It is not a shell parser — escapes, substitutions and variable expansion are
// left alone — but it is exactly enough to tell a command being RUN from a
// command merely NAMED inside an argument. strings.Fields cannot: it splits
// `-m "fellowship init --phase Implement"` into tokens that read as an
// invocation, and the leading-quote heuristic that papered over that also
// skipped a genuinely quoted binary path.
func shellFields(command string) []string {
	var (
		fields []string
		cur    strings.Builder
		inTok  bool
		quote  rune
	)
	flush := func() {
		if inTok {
			fields = append(fields, cur.String())
			cur.Reset()
			inTok = false
		}
	}
	for _, r := range command {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
			inTok = true
		case unicode.IsSpace(r):
			flush()
		default:
			cur.WriteRune(r)
			inTok = true
		}
	}
	flush()
	return fields
}

// PlanSkipPhase is the phase `fellowship init --plan-skip` starts a quest in:
// the plan already exists, so Research and Plan are recorded as skipped.
const PlanSkipPhase = "Implement"

// isFellowshipEscapeCommand returns true for fellowship CLI commands that are
// safe to execute even when gate_pending is true.
//
// A pending gate means "this teammate is waiting on the lead". The allowlist
// therefore must not contain anything that can clear that wait: `fellowship
// gate approve|reject` would let the blocked teammate approve its own gate,
// and `fellowship init` would reset the quest state that carries the pending
// flag. Both were previously allowed, which made the gate advisory rather than
// enforced. What remains is either read-only or side-channel bookkeeping
// (failure records, the notes board, todo status) that cannot advance the
// quest past the gate.
//
// Shell metacharacters are rejected to prevent bypass abuse (e.g., chaining
// a destructive command after fellowship via "&&" or ";").
func isFellowshipEscapeCommand(command string) bool {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" ||
		strings.ContainsAny(trimmed, ";&|<>\n\r`") ||
		strings.Contains(trimmed, "$(") {
		return false
	}
	// shellFields, not strings.Fields, for the same reason InitPhaseRequest
	// uses it: a quoted binary path ("$HOME/.claude/fellowship/bin/fellowship")
	// is one token being RUN, and a command merely NAMED inside a quoted
	// argument is not an invocation at all.
	fields := shellFields(trimmed)
	if len(fields) < 2 {
		return false
	}
	// Accept bare "fellowship" or any path ending in "/fellowship".
	if !isFellowshipBinary(fields[0]) {
		return false
	}
	// Allowlist of subcommands safe to run during gate_pending. Both the
	// current names and their deprecated aliases are listed, since the
	// dispatcher keeps the aliases working for one release.
	allowed := map[string]bool{
		"failures": true, // write/read failure records
		"autopsy":  true, // deprecated alias for failures
		"notes":    true, // read/write shared discovery board
		"bulletin": true, // deprecated alias for notes
		"todo":     true, // read/update todo status
		"errand":   true, // deprecated alias for todo
		"status":   true, // read-only status scan
		"health":   true, // read-only health scan
		"eagles":   true, // deprecated alias for health
		"history":  true, // read-only quest history
		"tome":     true, // deprecated alias for history
		"events":   true, // read-only event log
		"herald":   true, // deprecated alias for events
		"version":  true, // print version
	}
	if allowed[fields[1]] {
		return true
	}
	// `gate` is not allowlisted wholesale — only its read-only reporting form.
	return fields[1] == "gate" && len(fields) > 2 && fields[2] == "status"
}
