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
	// AutoApproved reports that the gate's phase was on the quest's
	// auto-approve list, so the submission advanced the phase immediately
	// instead of leaving a gate pending for the lead.
	AutoApproved bool
	// PrevPhase is the phase the gate was submitted from; NextPhase is the
	// phase it advances to (reached already when AutoApproved). The caller
	// needs both to write the same tome entries and heralds the lead's
	// `gate approve` writes.
	PrevPhase string
	NextPhase string
}

func GateSubmit(s *state.State, input *HookInput) SubmitResult {
	content := input.ToolInput.Content

	if !hasGateMarker(content) {
		return SubmitResult{}
	}

	if countGateMarkers(content) > 1 {
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
		if s.Phase == state.TerminalPhase {
			msg = "Quest is in its final phase — no further gates to submit. Open the PR and mark the task complete."
		}
		return SubmitResult{Block: true, Message: msg}
	}

	// Every submission goes through the same state machine the lead's
	// `gate approve` uses, including the auto-approved one — which used to
	// advance the phase by hand and so left gate_id behind.
	prevPhase := s.Phase
	gateID := fmt.Sprintf("gate-%s-%d", prevPhase, time.Now().Unix())
	if err := state.Submit(s, gateID); err != nil {
		return SubmitResult{Block: true, Message: fmt.Sprintf("Gate blocked: %v.", err)}
	}

	// Auto-approve checks the current (leaving) phase.
	if !slices.Contains(s.AutoApproveGates, prevPhase) {
		return SubmitResult{StateChanged: true, PrevPhase: prevPhase, NextPhase: nextPhase}
	}

	prev, next, err := state.Approve(s)
	if err != nil {
		// Unreachable: Submit just set the gate and NextPhase already succeeded.
		return SubmitResult{Block: true, Message: fmt.Sprintf("fellowship: auto-approve failed: %v", err)}
	}
	return SubmitResult{StateChanged: true, AutoApproved: true, PrevPhase: prev, NextPhase: next}
}

// RecordGateSubmitted records a "submitted" gate event in the quest tome. An
// auto-approved gate is *also* an approval: the caller records that half with
// gate.RecordApproval, exactly as the lead's `gate approve` does, so the two
// paths cannot drift.
func RecordGateSubmitted(conn *sqlite.Conn, questName, phase string) error {
	return tome.RecordGate(conn, questName, phase, "submitted", "")
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

// countGateMarkers counts the lines that begin with the gate marker. Detection
// and the duplicate check must agree on what a marker is: counting bare
// occurrences of "[GATE]" anywhere in the message rejected a gate that merely
// quoted the token in prose, while the message that opened the gate had to
// carry it at the start of a line.
func countGateMarkers(content string) int {
	n := 0
	for line := range strings.SplitSeq(content, "\n") {
		if strings.HasPrefix(line, "[GATE]") {
			n++
		}
	}
	return n
}

func hasGateMarker(content string) bool {
	return countGateMarkers(content) > 0
}
