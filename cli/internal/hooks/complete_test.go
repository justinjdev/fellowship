package hooks

import (
	"strings"
	"testing"

	"github.com/justinjdev/fellowship/cli/internal/state"
)

func TestHasCompleteCommand(t *testing.T) {
	cases := []struct {
		command string
		want    bool
	}{
		{"fellowship complete --dir /wt", true},
		{"~/.claude/fellowship/bin/fellowship complete --dir /wt", true},
		{`"$HOME/.claude/fellowship/bin/fellowship" complete`, true},
		{"git push && fellowship complete --dir .", true},
		{"cd /wt; fellowship complete", true},
		{"echo $(fellowship complete)", true},
		{`sh -c "fellowship complete --dir /wt"`, true},
		{"eval fellowship complete", true},
		{"fellowship gate status", false},
		{"fellowship phase confirm --dir /wt --phase Review", false},
		{`git commit -m "fellowship complete"`, false},
		{"ls", false},
		{"", false},
	}
	for _, c := range cases {
		if got := HasCompleteCommand(c.command); got != c.want {
			t.Errorf("HasCompleteCommand(%q) = %v, want %v", c.command, got, c.want)
		}
	}
}

// gate-guard refuses the Bash form of `fellowship complete` under exactly the
// rule the command itself applies: only Review, with no gate pending, may end
// the quest.
func TestGateGuard_RefusesCompleteOutsideReview(t *testing.T) {
	for _, phase := range state.GatePhases() {
		s := &state.State{Phase: phase}
		input := &HookInput{ToolName: "Bash", ToolInput: ToolInput{Command: "fellowship complete --dir /wt"}}
		result := GateGuard(s, input, GuardParams{})
		if !result.Block {
			t.Errorf("phase %s: `fellowship complete` should be refused", phase)
		} else if !strings.Contains(result.Message, "Cannot complete quest") {
			t.Errorf("phase %s: unexpected message %q", phase, result.Message)
		}
	}
}

func TestGateGuard_AllowsCompleteInReview(t *testing.T) {
	s := &state.State{Phase: state.TerminalPhase}
	input := &HookInput{ToolName: "Bash", ToolInput: ToolInput{Command: "fellowship complete --dir /wt"}}
	if result := GateGuard(s, input, GuardParams{}); result.Block {
		t.Errorf("`fellowship complete` in Review should be allowed, got: %s", result.Message)
	}
}

// The lead's exemption from the lead-only command rule does not extend to a
// completion: the invariant is about the quest's phase, not about who asks.
func TestGateGuard_LeadCannotCompleteEarlyEither(t *testing.T) {
	s := &state.State{Phase: "Implement"}
	input := &HookInput{ToolName: "Bash", SessionID: "lead", ToolInput: ToolInput{Command: "fellowship complete --dir /wt"}}
	if result := GateGuard(s, input, GuardParams{LeadSessionID: "lead"}); !result.Block {
		t.Error("`fellowship complete` before Review must be refused whoever runs it")
	}
}

// `phase confirm` is a prerequisite, not a phase move, so gate-guard lets it
// through in every phase a gate can be submitted from (and in Review, where
// it is harmless). Only a pending gate or a hold stop it, like any Bash.
func TestGateGuard_AllowsPhaseConfirmInEveryPhase(t *testing.T) {
	for _, phase := range state.Phases() {
		s := &state.State{Phase: phase}
		input := &HookInput{ToolName: "Bash", ToolInput: ToolInput{Command: "~/.claude/fellowship/bin/fellowship phase confirm --dir /wt --phase " + phase}}
		if result := GateGuard(s, input, GuardParams{}); result.Block {
			t.Errorf("phase %s: `phase confirm` should be allowed, got: %s", phase, result.Message)
		}
	}
	// Naming another phase is refused by the command, not the guard — the
	// guard has no phase to move and nothing to protect here.
	if got := LeadOnlyCommands("fellowship phase confirm --dir /wt --phase Implement"); len(got) != 0 {
		t.Errorf("phase confirm is not a lead-only command, got %+v", got)
	}
}

func TestGateGuard_PhaseConfirmBlockedWhilePending(t *testing.T) {
	s := &state.State{Phase: "Research", GatePending: true}
	input := &HookInput{ToolName: "Bash", ToolInput: ToolInput{Command: "fellowship phase confirm --dir /wt --phase Research"}}
	if result := GateGuard(s, input, GuardParams{}); !result.Block {
		t.Error("with a gate pending, `phase confirm` is Bash like any other and is blocked")
	}
}

// A teammate spawned with the Agent tool shares the lead's session id. The
// lead-only command exemption must not apply to its payloads: the agent id
// marks them as a subagent's.
func TestGateGuard_SubagentWithLeadSessionIsNotExemptFromLeadOnlyRule(t *testing.T) {
	s := &state.State{Phase: "Research"}
	input := &HookInput{
		ToolName:  "Bash",
		SessionID: "lead",
		AgentID:   "agent-3",
		AgentType: "general-purpose",
		ToolInput: ToolInput{Command: "fellowship init --dir /wt --phase Implement"},
	}
	if result := GateGuard(s, input, GuardParams{LeadSessionID: "lead"}); !result.Block {
		t.Error("a subagent sharing the lead's session id must not move the phase")
	}
	input.ToolInput.Command = "cd /repo && fellowship state init --claim-lead"
	if result := GateGuard(s, input, GuardParams{LeadSessionID: "lead"}); !result.Block {
		t.Error("a subagent sharing the lead's session id must not run lead commands")
	}
	// The lead's own conversation (no agent id) keeps its exemption.
	input.AgentID, input.AgentType = "", ""
	input.ToolInput.Command = "fellowship init --dir /wt --phase Implement"
	if result := GateGuard(s, input, GuardParams{LeadSessionID: "lead"}); result.Block {
		t.Errorf("the lead's own payload keeps the exemption, got: %s", result.Message)
	}
}

func TestIsLeadPayload(t *testing.T) {
	cases := []struct {
		name  string
		input *HookInput
		lead  string
		want  bool
	}{
		{"lead session, no agent", &HookInput{SessionID: "s"}, "s", true},
		{"lead session, subagent", &HookInput{SessionID: "s", AgentID: "a"}, "s", false},
		{"other session", &HookInput{SessionID: "t"}, "s", false},
		{"no session in payload", &HookInput{}, "s", false},
		{"no lead recorded", &HookInput{SessionID: "s"}, "", false},
		{"nil input", nil, "s", false},
	}
	for _, c := range cases {
		if got := IsLeadPayload(c.input, c.lead); got != c.want {
			t.Errorf("%s: IsLeadPayload = %v, want %v", c.name, got, c.want)
		}
	}
}
