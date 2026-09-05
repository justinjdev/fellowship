package hooks

import (
	"fmt"
	"strings"

	"github.com/justinjdev/fellowship/cli/internal/history"
	"github.com/justinjdev/fellowship/cli/internal/state"
	"zombiezen.com/go/sqlite"
)

func CompletionGuard(s *state.State, input *HookInput) HookResult {
	if input.ToolInput.Status != "completed" {
		return HookResult{}
	}
	if s.Phase != state.TerminalPhase {
		return HookResult{
			Block: true,
			Message: fmt.Sprintf("Cannot complete task — current phase is '%s'. Submit and clear the gates for %s before completing (the quest ends in %s).",
				s.Phase, strings.Join(state.GatePhases(), " → "), state.TerminalPhase),
		}
	}
	// A pending gate in the terminal phase should be impossible (no gate
	// leaves Review), but a hand-edited or stale store could carry one.
	// Completing then would drop a decision the lead never made.
	if s.GatePending {
		return HookResult{
			Block:   true,
			Message: "Cannot complete task — a gate is still awaiting the lead's decision.",
		}
	}
	return HookResult{}
}

// MarkHistoryCompleted marks the quest history status as "completed".
func MarkHistoryCompleted(conn *sqlite.Conn, questName string) error {
	return history.SetStatus(conn, questName, "completed")
}
