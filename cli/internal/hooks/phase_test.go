package hooks

import (
	"testing"

	"github.com/justinjdev/fellowship/cli/internal/state"
)

func TestConfirmPhase(t *testing.T) {
	cases := []struct {
		name         string
		phase        string // the quest's current phase
		confirm      string // the phase the teammate names
		wantRecorded bool
	}{
		{name: "the quest's own phase", phase: "Research", confirm: "Research", wantRecorded: true},
		{name: "the terminal phase, when in it", phase: "Review", confirm: "Review", wantRecorded: true},
		{name: "a phase the quest is not in", phase: "Research", confirm: "Implement"},
		{name: "the next phase is not a move", phase: "Plan", confirm: "Implement"},
		{name: "a name that is not a phase at all", phase: "Research", confirm: "Onboard"},
		{name: "case must match", phase: "Research", confirm: "research"},
		{name: "no phase", phase: "Research", confirm: ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := &state.State{Phase: c.phase}
			recorded, notice := ConfirmPhase(s, c.confirm)
			if recorded != c.wantRecorded {
				t.Errorf("recorded = %v, want %v (notice %q)", recorded, c.wantRecorded, notice)
			}
			if s.MetadataUpdated != c.wantRecorded {
				t.Errorf("MetadataUpdated = %v, want %v", s.MetadataUpdated, c.wantRecorded)
			}
			if (notice != "") == c.wantRecorded {
				t.Errorf("notice = %q, want a notice exactly when not recorded", notice)
			}
			if s.Phase != c.phase {
				t.Errorf("phase moved from %s to %s — confirm must never change the phase", c.phase, s.Phase)
			}
		})
	}
}

func TestConfirmPhase_NilState(t *testing.T) {
	if recorded, _ := ConfirmPhase(nil, "Research"); recorded {
		t.Error("nil state must not record")
	}
}
