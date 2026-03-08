package dashboard

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupTestRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	// Create a fake worktree directory with .fellowship/quest-state.json
	worktreeDir := filepath.Join(root, "worktrees", "quest-login")
	if err := os.MkdirAll(filepath.Join(worktreeDir, ".fellowship"), 0755); err != nil {
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
	if err := os.WriteFile(filepath.Join(worktreeDir, ".fellowship", "quest-state.json"), []byte(questState), 0644); err != nil {
		t.Fatalf("writing quest-state.json: %v", err)
	}

	// Create fellowship-state.json pointing to that worktree
	if err := os.MkdirAll(filepath.Join(root, ".fellowship"), 0755); err != nil {
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
	if err := os.WriteFile(filepath.Join(root, ".fellowship", "fellowship-state.json"), []byte(fellowshipState), 0644); err != nil {
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

func TestAPIGateApprove(t *testing.T) {
	root := setupTestRoot(t)
	srv := NewServer(root, 5)

	worktreeDir := filepath.Join(root, "worktrees", "quest-login")
	body := strings.NewReader(fmt.Sprintf(`{"dir":%q}`, worktreeDir))
	req := httptest.NewRequest("POST", "/api/gate/approve", body)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var qs QuestStatus
	if err := json.NewDecoder(w.Body).Decode(&qs); err != nil {
		t.Fatalf("decoding response: %v", err)
	}

	if qs.Phase != "Implement" {
		t.Errorf("Phase = %q, want %q", qs.Phase, "Implement")
	}
	if qs.GatePending {
		t.Errorf("GatePending = true, want false")
	}
	if qs.GateID != nil {
		t.Errorf("GateID = %v, want nil", qs.GateID)
	}
}

func TestAPIGateReject(t *testing.T) {
	root := setupTestRoot(t)
	srv := NewServer(root, 5)

	worktreeDir := filepath.Join(root, "worktrees", "quest-login")
	body := strings.NewReader(fmt.Sprintf(`{"dir":%q}`, worktreeDir))
	req := httptest.NewRequest("POST", "/api/gate/reject", body)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var qs QuestStatus
	if err := json.NewDecoder(w.Body).Decode(&qs); err != nil {
		t.Fatalf("decoding response: %v", err)
	}

	if qs.Phase != "Plan" {
		t.Errorf("Phase = %q, want %q", qs.Phase, "Plan")
	}
	if qs.GatePending {
		t.Errorf("GatePending = true, want false")
	}
	if qs.GateID != nil {
		t.Errorf("GateID = %v, want nil", qs.GateID)
	}
}

func TestAPIGateApprove_NoPending(t *testing.T) {
	root := setupTestRoot(t)
	srv := NewServer(root, 5)

	// Overwrite quest-state.json with gate_pending: false
	worktreeDir := filepath.Join(root, "worktrees", "quest-login")
	questState := `{
  "version": 1,
  "quest_name": "quest-login",
  "task_id": "t1",
  "team_name": "team",
  "phase": "Plan",
  "gate_pending": false,
  "gate_id": null,
  "lembas_completed": false,
  "metadata_updated": false,
  "auto_approve_gates": []
}`
	if err := os.WriteFile(filepath.Join(worktreeDir, ".fellowship", "quest-state.json"), []byte(questState), 0644); err != nil {
		t.Fatalf("writing quest-state.json: %v", err)
	}

	body := strings.NewReader(fmt.Sprintf(`{"dir":%q}`, worktreeDir))
	req := httptest.NewRequest("POST", "/api/gate/approve", body)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status code = %d, want %d; body: %s", w.Code, http.StatusBadRequest, w.Body.String())
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
