package hooks

import (
	"strings"

	"github.com/justinjdev/fellowship/cli/internal/state"
)

func GatePrereq(s *state.State, input *HookInput) bool {
	if strings.Contains(input.ToolInput.Skill, "lembas") {
		s.LembasCompleted = true
		return true
	}
	return false
}
