package hooks

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestIsolationGuard_AllowsInWorktree(t *testing.T) {
	// Session top-level is a worktree, distinct from the main root: never blocked.
	result := IsolationGuard(IsolationParams{
		FellowshipActive: true,
		MainRoot:         "/repo",
		SessionTopLevel:  "/repo/.worktrees/quest-1",
		ToolName:         "Write",
		FilePath:         "/repo/.worktrees/quest-1/src/main.go",
	})
	if result.Block {
		t.Errorf("teammate in its own worktree must be allowed, got: %s", result.Message)
	}
}

func TestIsolationGuard_BlocksMainTreeSourceWrite(t *testing.T) {
	for _, tool := range []string{"Edit", "Write", "NotebookEdit"} {
		result := IsolationGuard(IsolationParams{
			FellowshipActive: true,
			MainRoot:         "/repo",
			SessionTopLevel:  "/repo",
			ToolName:         tool,
			FilePath:         "/repo/src/main.go",
			// A session that is not the recorded lead.
			SessionID:     "teammate-session",
			LeadSessionID: "lead-session",
		})
		if !result.Block {
			t.Errorf("main-tree source write via %s during active fellowship must block", tool)
		}
		if !strings.Contains(result.Message, "src/main.go") {
			t.Errorf("message should name the offending file, got: %s", result.Message)
		}
	}
}

func TestIsolationGuard_AllowsCoordinationDirWrite(t *testing.T) {
	for _, path := range []string{
		"/repo/.fellowship/checkpoint.md",
		"/repo/.git/COMMIT_EDITMSG",
		"/repo/.claude/settings.json",
	} {
		result := IsolationGuard(IsolationParams{
			FellowshipActive: true,
			MainRoot:         "/repo",
			SessionTopLevel:  "/repo",
			ToolName:         "Write",
			FilePath:         path,
			DataDirName:      ".fellowship",
		})
		if result.Block {
			t.Errorf("coordination-dir write must be allowed: %s", path)
		}
	}
}

func TestIsolationGuard_AllowsCustomDataDirWrite(t *testing.T) {
	// A user-configured dataDir must be exempt just like the default.
	result := IsolationGuard(IsolationParams{
		FellowshipActive: true,
		MainRoot:         "/repo",
		SessionTopLevel:  "/repo",
		ToolName:         "Write",
		FilePath:         "/repo/queststate/checkpoint.md",
		DataDirName:      "queststate",
	})
	if result.Block {
		t.Error("write under a custom dataDir must be allowed")
	}
}

func TestIsolationGuard_BlocksDefaultDataDirNameWhenCustomConfigured(t *testing.T) {
	// If the user renamed the data dir, ".fellowship" is now an ordinary source
	// path in the main tree and must be blocked.
	result := IsolationGuard(IsolationParams{
		FellowshipActive: true,
		MainRoot:         "/repo",
		SessionTopLevel:  "/repo",
		ToolName:         "Write",
		FilePath:         "/repo/.fellowship/notes.md",
		DataDirName:      "queststate",
		SessionID:        "teammate-session",
		LeadSessionID:    "lead-session",
	})
	if !result.Block {
		t.Error("stale .fellowship path should not be exempt when dataDir is customized")
	}
}

// The main working tree is the lead's own workspace: its session must be able
// to write there while quests are running.
func TestIsolationGuard_AllowsLeadSession(t *testing.T) {
	result := IsolationGuard(IsolationParams{
		FellowshipActive: true,
		MainRoot:         "/repo",
		SessionTopLevel:  "/repo",
		ToolName:         "Write",
		FilePath:         "/repo/src/main.go",
		SessionID:        "lead-session",
		LeadSessionID:    "lead-session",
	})
	if result.Block {
		t.Errorf("the lead's own session must not be blocked in the main tree, got: %s", result.Message)
	}
}

// A quest registered against the main root is a positive mis-placement even
// when no session id is available on either side.
func TestIsolationGuard_BlocksQuestProvisionedInMainRoot(t *testing.T) {
	result := IsolationGuard(IsolationParams{
		FellowshipActive:         true,
		MainRoot:                 "/repo",
		SessionTopLevel:          "/repo",
		ToolName:                 "Write",
		FilePath:                 "/repo/src/main.go",
		SessionIsRegisteredQuest: true,
	})
	if !result.Block {
		t.Error("a quest worktree that resolves to the main root must be blocked")
	}
	if !strings.Contains(result.Message, "registered as a quest") {
		t.Errorf("message should explain the detection, got: %s", result.Message)
	}
}

// With no lead marker or no payload session id there is nothing to identify the
// writer, and the guard is a fail-open backstop: it allows.
func TestIsolationGuard_AllowsUnidentifiedWriter(t *testing.T) {
	cases := []struct {
		name          string
		sessionID     string
		leadSessionID string
	}{
		{name: "no ids at all"},
		{name: "payload id but no marker", sessionID: "some-session"},
		{name: "marker but no payload id", leadSessionID: "lead-session"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			result := IsolationGuard(IsolationParams{
				FellowshipActive: true,
				MainRoot:         "/repo",
				SessionTopLevel:  "/repo",
				ToolName:         "Write",
				FilePath:         "/repo/src/main.go",
				SessionID:        c.sessionID,
				LeadSessionID:    c.leadSessionID,
			})
			if result.Block {
				t.Errorf("an unidentifiable writer must be allowed, got: %s", result.Message)
			}
		})
	}
}

func TestIsolationGuard_AllowsWhenNoFellowship(t *testing.T) {
	result := IsolationGuard(IsolationParams{
		FellowshipActive: false,
		MainRoot:         "/repo",
		SessionTopLevel:  "/repo",
		ToolName:         "Write",
		FilePath:         "/repo/src/main.go",
	})
	if result.Block {
		t.Error("guard must be inert when no fellowship is active")
	}
}

