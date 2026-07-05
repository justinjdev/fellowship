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
		})
		if result.Block {
			t.Errorf("coordination-dir write must be allowed: %s", path)
		}
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
