package hooks

import (
	"testing"

	"github.com/justinjdev/fellowship/cli/internal/state"
)

// A held teammate could not run `fellowship status` to see why it was stopped,
// or record a failure about it: the hold blocked everything, including the
// read-only escape commands the pending-gate block already allowed. Nothing on
// that allowlist can release a hold.
func TestGateGuard_HoldAllowsTheEscapeCommands(t *testing.T) {
	reason := "waiting on the lead"
	held := func() *state.State {
		return &state.State{QuestName: "alpha", Phase: "Implement", Held: true, HeldReason: &reason}
	}

	allowed := []string{
		"fellowship status",
		"fellowship status --json",
		"fellowship gate status",
		"fellowship history show",
		"fellowship events --limit 5",
		"fellowship notes list",
		"fellowship failures scan --all",
		"fellowship todo list",
	}
	for _, cmd := range allowed {
		input := &HookInput{ToolInput: ToolInput{Command: cmd}}
		if result := GateGuard(held(), input, GuardParams{}); result.Block {
			t.Errorf("held quest should allow %q: %s", cmd, result.Message)
		}
	}

	blocked := []string{
		"go test ./...",
		"fellowship gate approve",
		"fellowship gate reject",
		"fellowship init",
		"fellowship status && rm -rf /",
	}
	for _, cmd := range blocked {
		input := &HookInput{ToolInput: ToolInput{Command: cmd}}
		if result := GateGuard(held(), input, GuardParams{}); !result.Block {
			t.Errorf("held quest should block %q", cmd)
		}
	}

	// A held quest still blocks file writes — the allowlist is about commands.
	input := &HookInput{ToolName: "Write", ToolInput: ToolInput{FilePath: "/repo/src/main.go"}}
	if result := GateGuard(held(), input, GuardParams{}); !result.Block {
		t.Error("held quest should block file writes")
	}
}