func TestIsolationGuard_AllowsNonMutatingTool(t *testing.T) {
	for _, tool := range []string{"Bash", "Read", "Grep", "Glob", ""} {
		result := IsolationGuard(IsolationParams{
			FellowshipActive: true,
			MainRoot:         "/repo",
			SessionTopLevel:  "/repo",
			ToolName:         tool,
			FilePath:         "/repo/src/main.go",
		})
		if result.Block {
			t.Errorf("non-mutating tool %q must be allowed", tool)
		}
	}
}

func TestIsolationGuard_AllowsNotebookPathInWorktree(t *testing.T) {
	result := IsolationGuard(IsolationParams{
		FellowshipActive: true,
		MainRoot:         "/repo",
		SessionTopLevel:  "/repo/.worktrees/quest-2",
		ToolName:         "NotebookEdit",
		FilePath:         "/repo/.worktrees/quest-2/analysis.ipynb",
	})
	if result.Block {
		t.Error("NotebookEdit inside a worktree must be allowed")
	}
}

func TestIsolationGuard_AllowsWriteOutsideMainRoot(t *testing.T) {
	// Some other repo/path that is not under the main root.
	result := IsolationGuard(IsolationParams{
		FellowshipActive: true,
		MainRoot:         "/repo",
		SessionTopLevel:  "/repo",
		ToolName:         "Write",
		FilePath:         "/tmp/scratch.txt",
	})
	if result.Block {
		t.Error("writes outside the main worktree are not the guard's concern")
	}
}

func TestIsolationGuard_AllowsEmptyFilePath(t *testing.T) {
	result := IsolationGuard(IsolationParams{
		FellowshipActive: true,
		MainRoot:         "/repo",
		SessionTopLevel:  "/repo",
		ToolName:         "Write",
		FilePath:         "",
	})
	if result.Block {
		t.Error("empty file path must be allowed")
	}
}

func TestRelWithin(t *testing.T) {
	cases := []struct {
		root, target string
		wantRel      string
		wantOK       bool
	}{
		{"/repo", "/repo/src/main.go", "src/main.go", true},
		{"/repo", "/repo", "", false},
		{"/repo", "/other/file.go", "", false},
		{"/repo", "/repository/file.go", "", false}, // prefix but not a child
	}
	for _, c := range cases {
		rel, ok := relWithin(c.root, c.target)
		if ok != c.wantOK || rel != c.wantRel {
			t.Errorf("relWithin(%q,%q) = (%q,%v), want (%q,%v)",
				c.root, c.target, rel, ok, c.wantRel, c.wantOK)
		}
	}
}

func TestSamePath(t *testing.T) {
	if !samePath("/repo", "/repo/") {
		t.Error("trailing slash should compare equal after clean")
	}
	if samePath("/repo", "/repo/sub") {
		t.Error("distinct paths must not be equal")
	}
	if samePath("", "/repo") {
		t.Error("empty path must not match")
	}
	// filepath.Clean is platform-aware; sanity check the join case.
	if !samePath(filepath.Join("/repo", "a", ".."), "/repo") {
		t.Error("normalized paths should compare equal")
	}
}

// A teammate spawned with the Agent tool runs in-process and shares the lead's
// session id; only the agent id in its hook payload tells it apart. Such a
// payload writing source into the main tree is a mis-placed teammate, never
// the lead — whatever session id it carries.
func TestIsolationGuard_SubagentSharingTheLeadSessionIsNotTheLead(t *testing.T) {
	result := IsolationGuard(IsolationParams{
		FellowshipActive: true,
		MainRoot:         "/repo",
		SessionTopLevel:  "/repo",
		ToolName:         "Write",
		FilePath:         "/repo/src/main.go",
		SessionID:        "lead-session",
		AgentID:          "agent-7",
		LeadSessionID:    "lead-session",
	})
	if !result.Block {
		t.Fatal("a subagent payload carrying the lead's session id must be blocked in the main tree")
	}
	if !strings.Contains(result.Message, "subagent") {
		t.Errorf("message should name the subagent detection, got: %s", result.Message)
	}
}

// The same subagent, with no lead recorded at all, is still identifiable as a
// subagent and still blocked: rule 3 does not need the lead's id for it.
func TestIsolationGuard_SubagentBlockedWithoutRecordedLead(t *testing.T) {
	result := IsolationGuard(IsolationParams{
		FellowshipActive: true,
		MainRoot:         "/repo",
		SessionTopLevel:  "/repo",
		ToolName:         "Write",
		FilePath:         "/repo/src/main.go",
		SessionID:        "some-session",
		AgentID:          "agent-7",
	})
	if !result.Block {
		t.Error("a subagent payload must be blocked in the main tree even with no lead recorded")
	}
}

// A subagent of the lead may still write coordination files in the main tree
// (a scout's findings under the data directory): the exemption is about the
// path, not the writer.
func TestIsolationGuard_SubagentMayWriteCoordinationPath(t *testing.T) {
	result := IsolationGuard(IsolationParams{
		FellowshipActive: true,
		MainRoot:         "/repo",
		SessionTopLevel:  "/repo",
		ToolName:         "Write",
		FilePath:         "/repo/.fellowship/scout-findings-scout-auth.md",
		DataDirName:      ".fellowship",
		SessionID:        "lead-session",
		AgentID:          "agent-7",
		LeadSessionID:    "lead-session",
	})
	if result.Block {
		t.Errorf("a subagent writing under the data directory must be allowed, got: %s", result.Message)
	}
}
