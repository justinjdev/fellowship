package hooks

import (
	"fmt"

	"github.com/justinjdev/fellowship/cli/internal/tome"
	"github.com/justinjdev/fellowship/cli/internal/state"
)

func CompletionGuard(s *state.State, input *HookInput) HookResult {
	if input.ToolInput.Status != "completed" {
		return HookResult{}
	}
	if s.Phase != "Complete" {
		return HookResult{
			Block:   true,
			Message: fmt.Sprintf("Cannot complete task — current phase is '%s'. You must submit gates for all phases (Onboard → Research → Plan → Implement → Review → Complete) before completing.", s.Phase),
		}
	}
	return HookResult{}
}

// MarkTomeCompleted marks the quest tome status as "completed".
func MarkTomeCompleted(tomePath string) {
	c := tome.LoadOrCreate(tomePath)
	c.Status = "completed"
	tome.Save(tomePath, c)
}
