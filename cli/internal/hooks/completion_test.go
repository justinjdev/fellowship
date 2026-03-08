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

func TestCompletionGuard_BlocksCompletionBeforeComplete(t *testing.T) {
	for _, phase := range []string{"Onboard", "Research", "Plan", "Implement", "Adversarial", "Review"} {
		s := &state.State{Phase: phase}
		input := &HookInput{ToolInput: ToolInput{Status: "completed"}}
		result := CompletionGuard(s, input)
		if !result.Block {
			t.Errorf("should block completion at phase %s", phase)
		}
	}
}

func TestCompletionGuard_AllowsCompletionAtComplete(t *testing.T) {
	s := &state.State{Phase: "Complete"}
	input := &HookInput{ToolInput: ToolInput{Status: "completed"}}
	result := CompletionGuard(s, input)
	if result.Block {
		t.Error("should allow completion at Complete phase")
	}
}
