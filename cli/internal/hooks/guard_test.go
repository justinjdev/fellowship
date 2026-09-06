package hooks

import (
	"strings"
	"testing"

	"github.com/justinjdev/fellowship/cli/internal/state"
)

func TestGateGuard_AllowsWhenNotPending(t *testing.T) {
	s := &state.State{Phase: "Research", GatePending: false}
	input := &HookInput{ToolInput: ToolInput{Command: "ls"}}
	result := GateGuard(s, input, GuardParams{})
	if result.Block {
		t.Errorf("should allow when not pending, got blocked: %s", result.Message)
	}
}

func TestGateGuard_BlocksWhenPending(t *testing.T) {
	s := &state.State{Phase: "Research", GatePending: true}
	input := &HookInput{ToolInput: ToolInput{Command: "ls"}}
	result := GateGuard(s, input, GuardParams{})
	if !result.Block {
		t.Error("should block when gate pending")
	}
}

func TestGateGuard_BlocksEditDuringEarlyPhase(t *testing.T) {
	for _, phase := range []string{"Research", "Plan"} {
		s := &state.State{Phase: phase}
		input := &HookInput{ToolInput: ToolInput{FilePath: "/repo/src/main.ts"}}
		result := GateGuard(s, input, GuardParams{})
		if !result.Block {
			t.Errorf("should block Edit to prod file during %s", phase)
		}
	}
}

func TestGateGuard_AllowsDataDirWriteDuringEarlyPhase(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // ensure default datadir (.fellowship)
	s := &state.State{Phase: "Research"}
	for _, path := range []string{"/repo/.fellowship/notes.md", ".fellowship/checkpoint.md"} {
		input := &HookInput{ToolInput: ToolInput{FilePath: path}}
		result := GateGuard(s, input, GuardParams{})
		if result.Block {
			t.Errorf("should allow .fellowship/ write during Research: %s", path)
		}
	}
}

func TestGateGuard_AllowsEditDuringLatePhase(t *testing.T) {
	for _, phase := range []string{"Implement", "Review"} {
		s := &state.State{Phase: phase}
		input := &HookInput{ToolInput: ToolInput{FilePath: "/repo/src/main.ts"}}
		result := GateGuard(s, input, GuardParams{})
		if result.Block {
			t.Errorf("should allow Edit during %s", phase)
		}
	}
}

func TestGateGuard_AllowsBashDuringEarlyPhase(t *testing.T) {
	s := &state.State{Phase: "Research"}
	input := &HookInput{ToolInput: ToolInput{Command: "ls"}}
	result := GateGuard(s, input, GuardParams{})
	if result.Block {
		t.Error("should allow Bash during Research")
	}
}

func TestGateGuard_BlocksNotebookEditDuringEarlyPhase(t *testing.T) {
	s := &state.State{Phase: "Research"}
	input := &HookInput{ToolInput: ToolInput{NotebookPath: "/repo/src/analysis.ipynb"}}
	result := GateGuard(s, input, GuardParams{})
	if !result.Block {
		t.Error("should block NotebookEdit to prod file during Research")
	}
}

func TestGateGuard_PendingBlocksEvenDuringLatePhase(t *testing.T) {
	s := &state.State{Phase: "Implement", GatePending: true}
	input := &HookInput{ToolInput: ToolInput{FilePath: "/repo/src/main.ts"}}
	result := GateGuard(s, input, GuardParams{})
	if !result.Block {
		t.Error("gate_pending should block even during Implement")
	}
}

func TestGateGuard_BlocksWhenHeld(t *testing.T) {
	s := &state.State{Phase: "Implement", Held: true}
	input := &HookInput{ToolInput: ToolInput{Command: "ls"}}
	result := GateGuard(s, input, GuardParams{})
	if !result.Block {
		t.Error("should block when quest is held")
	}
}

