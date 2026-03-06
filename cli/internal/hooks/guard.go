package hooks

import (
	"fmt"
	"strings"

	"github.com/justinjdev/fellowship/cli/internal/state"
)

type HookResult struct {
	Block   bool
	Message string
}

func GateGuard(s *state.State, input *HookInput) HookResult {
	if s.GatePending {
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
		if filePath != "" && !isTmpPath(filePath) {
			return HookResult{
				Block:   true,
				Message: fmt.Sprintf("Phase '%s' does not allow file modifications outside tmp/. Advance to Implement by submitting gates for each phase.", s.Phase),
			}
		}
	}

	return HookResult{}
}

func isTmpPath(path string) bool {
	return strings.Contains(path, "/tmp/") || strings.HasPrefix(path, "tmp/")
}
