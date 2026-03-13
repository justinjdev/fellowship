package hooks

import (
	"fmt"

	"github.com/justinjdev/fellowship/cli/internal/state"
	"github.com/justinjdev/fellowship/cli/internal/tome"
	"zombiezen.com/go/sqlite"
)

func CompletionGuard(s *state.State, input *HookInput) HookResult {
	if input.ToolInput.Status != "completed" {
		return HookResult{}
	}
	if s.Phase != "Complete" {
		return HookResult{
			Block:   true,
			Message: fmt.Sprintf("Cannot complete task — current phase is '%s'. You must submit gates for all phases (Onboard → Research → Plan → Implement → Adversarial → Review → Complete) before completing.", s.Phase),
		}
	}
	return HookResult{}
}

// MarkTomeCompleted marks the quest tome status as "completed".
func MarkTomeCompleted(conn *sqlite.Conn, questName string) {
	tome.SetStatus(conn, questName, "completed")
}
