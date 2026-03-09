package hooks

import (
	"strings"
	"testing"

	"github.com/justinjdev/fellowship/cli/internal/state"
)

func TestGateGuard_AllowsWhenNotPending(t *testing.T) {
	s := &state.State{Phase: "Research", GatePending: false}
	input := &HookInput{ToolInput: ToolInput{Command: "ls"}}
	result := GateGuard(s, input)
	if result.Block {
		t.Errorf("should allow when not pending, got blocked: %s", result.Message)
	}
}

func TestGateGuard_BlocksWhenPending(t *testing.T) {
	s := &state.State{Phase: "Research", GatePending: true}
	input := &HookInput{ToolInput: ToolInput{Command: "ls"}}
	result := GateGuard(s, input)
	if !result.Block {
		t.Error("should block when gate pending")
	}
}

func TestGateGuard_BlocksEditDuringEarlyPhase(t *testing.T) {
	for _, phase := range []string{"Onboard", "Research", "Plan"} {
		s := &state.State{Phase: phase}
		input := &HookInput{ToolInput: ToolInput{FilePath: "/repo/src/main.ts"}}
		result := GateGuard(s, input)
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
		result := GateGuard(s, input)
		if result.Block {
			t.Errorf("should allow .fellowship/ write during Research: %s", path)
		}
	}
}

func TestGateGuard_AllowsEditDuringLatePhase(t *testing.T) {
	for _, phase := range []string{"Implement", "Adversarial", "Review", "Complete"} {
		s := &state.State{Phase: phase}
		input := &HookInput{ToolInput: ToolInput{FilePath: "/repo/src/main.ts"}}
		result := GateGuard(s, input)
		if result.Block {
			t.Errorf("should allow Edit during %s", phase)
		}
	}
}

func TestGateGuard_AllowsBashDuringEarlyPhase(t *testing.T) {
	s := &state.State{Phase: "Research"}
	input := &HookInput{ToolInput: ToolInput{Command: "ls"}}
	result := GateGuard(s, input)
	if result.Block {
		t.Error("should allow Bash during Research")
	}
}

func TestGateGuard_BlocksNotebookEditDuringEarlyPhase(t *testing.T) {
	s := &state.State{Phase: "Research"}
	input := &HookInput{ToolInput: ToolInput{NotebookPath: "/repo/src/analysis.ipynb"}}
	result := GateGuard(s, input)
	if !result.Block {
		t.Error("should block NotebookEdit to prod file during Research")
	}
}

func TestGateGuard_PendingBlocksEvenDuringLatePhase(t *testing.T) {
	s := &state.State{Phase: "Implement", GatePending: true}
	input := &HookInput{ToolInput: ToolInput{FilePath: "/repo/src/main.ts"}}
	result := GateGuard(s, input)
	if !result.Block {
		t.Error("gate_pending should block even during Implement")
	}
}

func TestGateGuard_BlocksWhenHeld(t *testing.T) {
	s := &state.State{Phase: "Implement", Held: true}
	input := &HookInput{ToolInput: ToolInput{Command: "ls"}}
	result := GateGuard(s, input)
	if !result.Block {
		t.Error("should block when quest is held")
	}
}

func TestGateGuard_BlocksWhenHeldWithReason(t *testing.T) {
	reason := "file conflict with quest-auth"
	s := &state.State{Phase: "Implement", Held: true, HeldReason: &reason}
	input := &HookInput{ToolInput: ToolInput{Command: "ls"}}
	result := GateGuard(s, input)
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
	result := GateGuard(s, input)
	if !result.Block {
		t.Error("should block")
	}
	if !strings.Contains(result.Message, "held") {
		t.Errorf("held should take priority over gate_pending, got: %s", result.Message)
	}
}

func TestGateGuard_AllowsFellowshipGateRejectWhenPending(t *testing.T) {
	s := &state.State{Phase: "Research", GatePending: true}
	for _, cmd := range []string{
		"~/.claude/fellowship/bin/fellowship gate reject",
		"fellowship gate reject",
		"/usr/local/bin/fellowship gate reject",
	} {
		input := &HookInput{ToolInput: ToolInput{Command: cmd}}
		result := GateGuard(s, input)
		if result.Block {
			t.Errorf("fellowship gate reject should be allowed through gate_pending, cmd=%q got: %s", cmd, result.Message)
		}
	}
}

func TestGateGuard_AllowsFellowshipInitWhenPending(t *testing.T) {
	s := &state.State{Phase: "Implement", GatePending: true}
	input := &HookInput{ToolInput: ToolInput{Command: "~/.claude/fellowship/bin/fellowship init"}}
	result := GateGuard(s, input)
	if result.Block {
		t.Errorf("fellowship init should be allowed through gate_pending: %s", result.Message)
	}
}

func TestGateGuard_BlocksChainedCommandsWithFellowshipEscape(t *testing.T) {
	s := &state.State{Phase: "Implement", GatePending: true}
	for _, cmd := range []string{
		"rm -rf / && fellowship gate reject",
		"fellowship gate reject; rm -rf /",
		"fellowship gate reject || evil",
		"echo foo | fellowship gate reject",
	} {
		input := &HookInput{ToolInput: ToolInput{Command: cmd}}
		result := GateGuard(s, input)
		if !result.Block {
			t.Errorf("chained command should be blocked even with fellowship gate reject, cmd=%q", cmd)
		}
	}
}

func TestGateGuard_HeldBlocksFellowshipEscapeCommands(t *testing.T) {
	s := &state.State{Phase: "Implement", Held: true}
	input := &HookInput{ToolInput: ToolInput{Command: "fellowship gate reject"}}
	result := GateGuard(s, input)
	if !result.Block {
		t.Error("held state should block even fellowship escape commands")
	}
}
