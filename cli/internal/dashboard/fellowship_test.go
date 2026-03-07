package dashboard

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFellowshipState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fellowship-state.json")

	data := `{
  "name": "test-fellowship",
  "created_at": "2025-01-15T10:30:00Z",
  "quests": [
    {
      "name": "add-auth",
      "worktree": "/tmp/worktrees/add-auth",
      "task_id": "task-001"
    },
    {
      "name": "fix-bug",
      "worktree": "/tmp/worktrees/fix-bug",
      "task_id": "task-002"
    }
  ],
  "scouts": [
    {
      "name": "research-api",
      "task_id": "task-003"
    }
  ]
}`

	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	state, err := LoadFellowshipState(path)
	if err != nil {
		t.Fatalf("LoadFellowshipState() error: %v", err)
	}

	if state.Name != "test-fellowship" {
		t.Errorf("Name = %q, want %q", state.Name, "test-fellowship")
	}
	if state.CreatedAt != "2025-01-15T10:30:00Z" {
		t.Errorf("CreatedAt = %q, want %q", state.CreatedAt, "2025-01-15T10:30:00Z")
	}
	if len(state.Quests) != 2 {
		t.Fatalf("len(Quests) = %d, want 2", len(state.Quests))
	}
	if state.Quests[0].Name != "add-auth" {
		t.Errorf("Quests[0].Name = %q, want %q", state.Quests[0].Name, "add-auth")
	}
	if state.Quests[0].Worktree != "/tmp/worktrees/add-auth" {
		t.Errorf("Quests[0].Worktree = %q, want %q", state.Quests[0].Worktree, "/tmp/worktrees/add-auth")
	}
	if state.Quests[0].TaskID != "task-001" {
		t.Errorf("Quests[0].TaskID = %q, want %q", state.Quests[0].TaskID, "task-001")
	}
	if state.Quests[1].Name != "fix-bug" {
		t.Errorf("Quests[1].Name = %q, want %q", state.Quests[1].Name, "fix-bug")
	}
	if len(state.Scouts) != 1 {
		t.Fatalf("len(Scouts) = %d, want 1", len(state.Scouts))
	}
	if state.Scouts[0].Name != "research-api" {
		t.Errorf("Scouts[0].Name = %q, want %q", state.Scouts[0].Name, "research-api")
	}
	if state.Scouts[0].TaskID != "task-003" {
		t.Errorf("Scouts[0].TaskID = %q, want %q", state.Scouts[0].TaskID, "task-003")
	}
}

func TestLoadFellowshipState_Missing(t *testing.T) {
	_, err := LoadFellowshipState("/nonexistent/path/fellowship-state.json")
	if err == nil {
		t.Fatal("LoadFellowshipState() expected error for missing file, got nil")
	}
}