func TestGateGuard_BlocksWhenHeldWithReason(t *testing.T) {
	reason := "file conflict with quest-auth"
	s := &state.State{Phase: "Implement", Held: true, HeldReason: &reason}
	input := &HookInput{ToolInput: ToolInput{Command: "ls"}}
	result := GateGuard(s, input, GuardParams{})
	if !result.Block {
		t.Error("should block when quest is held")
	}
	if !strings.Contains(result.Message, reason) {
		t.Errorf("message should include held reason, got: %s", result.Message)
	}
}

func TestGateGuard_HeldTakesPriorityOverGatePending(t *testing.T) {
	s := &state.State{Phase: "Implement", Held: true, GatePending: true}
	input := &HookInput{ToolInput: ToolInput{Command: "ls"}}
	result := GateGuard(s, input, GuardParams{})
	if !result.Block {
		t.Error("should block")
	}
	if !strings.Contains(result.Message, "held") {
		t.Errorf("held should take priority over gate_pending, got: %s", result.Message)
	}
}

func TestGateGuard_AllowsAllowlistedFellowshipCommandsWhenPending(t *testing.T) {
	s := &state.State{Phase: "Research", GatePending: true}
	for _, cmd := range []string{
		"fellowship gate status",
		"fellowship gate status --dir /tmp/worktree",
		"fellowship failures create --dir /tmp/repo",
		"fellowship failures scan --dir /tmp/repo --modules auth",
		"fellowship failures infer --dir /tmp/worktree",
		"fellowship todo list --dir .",
		"fellowship status --json",
		"fellowship health --json",
		"fellowship history show",
		"fellowship events --json",
		"fellowship version",
		"~/.claude/fellowship/bin/fellowship gate status",
		"/usr/local/bin/fellowship health",
		// Deprecated aliases stay allowlisted too.
		"fellowship autopsy create --dir /tmp/repo",
		"fellowship errand list --dir .",
		"fellowship eagles --json",
		"fellowship tome show",
		"fellowship herald --json",
	} {
		input := &HookInput{ToolInput: ToolInput{Command: cmd}}
		result := GateGuard(s, input, GuardParams{})
		if result.Block {
			t.Errorf("allowlisted fellowship command should be allowed through gate_pending, cmd=%q got: %s", cmd, result.Message)
		}
	}
}

// A teammate waiting on a gate must not be able to clear its own wait. Both
// `fellowship gate approve|reject` (self-approval) and `fellowship init`
// (resetting the state that holds the pending flag) were once allowlisted,
// which made the gate advisory rather than enforced.
func TestGateGuard_BlocksSelfApprovalWhenPending(t *testing.T) {
	s := &state.State{Phase: "Research", GatePending: true}
	for _, cmd := range []string{
		"fellowship gate approve",
		"fellowship gate reject",
		"fellowship gate approve --dir /tmp/worktree",
		"fellowship gate",
		"fellowship init",
		"fellowship init --phase Implement",
		"~/.claude/fellowship/bin/fellowship gate approve",
		"/usr/local/bin/fellowship init",
	} {
		input := &HookInput{ToolInput: ToolInput{Command: cmd}}
		if result := GateGuard(s, input, GuardParams{}); !result.Block {
			t.Errorf("pending teammate must not be able to run %q", cmd)
		}
	}
}

func TestGateGuard_BlocksNonAllowlistedFellowshipCommandsWhenPending(t *testing.T) {
	s := &state.State{Phase: "Research", GatePending: true}
	for _, cmd := range []string{
		"fellowship state update-quest --name quest-1 --status completed",
		"fellowship hold --dir /tmp/worktree",
		"fellowship unhold --dir /tmp/worktree",
		"fellowship dashboard",
		"fellowship group approve foo",
	} {
		input := &HookInput{ToolInput: ToolInput{Command: cmd}}
		result := GateGuard(s, input, GuardParams{})
		if !result.Block {
			t.Errorf("non-allowlisted fellowship command should be blocked during gate_pending, cmd=%q", cmd)
		}
	}
}

