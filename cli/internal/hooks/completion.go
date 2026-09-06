package hooks

import (
	"fmt"
	"strings"

	"github.com/justinjdev/fellowship/cli/internal/history"
	"github.com/justinjdev/fellowship/cli/internal/state"
	"zombiezen.com/go/sqlite"
)

// CompletionCheck decides whether a quest may be marked complete: only in the
// terminal phase, with no gate awaiting the lead's decision. It is the one
// rule behind `fellowship complete` and behind gate-guard's refusal of that
// command's Bash form, so the CLI and the hook cannot disagree.
func CompletionCheck(s *state.State) HookResult {
	if s == nil {
		return HookResult{Block: true, Message: "Cannot complete quest — no quest state."}
	}
	if s.Phase != state.TerminalPhase {
		return HookResult{
			Block: true,
			Message: fmt.Sprintf("Cannot complete quest — current phase is '%s'. Submit and clear the gates for %s before completing (the quest ends in %s).",
				s.Phase, strings.Join(state.GatePhases(), " → "), state.TerminalPhase),
		}
	}
	// A pending gate in the terminal phase should be impossible (no gate
	// leaves Review), but a hand-edited or stale store could carry one.
	// Completing then would drop a decision the lead never made.
	if s.GatePending {
		return HookResult{
			Block:   true,
			Message: "Cannot complete quest — a gate is still awaiting the lead's decision.",
		}
	}
	if s.Held {
		return HookResult{
			Block:   true,
			Message: "Cannot complete quest — the quest is held by the lead. Wait for the lead to unhold it.",
		}
	}
	return HookResult{}
}

// MarkHistoryCompleted marks the quest history status as "completed".
func MarkHistoryCompleted(conn *sqlite.Conn, questName string) error {
	return history.SetStatus(conn, questName, "completed")
}
