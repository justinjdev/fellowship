package hooks

import (
	"testing"

	"github.com/justinjdev/fellowship/cli/internal/state"
)

func TestCompletionGuard_AllowsNonCompletion(t *testing.T) {
	s := &state.State{Phase: "Research"}
	input := &HookInput{ToolInput: ToolInput{Status: "in_progress"}}
	result := CompletionGuard(s, input)
	if result.Block {
		t.Error("should allow non-completion updates")
	}
}

func TestCompletionGuard_AllowsMetadataOnly(t *testing.T) {
	s := &state.State{Phase: "Research"}
	input := &HookInput{ToolInput: ToolInput{Metadata: &TaskMetadata{Phase: "Research"}}}
	result := CompletionGuard(s, input)
	if result.Block {
		t.Error("should allow metadata-only updates")
	}
}

func TestCompletionGuard_BlocksCompletionBeforeReview(t *testing.T) {
	for _, phase := range state.GatePhases() {
		s := &state.State{Phase: phase}
		input := &HookInput{ToolInput: ToolInput{Status: "completed"}}
		result := CompletionGuard(s, input)
		if !result.Block {
			t.Errorf("should block completion at phase %s", phase)
		}
	}
}

func TestCompletionGuard_AllowsCompletionInReview(t *testing.T) {
	s := &state.State{Phase: state.TerminalPhase}
	input := &HookInput{ToolInput: ToolInput{Status: "completed"}}
	result := CompletionGuard(s, input)
	if result.Block {
		t.Errorf("should allow completion in the terminal phase %s", state.TerminalPhase)
	}
}

// A gate should never be pending in the terminal phase — nothing leaves it —
// but a stale or hand-edited store could carry one, and completing then would
// bury a decision the lead never made.
func TestCompletionGuard_BlocksCompletionWithPendingGateInReview(t *testing.T) {
	gid := "gate-Implement-1"
	s := &state.State{Phase: state.TerminalPhase, GatePending: true, GateID: &gid}
	input := &HookInput{ToolInput: ToolInput{Status: "completed"}}
	result := CompletionGuard(s, input)
	if !result.Block {
		t.Error("should block completion while a gate is still pending")
	}
}