func TestGateGuard_BlocksChainedCommandsWithFellowshipEscape(t *testing.T) {
	s := &state.State{Phase: "Implement", GatePending: true}
	for _, cmd := range []string{
		"rm -rf / && fellowship gate reject",
		"fellowship gate reject; rm -rf /",
		"fellowship gate reject || evil",
		"echo foo | fellowship gate reject",
		"echo fellowship gate reject",      // first token is echo, not fellowship
		"fellowship gate reject\nrm -rf /", // newline-separated second command
		"$(fellowship gate reject)",        // subshell
	} {
		input := &HookInput{ToolInput: ToolInput{Command: cmd}}
		result := GateGuard(s, input, GuardParams{})
		if !result.Block {
			t.Errorf("chained command should be blocked even with fellowship gate reject, cmd=%q", cmd)
		}
	}
}

func TestGateGuard_HeldBlocksFellowshipEscapeCommands(t *testing.T) {
	s := &state.State{Phase: "Implement", Held: true}
	input := &HookInput{ToolInput: ToolInput{Command: "fellowship gate reject"}}
	result := GateGuard(s, input, GuardParams{})
	if !result.Block {
		t.Error("held state should block even fellowship escape commands")
	}
}

func TestWorktreeGuard_BlocksBareCD(t *testing.T) {
	for _, cmd := range []string{
		"cd .claude/worktrees/quest-1",
		"cd /home/user/repo/.claude/worktrees/quest-1",
		"cd .claude/worktrees/quest-1/src",
	} {
		input := &HookInput{ToolInput: ToolInput{Command: cmd}}
		result := WorktreeGuard(input, "", nil)
		if !result.Block {
			t.Errorf("should block bare cd into worktree, cmd=%q", cmd)
		}
	}
}

func TestWorktreeGuard_BlocksPushd(t *testing.T) {
	input := &HookInput{ToolInput: ToolInput{Command: "pushd .claude/worktrees/quest-1"}}
	result := WorktreeGuard(input, "", nil)
	if !result.Block {
		t.Error("should block pushd into worktree")
	}
}

func TestWorktreeGuard_AllowsScopedCD(t *testing.T) {
	for _, cmd := range []string{
		"cd .claude/worktrees/quest-1 && git log",
		"cd .claude/worktrees/quest-1 && go test ./...",
		"cd .claude/worktrees/quest-1 || echo fail",
	} {
		input := &HookInput{ToolInput: ToolInput{Command: cmd}}
		result := WorktreeGuard(input, "", nil)
		if result.Block {
			t.Errorf("should allow scoped cd, cmd=%q", cmd)
		}
	}
}

func TestWorktreeGuard_AllowsNonWorktreeCD(t *testing.T) {
	for _, cmd := range []string{
		"cd /tmp",
		"cd src/auth",
		"cd ..",
	} {
		input := &HookInput{ToolInput: ToolInput{Command: cmd}}
		result := WorktreeGuard(input, "", nil)
		if result.Block {
			t.Errorf("should allow cd to non-worktree path, cmd=%q", cmd)
		}
	}
}

func TestWorktreeGuard_AllowsNonCDCommands(t *testing.T) {
	for _, cmd := range []string{
		"cat .claude/worktrees/quest-1/src/main.go",
		"ls .claude/worktrees/quest-1",
		"grep -r foo .claude/worktrees/quest-1",
	} {
		input := &HookInput{ToolInput: ToolInput{Command: cmd}}
		result := WorktreeGuard(input, "", nil)
		if result.Block {
			t.Errorf("should allow non-cd commands referencing worktrees, cmd=%q", cmd)
		}
	}
}

