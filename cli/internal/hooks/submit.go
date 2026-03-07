package hooks

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/justinjdev/fellowship/cli/internal/tome"
	"github.com/justinjdev/fellowship/cli/internal/state"
)

type SubmitResult struct {
	Block        bool
	Message      string
	StateChanged bool
}

func GateSubmit(s *state.State, input *HookInput) SubmitResult {
	content := input.ToolInput.Content

	if !hasGateMarker(content) {
		return SubmitResult{}
	}

	if strings.Count(content, "[GATE]") > 1 {
		return SubmitResult{Block: true, Message: "Multiple [GATE] markers detected — send one gate per message."}
	}

	if s.GatePending {
		return SubmitResult{Block: true, Message: "Gate already pending — wait for lead approval before submitting another gate."}
	}

	var missing []string
	if !s.LembasCompleted {
		missing = append(missing, "lembas not completed")
	}
	if !s.MetadataUpdated {
		missing = append(missing, "metadata not updated")
	}
	if len(missing) > 0 {
		return SubmitResult{
			Block:   true,
			Message: fmt.Sprintf("Gate blocked: %s. Run /lembas and update task metadata before submitting a gate.", strings.Join(missing, ", ")),
		}
	}

	nextPhase, err := state.NextPhase(s.Phase)
	if err != nil {
		msg := fmt.Sprintf("fellowship: %v — cannot submit gate", err)
		if s.Phase == "Complete" {
			msg = "Quest already complete — no further gates to submit."
		}
		return SubmitResult{Block: true, Message: msg}
	}

	// Auto-approve checks current (leaving) phase.
	if slices.Contains(s.AutoApproveGates, s.Phase) {
		s.Phase = nextPhase
		s.LembasCompleted = false
		s.MetadataUpdated = false
		return SubmitResult{StateChanged: true}
	}

	gateID := fmt.Sprintf("gate-%s-%d", s.Phase, time.Now().Unix())
	s.GatePending = true
	s.GateID = &gateID
	return SubmitResult{StateChanged: true}
}

// RecordGateSubmitted records a "submitted" gate event in the quest tome.
func RecordGateSubmitted(tomePath string, phase string) {
	c := tome.LoadOrCreate(tomePath)
	tome.RecordGate(c, phase, "submitted")
	tome.RecordPhase(c, phase)
	tome.Save(tomePath, c)
}

func hasGateMarker(content string) bool {
	for line := range strings.SplitSeq(content, "\n") {
		if strings.HasPrefix(line, "[GATE]") {
			return true
		}
	}
	return false
}
