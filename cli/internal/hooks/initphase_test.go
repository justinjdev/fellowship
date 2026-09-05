package hooks

import (
	"testing"

	"github.com/justinjdev/fellowship/cli/internal/state"
)

// `fellowship init --phase X` on an existing quest row is a phase move, and a
// phase move is a gate decision. These cover the Bash form of that attempt.

func TestInitPhaseRequest(t *testing.T) {
	cases := []struct {
		command string
		want    string
		wantOK  bool
	}{
		{command: "fellowship init --phase Implement", want: "Implement", wantOK: true},
		{command: "fellowship init --phase=Implement", want: "Implement", wantOK: true},
		{command: "fellowship init -phase Review", want: "Review", wantOK: true},
		{command: "fellowship init --plan-skip", want: "Implement", wantOK: true},
		{command: "fellowship init --quest q --plan-skip", want: "Implement", wantOK: true},
		{command: "cd /wt && fellowship init --phase Implement", want: "Implement", wantOK: true},
		{command: "/usr/local/bin/fellowship init --phase Plan", want: "Plan", wantOK: true},
		// Not a phase move.
		{command: "fellowship init", wantOK: false},
		{command: "fellowship init --quest alpha", wantOK: false},
		{command: "fellowship status", wantOK: false},
		{command: "", wantOK: false},
		{command: "fellowship gate approve", wantOK: false},
	}
	for _, c := range cases {
		got, ok := InitPhaseRequest(c.command)
		if ok != c.wantOK || got != c.want {
			t.Errorf("InitPhaseRequest(%q) = (%q, %v), want (%q, %v)", c.command, got, ok, c.want, c.wantOK)
		}
	}
}

func TestGateGuard_InitPhaseChange(t *testing.T) {
	cases := []struct {
		name      string
		phase     string
		command   string
		sessionID string
		lead      string
		wantBlock bool
	}{
		{
			name:      "teammate cannot move the phase with init",
			phase:     "Research",
			command:   "fellowship init --phase Implement",
			sessionID: "teammate",
			lead:      "lead",
			wantBlock: true,
		},
		{
			name:      "teammate cannot move the phase with --plan-skip",
			phase:     "Research",
			command:   "fellowship init --plan-skip",
			sessionID: "teammate",
			lead:      "lead",
			wantBlock: true,
		},
		{
			name:      "an unidentified session cannot move the phase either",
			phase:     "Research",
			command:   "fellowship init --phase Implement",
			sessionID: "",
			lead:      "lead",
			wantBlock: true,
		},
		{
			name:      "the lead may move the phase",
			phase:     "Research",
			command:   "fellowship init --phase Implement",
			sessionID: "lead",
			lead:      "lead",
			wantBlock: false,
		},
		{
			name:      "re-running init for the phase the quest is in is not a move",
			phase:     "Implement",
			command:   "fellowship init --phase Implement",
			sessionID: "teammate",
			lead:      "lead",
			wantBlock: false,
		},
		{
			name:      "plain init still resets the gate flags for anyone",
			phase:     "Research",
			command:   "fellowship init --quest alpha",
			sessionID: "teammate",
			lead:      "lead",
			wantBlock: false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := &state.State{QuestName: "alpha", Phase: c.phase}
			input := &HookInput{SessionID: c.sessionID, ToolInput: ToolInput{Command: c.command}}
			result := GateGuard(s, input, GuardParams{LeadSessionID: c.lead})
			if result.Block != c.wantBlock {
				t.Errorf("GateGuard block = %v, want %v (%s)", result.Block, c.wantBlock, result.Message)
			}
		})
	}
}

func TestIsLeadSession(t *testing.T) {
	cases := []struct {
		session, lead string
		want          bool
	}{
		{"a", "a", true},
		{"a", "b", false},
		{"", "", false},
		{"", "a", false},
		{"a", "", false},
	}
	for _, c := range cases {
		if got := IsLeadSession(c.session, c.lead); got != c.want {
			t.Errorf("IsLeadSession(%q, %q) = %v, want %v", c.session, c.lead, got, c.want)
		}
	}
}
