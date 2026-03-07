package dashboard

import (
	"fmt"
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

func TestDiscoverQuests_FromFellowshipState(t *testing.T) {
	root := t.TempDir()

	// Create a fake worktree directory with tmp/quest-state.json
	worktreeDir := filepath.Join(root, "worktrees", "quest-auth")
	if err := os.MkdirAll(filepath.Join(worktreeDir, "tmp"), 0755); err != nil {
		t.Fatalf("creating worktree dir: %v", err)
	}

	questState := `{
  "version": 1,
  "quest_name": "quest-auth",
  "task_id": "t1",
  "team_name": "team",
  "phase": "Implement",
  "gate_pending": false,
  "gate_id": null,
  "lembas_completed": false,
  "metadata_updated": false,
  "auto_approve_gates": []
}`
	if err := os.WriteFile(filepath.Join(worktreeDir, "tmp", "quest-state.json"), []byte(questState), 0644); err != nil {
		t.Fatalf("writing quest-state.json: %v", err)
	}

	// Create fellowship-state.json pointing to that worktree
	if err := os.MkdirAll(filepath.Join(root, "tmp"), 0755); err != nil {
		t.Fatalf("creating tmp dir: %v", err)
	}
	fellowshipState := fmt.Sprintf(`{
  "name": "test-fellowship",
  "created_at": "2025-01-15T10:30:00Z",
  "quests": [
    {
      "name": "quest-auth",
      "worktree": %q,
      "task_id": "t1"
    }
  ],
  "scouts": []
}`, worktreeDir)
	if err := os.WriteFile(filepath.Join(root, "tmp", "fellowship-state.json"), []byte(fellowshipState), 0644); err != nil {
		t.Fatalf("writing fellowship-state.json: %v", err)
	}

	// Call DiscoverQuests
	status, err := DiscoverQuests(root)
	if err != nil {
		t.Fatalf("DiscoverQuests() error: %v", err)
	}

	if status.Name != "test-fellowship" {
		t.Errorf("Name = %q, want %q", status.Name, "test-fellowship")
	}
	if len(status.Quests) != 1 {
		t.Fatalf("len(Quests) = %d, want 1", len(status.Quests))
	}
	q := status.Quests[0]
	if q.Name != "quest-auth" {
		t.Errorf("Quest.Name = %q, want %q", q.Name, "quest-auth")
	}
	if q.Phase != "Implement" {
		t.Errorf("Quest.Phase = %q, want %q", q.Phase, "Implement")
	}
	if q.GatePending != false {
		t.Errorf("Quest.GatePending = %v, want false", q.GatePending)
	}
	if q.GateID != nil {
		t.Errorf("Quest.GateID = %v, want nil", q.GateID)
	}
	if q.Worktree != worktreeDir {
		t.Errorf("Quest.Worktree = %q, want %q", q.Worktree, worktreeDir)
	}
}

func TestDiscoverQuests_SkipsMissingWorktree(t *testing.T) {
	root := t.TempDir()

	// Create fellowship-state.json pointing to a non-existent worktree
	if err := os.MkdirAll(filepath.Join(root, "tmp"), 0755); err != nil {
		t.Fatalf("creating tmp dir: %v", err)
	}
	fellowshipState := `{
  "name": "test-fellowship",
  "created_at": "2025-01-15T10:30:00Z",
  "quests": [
    {
      "name": "quest-missing",
      "worktree": "/nonexistent/worktree",
      "task_id": "t1"
    }
  ],
  "scouts": []
}`
	if err := os.WriteFile(filepath.Join(root, "tmp", "fellowship-state.json"), []byte(fellowshipState), 0644); err != nil {
		t.Fatalf("writing fellowship-state.json: %v", err)
	}

	status, err := DiscoverQuests(root)
	if err != nil {
		t.Fatalf("DiscoverQuests() error: %v", err)
	}

	if len(status.Quests) != 0 {
		t.Errorf("len(Quests) = %d, want 0 (missing worktree should be skipped)", len(status.Quests))
	}
}

func TestLoadFellowshipState_Missing(t *testing.T) {
	_, err := LoadFellowshipState("/nonexistent/path/fellowship-state.json")
	if err == nil {
		t.Fatal("LoadFellowshipState() expected error for missing file, got nil")
	}
}