func TestWorktreeGuard_BlocksWorktreeRoot(t *testing.T) {
	for _, cmd := range []string{
		"cd .claude/worktrees",
		"cd /home/user/repo/.claude/worktrees",
		"pushd .claude/worktrees",
	} {
		input := &HookInput{ToolInput: ToolInput{Command: cmd}}
		result := WorktreeGuard(input, "", nil)
		if !result.Block {
			t.Errorf("should block cd into worktree root, cmd=%q", cmd)
		}
	}
}

func TestWorktreeGuard_BlocksQuotedTarget(t *testing.T) {
	for _, cmd := range []string{
		`cd ".claude/worktrees/quest-1"`,
		`cd '.claude/worktrees/quest-1'`,
	} {
		input := &HookInput{ToolInput: ToolInput{Command: cmd}}
		result := WorktreeGuard(input, "", nil)
		if !result.Block {
			t.Errorf("should block quoted cd into worktree, cmd=%q", cmd)
		}
	}
}

func TestWorktreeGuard_BlocksTrailingSemicolon(t *testing.T) {
	input := &HookInput{ToolInput: ToolInput{Command: "cd .claude/worktrees/quest-1;"}}
	result := WorktreeGuard(input, "", nil)
	if !result.Block {
		t.Errorf("should block cd with trailing semicolon")
	}
}

func TestWorktreeGuard_AllowsEmptyCommand(t *testing.T) {
	input := &HookInput{ToolInput: ToolInput{Command: ""}}
	result := WorktreeGuard(input, "", nil)
	if result.Block {
		t.Error("should allow empty command")
	}
}

func TestWorktreeGuard_AllowsNilInput(t *testing.T) {
	result := WorktreeGuard(nil, "", nil)
	if result.Block {
		t.Error("should allow nil input")
	}
}

func TestWorktreeGuard_BlocksOutOfTreeWorktree(t *testing.T) {
	worktrees := []string{"/repo/.worktrees/quest-1", "/repo-worktrees/quest-2"}
	cases := []struct {
		cmd string
		cwd string
	}{
		{"cd /repo/.worktrees/quest-1", "/repo"},   // absolute
		{"cd .worktrees/quest-1", "/repo"},         // relative to cwd
		{"cd .worktrees/quest-1/src", "/repo"},     // subdir of a worktree
		{"cd ../repo-worktrees/quest-2", "/repo"},  // sibling dir, relative
		{"pushd /repo-worktrees/quest-2", "/repo"}, // absolute pushd
	}
	for _, c := range cases {
		input := &HookInput{ToolInput: ToolInput{Command: c.cmd}}
		result := WorktreeGuard(input, c.cwd, worktrees)
		if !result.Block {
			t.Errorf("should block cd into out-of-tree worktree, cmd=%q", c.cmd)
		}
	}
}

func TestWorktreeGuard_AllowsNonWorktreeCDWithWorktrees(t *testing.T) {
	worktrees := []string{"/repo/.worktrees/quest-1"}
	for _, cmd := range []string{
		"cd /repo/src",                 // sibling of, not inside, a worktree
		"cd src/auth",                  // ordinary relative dir
		"cd /repo/.worktrees-backup/x", // prefix-adjacent but not a real child
	} {
		input := &HookInput{ToolInput: ToolInput{Command: cmd}}
		result := WorktreeGuard(input, "/repo", worktrees)
		if result.Block {
			t.Errorf("should allow cd to non-worktree path, cmd=%q", cmd)
		}
	}
}

func TestWorktreeGuard_AllowsScopedOutOfTreeCD(t *testing.T) {
	// Scoped commands remain safe — CWD doesn't persist between Bash calls.
	worktrees := []string{"/repo/.worktrees/quest-1"}
	input := &HookInput{ToolInput: ToolInput{Command: "cd /repo/.worktrees/quest-1 && go test ./..."}}
	result := WorktreeGuard(input, "/repo", worktrees)
	if result.Block {
		t.Error("scoped cd into an out-of-tree worktree should be allowed")
	}
}
