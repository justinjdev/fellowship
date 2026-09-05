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
		// Named inside a quoted argument, not run: a commit message or an echo
		// must not be mistaken for the command it quotes.
		{command: `git commit -m "fellowship init --phase Implement"`, wantOK: false},
		{command: `grep -rn 'fellowship init --phase Review' docs`, wantOK: false},
		// The quote is around the binary being RUN, not around a message
		// naming it: a leading-quote test alone waved these through.
		{command: `'fellowship' init --phase Implement`, want: "Implement", wantOK: true},
		{command: `"$HOME/.claude/fellowship/bin/fellowship" init --plan-skip`, want: "Implement", wantOK: true},
		// ...and the quote opens earlier in the argument, so the binary name
		// itself carries none. Still only a message.
		{command: `git commit -m 'ran fellowship init --phase Implement'`, wantOK: false},
	}
	for _, c := range cases {
		got, ok := InitPhaseRequest(c.command)
		if ok != c.wantOK || got != c.want {
			t.Errorf("InitPhaseRequest(%q) = (%q, %v), want (%q, %v)", c.command, got, ok, c.want, c.wantOK)
		}
	}
}

// An out-of-date store blocks every gate hook until some non-hook invocation
// migrates it — and gate-guard gates Bash, so a command has to get through or
// the block denies its own remedy. It must be one that cannot also advance the
// quest: `fellowship init` resets an existing row's gate_pending, so a
// gate-blocked teammate would release itself the first time a binary upgrade
// made the store stale.
func TestIsStoreUpgradeCommand(t *testing.T) {
	cases := []struct {
		command string
		want    bool
	}{
		{"fellowship status", true},
		{"fellowship gate status", true},
		{"fellowship history show", true},
		{"/usr/local/bin/fellowship status", true},
		{`"$HOME/.claude/fellowship/bin/fellowship" status`, true},
		// Mutating commands are never the remedy.
		{"fellowship init", false},
		{"fellowship init --quest alpha", false},
		{"fellowship state init --name f", false},
		{"fellowship gate approve", false},
		{"fellowship", false},
		{"", false},
		{"git init", false},
		// Nothing may ride along on the allowance.
		{"fellowship status && rm -rf .fellowship", false},
		{"fellowship status; echo done", false},
		{"fellowship status $(whoami)", false},
	}
	for _, c := range cases {
		if got := IsStoreUpgradeCommand(c.command); got != c.want {
			t.Errorf("IsStoreUpgradeCommand(%q) = %v, want %v", c.command, got, c.want)
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
