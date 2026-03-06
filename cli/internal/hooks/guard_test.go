package hooks

import (
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

func TestGateGuard_AllowsTmpWriteDuringEarlyPhase(t *testing.T) {
	s := &state.State{Phase: "Research"}
	for _, path := range []string{"/repo/tmp/notes.md", "tmp/checkpoint.md"} {
		input := &HookInput{ToolInput: ToolInput{FilePath: path}}
		result := GateGuard(s, input)
		if result.Block {
			t.Errorf("should allow tmp/ write during Research: %s", path)
		}
	}
}

func TestGateGuard_AllowsEditDuringLatePhase(t *testing.T) {
	for _, phase := range []string{"Implement", "Review", "Complete"} {
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
