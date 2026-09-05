package hooks

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/justinjdev/fellowship/cli/internal/datadir"
	"github.com/justinjdev/fellowship/cli/internal/state"
)

type HookResult struct {
	Block   bool
	Message string
}

func GateGuard(s *state.State, input *HookInput) HookResult {
	if s.Held {
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

	if s.GatePending && !isFellowshipEscapeCommand(input.ToolInput.Command) {
		return HookResult{
			Block:   true,
			Message: "Gate pending — waiting for lead approval. Do not take any action until the lead approves your gate.",
		}
	}

	if state.IsEarlyPhase(s.Phase) {
		filePath := input.ToolInput.FilePath
		if filePath == "" {
			filePath = input.ToolInput.NotebookPath
		}
		if filePath != "" && !datadir.IsDataDirPath(filePath) {
			return HookResult{
				Block:   true,
				Message: fmt.Sprintf("Phase '%s' does not allow file modifications outside %s/. Submit this phase's gate to advance toward Implement.", s.Phase, datadir.Name()),
			}
		}
	}

	return HookResult{}
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
	fields := strings.Fields(trimmed)
	if len(fields) < 2 {
		return false
	}
	// Accept bare "fellowship" or any path ending in "/fellowship".
	bin := fields[0]
	if bin != "fellowship" && !strings.HasSuffix(bin, "/fellowship") {
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
