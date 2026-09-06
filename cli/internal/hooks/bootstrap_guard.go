package hooks

import (
	"fmt"
	"strings"

	"github.com/justinjdev/fellowship/cli/internal/datadir"
)

// BootstrapGuard decides a tool call for a registered quest that has no state
// row yet — the window between the lead registering the worktree and the
// teammate running `fellowship init`.
//
// The window exists so init can run, not so the lifecycle can be skipped: a
// quest that never runs init has no phase, no gates and no history, and
// nothing could ever be enforced against it. So, until the row exists:
//
//   - Edit/Write/NotebookEdit may touch the data directory (a plan-driven
//     quest copies its plan there before `init --plan-skip`), never the store,
//     and nothing else.
//   - Bash may run the fellowship CLI (minus the lead's `state` commands),
//     read-only git, and a few read-only shell builtins — enough for the
//     isolation self-check and init — with no output redirection. Anything
//     that could write a file or commit waits for init.
//
// questName is used only in the messages.
func BootstrapGuard(input *HookInput, questName, dataDirName string) HookResult {
	if input == nil {
		return HookResult{}
	}
	if dataDirName == "" {
		dataDirName = datadir.Name()
	}
	initHint := fmt.Sprintf("Quest %s has no state yet — run \"fellowship init --dir <worktree>\" first. The quest lifecycle starts there; nothing is skipped.", questName)

	if p := TargetPath(input); p != "" {
		if r := StoreWriteGuard(input); r.Block {
			return r
		}
		if datadir.IsPathIn(p, dataDirName) {
			return HookResult{}
		}
		return HookResult{Block: true, Message: "fellowship: no file writes before init. " + initHint}
	}

	cmd := input.ToolInput.Command
	if strings.TrimSpace(cmd) == "" {
		return HookResult{}
	}
	for _, inv := range LeadOnlyCommands(cmd) {
		if inv.Subcommand == "state" {
			return HookResult{Block: true, Message: fmt.Sprintf("\"fellowship state %s\" is a lead command and this is a quest worktree. Ask the lead to run it.", strings.TrimSpace(inv.Detail))}
		}
	}
	for _, tokens := range commandInvocations(cmd, 0) {
		if !bootstrapInvocationAllowed(tokens) {
			return HookResult{Block: true, Message: fmt.Sprintf("fellowship: %q is not allowed before init — only the fellowship CLI, read-only git, and read-only shell builtins run in this window. %s", strings.Join(tokens, " "), initHint)}
		}
	}
	return HookResult{}
}

// bootstrapInvocationAllowed reports whether one shell invocation may run
// before the quest has a state row.
func bootstrapInvocationAllowed(tokens []string) bool {
	if len(tokens) == 0 {
		return true
	}
	for _, t := range tokens {
		if strings.HasPrefix(t, ">") || strings.HasPrefix(t, "&>") || strings.Contains(t, ">>") {
			return false
		}
	}
	first := tokens[0]
	if isFellowshipBinary(first) {
		return true
	}
	base := first
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	if base == "git" {
		return gitReadOnly(tokens[1:])
	}
	switch base {
	case "cd", "pwd", "ls", "cat", "echo", "printf", "head", "tail", "grep", "find", "wc",
		"test", "[", "true", "stat", "readlink", "dirname", "basename", "which", "realpath", "sleep":
		return true
	}
	return false
}

// gitReadOnly reports whether a git argument list is one of the inspection
// forms the isolation self-check and a fresh quest legitimately run.
func gitReadOnly(args []string) bool {
	for i, a := range args {
		if a == "-C" || a == "--git-dir" || a == "--work-tree" {
			continue
		}
		if i > 0 && (args[i-1] == "-C" || args[i-1] == "--git-dir" || args[i-1] == "--work-tree") {
			continue
		}
		if strings.HasPrefix(a, "-") {
			continue
		}
		switch a {
		case "rev-parse", "status", "log", "diff", "show", "branch", "symbolic-ref", "ls-files", "worktree", "remote", "config":
			if a == "worktree" || a == "remote" || a == "config" {
				// Only their listing forms.
				return len(args) > i+1 && (args[i+1] == "list" || args[i+1] == "-v" || args[i+1] == "--get" || args[i+1] == "--list")
			}
			return true
		default:
			return false
		}
	}
	return false
}
