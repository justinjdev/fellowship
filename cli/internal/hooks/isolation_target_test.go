package hooks

import "testing"

// The guard used to look only at where the SESSION stood: a teammate inside its
// own worktree returned "not my concern" before the target path was ever
// considered, so an absolute path into the main tree was free.
func TestIsolationGuard_TargetPathDecides(t *testing.T) {
	base := IsolationParams{
		FellowshipActive:         true,
		MainRoot:                 "/repo",
		SessionTopLevel:          "/wt/quest-1",
		TargetTopLevel:           "/repo",
		ToolName:                 "Write",
		FilePath:                 "/repo/src/main.go",
		DataDirName:              ".fellowship",
		SessionID:                "teammate",
		LeadSessionID:            "lead",
		SessionIsRegisteredQuest: true,
	}

	cases := []struct {
		name      string
		mutate    func(*IsolationParams)
		wantBlock bool
	}{
		{
			name:      "teammate in its worktree writing into the main tree",
			wantBlock: true,
		},
		{
			name: "same write, no session ids at all — the registered quest is enough",
			mutate: func(p *IsolationParams) {
				p.SessionID = ""
				p.LeadSessionID = ""
			},
			wantBlock: true,
		},
		{
			name: "an unregistered session with known ids is still a non-lead",
			mutate: func(p *IsolationParams) {
				p.SessionIsRegisteredQuest = false
			},
			wantBlock: true,
		},
		{
			name: "the lead writing in the main tree from anywhere",
			mutate: func(p *IsolationParams) {
				p.SessionID = "lead"
				p.SessionIsRegisteredQuest = false
			},
		},
		{
			name: "a write that lands inside the teammate's own worktree",
			mutate: func(p *IsolationParams) {
				p.TargetTopLevel = "/wt/quest-1"
				p.FilePath = "/wt/quest-1/src/main.go"
			},
		},
		{
			name: "a target outside any repo we know",
			mutate: func(p *IsolationParams) {
				p.TargetTopLevel = ""
				p.FilePath = "/tmp/scratch.go"
			},
		},
		{
			name: "a non-mutating tool",
			mutate: func(p *IsolationParams) {
				p.ToolName = "Bash"
			},
		},
		{
			name: "no fellowship running",
			mutate: func(p *IsolationParams) {
				p.FellowshipActive = false
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := base
			if c.mutate != nil {
				c.mutate(&p)
			}
			if got := IsolationGuard(p); got.Block != c.wantBlock {
				t.Errorf("IsolationGuard block = %v, want %v (%s)", got.Block, c.wantBlock, got.Message)
			}
		})
	}
}

// The data directory is exempt so the lead can manage coordination files in its
// own tree — not so a teammate can reach into the main tree's from its worktree.
func TestIsolationGuard_CoordinationExemptionIsScopedToTheSessionsTree(t *testing.T) {
	teammate := IsolationParams{
		FellowshipActive:         true,
		MainRoot:                 "/repo",
		SessionTopLevel:          "/wt/quest-1",
		TargetTopLevel:           "/repo",
		ToolName:                 "Write",
		FilePath:                 "/repo/.fellowship/notes.md",
		DataDirName:              ".fellowship",
		SessionIsRegisteredQuest: true,
	}
	if got := IsolationGuard(teammate); !got.Block {
		t.Error("a teammate writing the main tree's data directory must be blocked")
	}

	lead := teammate
	lead.SessionTopLevel = "/repo"
	lead.SessionIsRegisteredQuest = false
	lead.SessionID = "lead"
	lead.LeadSessionID = "lead"
	if got := IsolationGuard(lead); got.Block {
		t.Errorf("the lead's own coordination write must be allowed: %s", got.Message)
	}

	// And the same exemption in the main tree covers .git and .claude.
	for _, path := range []string{"/repo/.git/config", "/repo/.claude/settings.local.json"} {
		p := lead
		p.FilePath = path
		if got := IsolationGuard(p); got.Block {
			t.Errorf("%s should be exempt in the main tree: %s", path, got.Message)
		}
	}
}
