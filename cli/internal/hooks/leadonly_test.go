package hooks

import (
	"testing"

	"github.com/justinjdev/fellowship/cli/internal/state"
)

// A verifier broke the first version of this guard with one Bash call from a
// quest worktree: `cd <main-repo-root> && fellowship state init --claim-lead`.
// The scanner only looked for `init --phase`, so the teammate became the
// recorded lead — after which it could move its own phase and write in the main
// tree, while the real lead was refused.
func TestLeadOnlyCommand(t *testing.T) {
	cases := []struct {
		name    string
		command string
		want    LeadOnlyInvocation
		wantOK  bool
	}{
		// --- the reported repros -------------------------------------------
		{
			name:    "claim-lead from a quest worktree",
			command: "cd /repo && fellowship state init --claim-lead",
			want:    LeadOnlyInvocation{Subcommand: "state", Detail: "init"},
			wantOK:  true,
		},
		{
			name:    "a plain state init overwrites the fellowship on the way past",
			command: "cd /repo && fellowship state init --name evil --skip-hook-install",
			want:    LeadOnlyInvocation{Subcommand: "state", Detail: "init"},
			wantOK:  true,
		},
		{
			name:    "sh -c wrapping",
			command: `sh -c "fellowship init --phase Review"`,
			want:    LeadOnlyInvocation{Subcommand: "init", Detail: "Review"},
			wantOK:  true,
		},
		{
			name:    "bash -lc wrapping around the state command",
			command: `bash -lc 'fellowship state init --claim-lead'`,
			want:    LeadOnlyInvocation{Subcommand: "state", Detail: "init"},
			wantOK:  true,
		},
		{
			name:    "the installed full path",
			command: "~/.claude/fellowship/bin/fellowship init --phase Review",
			want:    LeadOnlyInvocation{Subcommand: "init", Detail: "Review"},
			wantOK:  true,
		},
		{
			name:    "the plugin's wrapper script",
			command: "/plugin/hooks/scripts/fellowship.sh state add-quest --name x",
			want:    LeadOnlyInvocation{Subcommand: "state", Detail: "add-quest"},
			wantOK:  true,
		},
		{
			name:    "an environment assignment in front of the binary",
			command: "CLAUDE_CODE_SESSION_ID=lead-1 fellowship state init --claim-lead",
			want:    LeadOnlyInvocation{Subcommand: "state", Detail: "init"},
			wantOK:  true,
		},
		{
			name:    "hidden behind a semicolon",
			command: "echo hi; fellowship state show",
			want:    LeadOnlyInvocation{Subcommand: "state", Detail: "show"},
			wantOK:  true,
		},
		{
			name:    "hidden behind a pipe",
			command: "true | fellowship state show --json",
			want:    LeadOnlyInvocation{Subcommand: "state", Detail: "show"},
			wantOK:  true,
		},
		{
			name:    "eval",
			command: `eval fellowship state init --claim-lead`,
			want:    LeadOnlyInvocation{Subcommand: "state", Detail: "init"},
			wantOK:  true,
		},
		{
			name:    "nested one shell deeper",
			command: `sh -c "bash -c 'fellowship state init --claim-lead'"`,
			want:    LeadOnlyInvocation{Subcommand: "state", Detail: "init"},
			wantOK:  true,
		},
		{
			name:    "quoted binary path",
			command: `"$HOME/.claude/fellowship/bin/fellowship" init --plan-skip`,
			want:    LeadOnlyInvocation{Subcommand: "init", Detail: "Implement"},
			wantOK:  true,
		},
		{
			name:    "--phase= form",
			command: "fellowship init --phase=Implement",
			want:    LeadOnlyInvocation{Subcommand: "init", Detail: "Implement"},
			wantOK:  true,
		},

		// --- not lead-only --------------------------------------------------
		{name: "a plain init resets the gate flags and moves nothing", command: "fellowship init --dir /wt"},
		{name: "the quest's own reporting", command: "fellowship gate status"},
		{name: "the quest's own bookkeeping", command: "fellowship todo update 1 done"},
		{name: "status", command: "fellowship status --json"},
		{name: "nothing to do with the CLI", command: "go test ./..."},
		{name: "empty", command: ""},
		{name: "git init is not fellowship init", command: "git init --phase Review"},
		{
			name:    "the command NAMED inside a commit message is not an invocation",
			command: `git commit -m "fellowship state init --claim-lead"`,
		},
		{
			name:    "...nor inside a grep pattern",
			command: `grep -rn 'fellowship state init' docs`,
		},
		{
			name:    "...nor a quoted sh -c that is itself only quoted text",
			command: `git commit -m "sh -c 'fellowship state init'"`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := LeadOnlyCommand(c.command)
			if ok != c.wantOK || got != c.want {
				t.Errorf("LeadOnlyCommand(%q) = (%+v, %v), want (%+v, %v)", c.command, got, ok, c.want, c.wantOK)
			}
		})
	}
}

// gate-guard runs only where a quest row resolved — inside a registered quest
// worktree — so a lead command there is a teammate reaching for the lead's own
// command set, whatever the quest's phase and with no gate pending.
func TestGateGuard_BlocksLeadOnlyCommands(t *testing.T) {
	blocked := []string{
		"cd /repo && fellowship state init --claim-lead",
		"cd /repo && fellowship state init --name evil --skip-hook-install",
		`sh -c "cd /repo && fellowship state init --claim-lead"`,
		"fellowship state update-quest --name alpha --status completed",
		"fellowship state show",
		`sh -c "fellowship init --phase Review"`,
		"~/.claude/fellowship/bin/fellowship init --phase Review",
	}
	for _, cmd := range blocked {
		s := &state.State{QuestName: "alpha", Phase: "Implement"}
		input := &HookInput{SessionID: "teammate", ToolInput: ToolInput{Command: cmd}}
		if result := GateGuard(s, input, GuardParams{LeadSessionID: "lead"}); !result.Block {
			t.Errorf("should block %q", cmd)
		}
	}

	// The lead's own session is never blocked by this rule — it is the one
	// session these commands belong to.
	for _, cmd := range blocked {
		s := &state.State{QuestName: "alpha", Phase: "Implement"}
		input := &HookInput{SessionID: "lead", ToolInput: ToolInput{Command: cmd}}
		if result := GateGuard(s, input, GuardParams{LeadSessionID: "lead"}); result.Block {
			t.Errorf("should allow the lead to run %q: %s", cmd, result.Message)
		}
	}

	allowed := []string{
		"fellowship init --dir /wt",
		"fellowship gate status",
		"fellowship todo list",
		"go test ./...",
		`git commit -m "fellowship state init"`,
	}
	for _, cmd := range allowed {
		s := &state.State{QuestName: "alpha", Phase: "Implement"}
		input := &HookInput{SessionID: "teammate", ToolInput: ToolInput{Command: cmd}}
		if result := GateGuard(s, input, GuardParams{LeadSessionID: "lead"}); result.Block {
			t.Errorf("should allow %q: %s", cmd, result.Message)
		}
	}
}
