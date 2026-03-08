package hooks

import (
	"testing"

	"github.com/justinjdev/fellowship/cli/internal/state"
)

func TestGateSubmit_AllowsNonGateMessage(t *testing.T) {
	s := &state.State{Phase: "Research"}
	input := &HookInput{ToolInput: ToolInput{Content: "Here is a status update"}}
	result := GateSubmit(s, input)
	if result.Block {
		t.Error("should allow non-gate messages")
	}
	if result.StateChanged {
		t.Error("state should not change for non-gate messages")
	}
}

func TestGateSubmit_DetectsGateMarker(t *testing.T) {
	s := &state.State{Phase: "Research", LembasCompleted: true, MetadataUpdated: true}
	input := &HookInput{ToolInput: ToolInput{Content: "[GATE] Research complete\n- [x] done"}}
	result := GateSubmit(s, input)
	if result.Block {
		t.Error("should allow gate with prereqs met")
	}
	if !result.StateChanged {
		t.Error("state should change for gate submission")
	}
}

func TestGateSubmit_BlocksWithoutLembas(t *testing.T) {
	s := &state.State{Phase: "Research", MetadataUpdated: true}
	input := &HookInput{ToolInput: ToolInput{Content: "[GATE] Research complete"}}
	result := GateSubmit(s, input)
	if !result.Block {
		t.Error("should block without lembas")
	}
}

func TestGateSubmit_BlocksWithoutMetadata(t *testing.T) {
	s := &state.State{Phase: "Research", LembasCompleted: true}
	input := &HookInput{ToolInput: ToolInput{Content: "[GATE] Research complete"}}
	result := GateSubmit(s, input)
	if !result.Block {
		t.Error("should block without metadata")
	}
}

func TestGateSubmit_BlocksDuplicateGate(t *testing.T) {
	s := &state.State{Phase: "Research", GatePending: true, LembasCompleted: true, MetadataUpdated: true}
	input := &HookInput{ToolInput: ToolInput{Content: "[GATE] Research complete"}}
	result := GateSubmit(s, input)
	if !result.Block {
		t.Error("should block duplicate gate")
	}
}

func TestGateSubmit_BlocksMultipleGateMarkers(t *testing.T) {
	s := &state.State{Phase: "Research", LembasCompleted: true, MetadataUpdated: true}
	input := &HookInput{ToolInput: ToolInput{Content: "[GATE] Research\n[GATE] Plan"}}
	result := GateSubmit(s, input)
	if !result.Block {
		t.Error("should block multiple gate markers")
	}
}

func TestGateSubmit_SetsPending(t *testing.T) {
	s := &state.State{Phase: "Research", LembasCompleted: true, MetadataUpdated: true}
	input := &HookInput{ToolInput: ToolInput{Content: "[GATE] Research complete"}}
	GateSubmit(s, input)
	if !s.GatePending {
		t.Error("should set gate_pending")
	}
	if s.GateID == nil || *s.GateID == "" {
		t.Error("should set gate_id")
	}
}

func TestGateSubmit_AutoApprovesByCurrentPhase(t *testing.T) {
	s := &state.State{Phase: "Research", LembasCompleted: true, MetadataUpdated: true, AutoApproveGates: []string{"Research"}}
	input := &HookInput{ToolInput: ToolInput{Content: "[GATE] Research complete"}}
	GateSubmit(s, input)
	if s.Phase != "Plan" {
		t.Errorf("Phase = %q, want Plan", s.Phase)
	}
	if s.GatePending {
		t.Error("gate_pending should stay false for auto-approve")
	}
	if s.LembasCompleted {
		t.Error("lembas should reset after auto-approve")
	}
	if s.MetadataUpdated {
		t.Error("metadata should reset after auto-approve")
	}
}

func TestGateSubmit_DoesNotAutoApproveByDestination(t *testing.T) {
	s := &state.State{Phase: "Research", LembasCompleted: true, MetadataUpdated: true, AutoApproveGates: []string{"Plan"}}
	input := &HookInput{ToolInput: ToolInput{Content: "[GATE] Research complete"}}
	GateSubmit(s, input)
	if !s.GatePending {
		t.Error("should set pending — Plan is the destination, not current phase")
	}
}

func TestGateSubmit_BlocksAtComplete(t *testing.T) {
	s := &state.State{Phase: "Complete", LembasCompleted: true, MetadataUpdated: true}
	input := &HookInput{ToolInput: ToolInput{Content: "[GATE] done"}}
	result := GateSubmit(s, input)
	if !result.Block {
		t.Error("should block gate at Complete phase")
	}
}

func TestGateSubmit_BlocksUnknownPhase(t *testing.T) {
	s := &state.State{Phase: "InvalidPhase", LembasCompleted: true, MetadataUpdated: true}
	input := &HookInput{ToolInput: ToolInput{Content: "[GATE] done"}}
	result := GateSubmit(s, input)
	if !result.Block {
		t.Error("should block gate at unknown phase")
	}
}

func TestGateSubmit_AllPhaseTransitions(t *testing.T) {
	transitions := []struct{ from, to string }{
		{"Onboard", "Research"},
		{"Research", "Plan"},
		{"Plan", "Implement"},
		{"Implement", "Adversarial"},
		{"Adversarial", "Review"},
		{"Review", "Complete"},
	}
	for _, tr := range transitions {
		s := &state.State{Phase: tr.from, LembasCompleted: true, MetadataUpdated: true, AutoApproveGates: []string{tr.from}}
		input := &HookInput{ToolInput: ToolInput{Content: "[GATE] phase complete"}}
		result := GateSubmit(s, input)
		if result.Block {
			t.Errorf("%s -> %s: should not block", tr.from, tr.to)
		}
		if s.Phase != tr.to {
			t.Errorf("%s -> %s: Phase = %q", tr.from, tr.to, s.Phase)
		}
	}
}
