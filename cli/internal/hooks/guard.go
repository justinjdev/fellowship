package hooks

import (
	"fmt"
	"strings"

	"github.com/justinjdev/fellowship/cli/internal/datadir"
	"github.com/justinjdev/fellowship/cli/internal/state"
)

type HookResult struct {
	Block   bool
	Message string
}

func GateGuard(s *state.State, input *HookInput) HookResult {
	if s.Held {
		msg := "Quest is held — paused by the lead."
		if s.HeldReason != nil {
			msg += " Reason: " + *s.HeldReason
		}
		msg += " Wait for the lead to unhold before taking any action."
		return HookResult{
			Block:   true,
			Message: msg,
		}
	}

	if s.GatePending && !isFellowshipEscapeCommand(input.ToolInput.Command) {
		return HookResult{
			Block:   true,
			Message: "Gate pending — waiting for lead approval. Do not take any action until the lead approves your gate.",
		}
	}

	if state.IsEarlyPhase(s.Phase) {
		filePath := input.ToolInput.FilePath
		if filePath == "" {
			filePath = input.ToolInput.NotebookPath
		}
		if filePath != "" && !datadir.IsDataDirPath(filePath) {
			return HookResult{
				Block:   true,
				Message: fmt.Sprintf("Phase '%s' does not allow file modifications outside %s/. Advance to Implement by submitting gates for each phase.", s.Phase, datadir.Name()),
			}
		}
	}

	return HookResult{}
}

// isFellowshipEscapeCommand returns true for fellowship CLI commands that must
// be allowed through even when gate_pending is true — specifically the commands
// needed to unstick a blocked session without requiring user intervention.
//
// Shell chaining operators (&&, ||, ;, |) are rejected to prevent abuse.
func isFellowshipEscapeCommand(command string) bool {
	if command == "" {
		return false
	}
	trimmed := strings.TrimSpace(command)
	if strings.Contains(trimmed, "&&") || strings.Contains(trimmed, "||") ||
		strings.Contains(trimmed, ";") || strings.Contains(trimmed, "|") {
		return false
	}
	return strings.Contains(trimmed, "fellowship gate reject") ||
		strings.Contains(trimmed, "fellowship gate approve") ||
		strings.Contains(trimmed, "fellowship init")
}

