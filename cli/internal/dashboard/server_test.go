package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/justinjdev/fellowship/cli/internal/db"
	"github.com/justinjdev/fellowship/cli/internal/herald"
	"github.com/justinjdev/fellowship/cli/internal/state"
)

func setupTestDB(t *testing.T) (*db.DB, string) {
	t.Helper()
	d := db.OpenTest(t)
	worktreeDir := "/tmp/test-worktrees/quest-login"

	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		if err := InitFellowship(conn, "test-fellowship", "/tmp/repo", "main"); err != nil {
			return err
		}
		if err := AddQuest(conn, QuestEntry{
			Name:     "quest-login",
			Worktree: worktreeDir,
			TaskID:   "t1",
		}); err != nil {
			return err
		}
		gateID := "gate-plan-review"
		if err := state.Upsert(conn, &state.State{
			QuestName:   "quest-login",
			TaskID:      "t1",
			TeamName:    "team",
			Phase:       "Plan",
			GatePending: true,
			GateID:      &gateID,
		}); err != nil {
			return err
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	return d, worktreeDir
}

func TestAPIStatus(t *testing.T) {
	d, _ := setupTestDB(t)
	srv := NewServer(d, 5)

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
	d, worktreeDir := setupTestDB(t)
	srv := NewServer(d, 5)

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
	d, worktreeDir := setupTestDB(t)
	srv := NewServer(d, 5)

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
	d, worktreeDir := setupTestDB(t)

	// Override quest state with gate_pending: false
	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		return state.Upsert(conn, &state.State{
			QuestName:   "quest-login",
			TaskID:      "t1",
			TeamName:    "team",
			Phase:       "Plan",
			GatePending: false,
		})
	}); err != nil {
		t.Fatal(err)
	}

	srv := NewServer(d, 5)

	body := strings.NewReader(fmt.Sprintf(`{"dir":%q}`, worktreeDir))
	req := httptest.NewRequest("POST", "/api/gate/approve", body)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status code = %d, want %d; body: %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

func TestAPIGateApprove_HeraldLogging(t *testing.T) {
	d, worktreeDir := setupTestDB(t)
	srv := NewServer(d, 5)

	body := strings.NewReader(fmt.Sprintf(`{"dir":%q}`, worktreeDir))
	req := httptest.NewRequest("POST", "/api/gate/approve", body)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	// Read herald entries from DB
	var tidings []herald.Tiding
	if err := d.WithConn(context.Background(), func(conn *db.Conn) error {
		var err error
		tidings, err = herald.Read(conn, "quest-login", 0)
		return err
	}); err != nil {
		t.Fatal(err)
	}

	if len(tidings) < 2 {
		t.Fatalf("expected at least 2 tidings (GateApproved + PhaseTransition), got %d", len(tidings))
	}

	var foundApproved, foundTransition bool
	for _, td := range tidings {
		if td.Type == herald.GateApproved && td.Phase == "Plan" {
			foundApproved = true
		}
		if td.Type == herald.PhaseTransition && td.Phase == "Implement" {
			foundTransition = true
		}
	}
	if !foundApproved {
		t.Error("expected GateApproved tiding for Plan phase")
	}
	if !foundTransition {
		t.Error("expected PhaseTransition tiding for Implement phase")
	}
}

func TestAPIGateReject_HeraldLogging(t *testing.T) {
	d, worktreeDir := setupTestDB(t)
	srv := NewServer(d, 5)

	body := strings.NewReader(fmt.Sprintf(`{"dir":%q}`, worktreeDir))
	req := httptest.NewRequest("POST", "/api/gate/reject", body)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var tidings []herald.Tiding
	if err := d.WithConn(context.Background(), func(conn *db.Conn) error {
		var err error
		tidings, err = herald.Read(conn, "quest-login", 0)
		return err
	}); err != nil {
		t.Fatal(err)
	}

	var foundRejected bool
	for _, td := range tidings {
		if td.Type == herald.GateRejected && td.Phase == "Plan" {
			foundRejected = true
		}
	}
	if !foundRejected {
		t.Error("expected GateRejected tiding for Plan phase")
	}
}

func TestAPIStatus_NotFound(t *testing.T) {
	d, _ := setupTestDB(t)
	srv := NewServer(d, 5)

	req := httptest.NewRequest("GET", "/api/nonexistent", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code == http.StatusOK {
		t.Errorf("expected non-200 for unknown route, got %d", w.Code)
	}
}
