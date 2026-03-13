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
				Message: fmt.Sprintf("Phase '%s' does not allow file modifications outside %s/. Advance to Implement by submitting gates for each phase.", s.Phase, datadir.Name()),
			}
		}
	}

	return HookResult{}
}

// WorktreeGuard blocks the lead session from cd'ing into quest worktrees.
// It is called when no quest state file exists (indicating this is the lead session).
// Scoped commands (cd path && ...) are allowed since CWD doesn't persist between
// Bash tool calls.
func WorktreeGuard(input *HookInput) HookResult {
	if input == nil {
		return HookResult{}
	}
	cmd := strings.TrimSpace(input.ToolInput.Command)
	if cmd == "" {
		return HookResult{}
	}

	if isWorktreeCD(cmd) {
		return HookResult{
			Block:   true,
			Message: "Gandalf must not cd into quest worktrees. Use --dir <path> for fellowship commands, or absolute paths for reading files.",
		}
	}
	return HookResult{}
}

// isWorktreeCD detects bare "cd <worktree>" or "pushd <worktree>" commands.
// Scoped commands like "cd <worktree> && <cmd>" are allowed.
func isWorktreeCD(command string) bool {
	fields := strings.Fields(command)
	if len(fields) < 2 {
		return false
	}

	verb := fields[0]
	if verb != "cd" && verb != "pushd" {
		return false
	}

	target := fields[1]
	if !isWorktreePath(target) {
		return false
	}

	// If there's anything after the target, it's a scoped command — allow it.
	// e.g., "cd path && git log" has fields[2] == "&&"
	if len(fields) > 2 {
		return false
	}
	return true
}

// isWorktreePath checks if a path leads into the worktree directory.
func isWorktreePath(path string) bool {
	normalized := filepath.ToSlash(filepath.Clean(path))
	return strings.Contains(normalized, "/.claude/worktrees/") ||
		strings.HasPrefix(normalized, ".claude/worktrees/")
}

// isFellowshipEscapeCommand returns true for fellowship CLI commands that are
// safe to execute even when gate_pending is true. These are commands that
// operate on state/metadata files, not source code.
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
	// Allowlist of subcommands safe to run during gate_pending.
	allowed := map[string]bool{
		"gate":    true, // approve/reject gates
		"init":    true, // reset state file
		"autopsy":  true, // write/read failure records
		"bulletin": true, // read/write shared discovery board
		"errand":   true, // read/update errand status
		"status":  true, // read-only status scan
		"eagles":  true, // read-only health scan
		"tome":    true, // read-only quest history
		"herald":  true, // read-only event log
		"version": true, // print version
	}
	return allowed[fields[1]]
}

