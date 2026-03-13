package hooks

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/justinjdev/fellowship/cli/internal/state"
	"github.com/justinjdev/fellowship/cli/internal/tome"
	"zombiezen.com/go/sqlite"
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
// If autoApproved is true, the phase is also recorded as completed.
func RecordGateSubmitted(conn *sqlite.Conn, questName, phase string, autoApproved bool) {
	tome.RecordGate(conn, questName, phase, "submitted", "")
	if autoApproved {
		tome.RecordGate(conn, questName, phase, "approved", "")
		tome.RecordPhase(conn, questName, phase, 0)
	}
}

// HookSpecificOutput is the JSON structure Claude Code expects from
// PreToolUse hooks when they need to modify tool input.
type HookSpecificOutput struct {
	HSO hookSpecificOutputInner `json:"hookSpecificOutput"`
}

type hookSpecificOutputInner struct {
	HookEventName            string            `json:"hookEventName"`
	PermissionDecision       string            `json:"permissionDecision"`
	PermissionDecisionReason string            `json:"permissionDecisionReason,omitempty"`
	UpdatedInput             map[string]string `json:"updatedInput,omitempty"`
}

// NewAllowOutput returns a HookSpecificOutput that allows the tool call
// with optional input mutation.
func NewAllowOutput(updatedInput map[string]string) HookSpecificOutput {
	return HookSpecificOutput{
		HSO: hookSpecificOutputInner{
			HookEventName:      "PreToolUse",
			PermissionDecision: "allow",
			UpdatedInput:       updatedInput,
		},
	}
}

// NewDenyOutput returns a HookSpecificOutput that blocks the tool call.
func NewDenyOutput(reason string) HookSpecificOutput {
	return HookSpecificOutput{
		HSO: hookSpecificOutputInner{
			HookEventName:            "PreToolUse",
			PermissionDecision:       "deny",
			PermissionDecisionReason: reason,
		},
	}
}

func hasGateMarker(content string) bool {
	for line := range strings.SplitSeq(content, "\n") {
		if strings.HasPrefix(line, "[GATE]") {
			return true
		}
	}
	return false
}
