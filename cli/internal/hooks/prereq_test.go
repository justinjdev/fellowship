package hooks

import (
	"testing"

	"github.com/justinjdev/fellowship/cli/internal/state"
)

func TestGatePrereq_SetsLembasOnLembasSkill(t *testing.T) {
	for _, skill := range []string{"lembas", "fellowship:lembas", "my-plugin:lembas"} {
		s := &state.State{LembasCompleted: false}
		input := &HookInput{ToolInput: ToolInput{Skill: skill}}
		GatePrereq(s, input)
		if !s.LembasCompleted {
			t.Errorf("should set lembas_completed for skill %q", skill)
		}
	}
}

func TestGatePrereq_IgnoresOtherSkills(t *testing.T) {
	s := &state.State{LembasCompleted: false}
	input := &HookInput{ToolInput: ToolInput{Skill: "council"}}
	GatePrereq(s, input)
	if s.LembasCompleted {
		t.Error("should not set lembas_completed for non-lembas skill")
	}
}
