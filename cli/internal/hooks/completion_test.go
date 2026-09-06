package hooks

import (
	"testing"

	"github.com/justinjdev/fellowship/cli/internal/state"
)

func TestCompletionCheck_BlocksBeforeReview(t *testing.T) {
	for _, phase := range state.GatePhases() {
		if result := CompletionCheck(&state.State{Phase: phase}); !result.Block {
			t.Errorf("should block completion at phase %s", phase)
		}
	}
}

func TestCompletionCheck_AllowsInReview(t *testing.T) {
	if result := CompletionCheck(&state.State{Phase: state.TerminalPhase}); result.Block {
		t.Errorf("should allow completion in the terminal phase %s: %s", state.TerminalPhase, result.Message)
	}
}

// A gate should never be pending in the terminal phase — nothing leaves it —
// but a stale or hand-edited store could carry one, and completing then would
// bury a decision the lead never made.
func TestCompletionCheck_BlocksWithPendingGateInReview(t *testing.T) {
	gid := "gate-Implement-1"
	s := &state.State{Phase: state.TerminalPhase, GatePending: true, GateID: &gid}
	if result := CompletionCheck(s); !result.Block {
		t.Error("should block completion while a gate is still pending")
	}
}

func TestCompletionCheck_BlocksWhileHeld(t *testing.T) {
	if result := CompletionCheck(&state.State{Phase: state.TerminalPhase, Held: true}); !result.Block {
		t.Error("should block completion while the quest is held")
	}
}

func TestCompletionCheck_NilState(t *testing.T) {
	if result := CompletionCheck(nil); !result.Block {
		t.Error("nil state must block")
	}
}
