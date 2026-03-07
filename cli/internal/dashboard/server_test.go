package dashboard

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func setupTestRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	// Create a fake worktree directory with tmp/quest-state.json
	worktreeDir := filepath.Join(root, "worktrees", "quest-login")
	if err := os.MkdirAll(filepath.Join(worktreeDir, "tmp"), 0755); err != nil {
		t.Fatalf("creating worktree dir: %v", err)
	}

	questState := `{
  "version": 1,
  "quest_name": "quest-login",
  "task_id": "t1",
  "team_name": "team",
  "phase": "Plan",
  "gate_pending": true,
  "gate_id": "gate-plan-review",
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
      "name": "quest-login",
      "worktree": %q,
      "task_id": "t1"
    }
  ],
  "scouts": []
}`, worktreeDir)
	if err := os.WriteFile(filepath.Join(root, "tmp", "fellowship-state.json"), []byte(fellowshipState), 0644); err != nil {
		t.Fatalf("writing fellowship-state.json: %v", err)
	}

	return root
}

func TestAPIStatus(t *testing.T) {
	root := setupTestRoot(t)
	srv := NewServer(root, 5)

	req := httptest.NewRequest("GET", "/api/status", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", contentType, "application/json")
	}

	var status DashboardStatus
	if err := json.NewDecoder(w.Body).Decode(&status); err != nil {
		t.Fatalf("decoding response: %v", err)
	}

	if status.Name != "test-fellowship" {
		t.Errorf("Name = %q, want %q", status.Name, "test-fellowship")
	}
	if status.PollInterval != 5 {
		t.Errorf("PollInterval = %d, want 5", status.PollInterval)
	}
	if len(status.Quests) != 1 {
		t.Fatalf("len(Quests) = %d, want 1", len(status.Quests))
	}

	q := status.Quests[0]
	if q.Name != "quest-login" {
		t.Errorf("Quest.Name = %q, want %q", q.Name, "quest-login")
	}
	if q.Phase != "Plan" {
		t.Errorf("Quest.Phase = %q, want %q", q.Phase, "Plan")
	}
	if q.GatePending != true {
		t.Errorf("Quest.GatePending = %v, want true", q.GatePending)
	}
	if q.GateID == nil || *q.GateID != "gate-plan-review" {
		t.Errorf("Quest.GateID = %v, want %q", q.GateID, "gate-plan-review")
	}
}

func TestAPIStatus_NotFound(t *testing.T) {
	root := setupTestRoot(t)
	srv := NewServer(root, 5)

	req := httptest.NewRequest("GET", "/api/nonexistent", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code == http.StatusOK {
		t.Errorf("expected non-200 for unknown route, got %d", w.Code)
	}
}
