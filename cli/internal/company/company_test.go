package company

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/justinjdev/fellowship/cli/internal/dashboard"
	"github.com/justinjdev/fellowship/cli/internal/db"
	"github.com/justinjdev/fellowship/cli/internal/herald"
	"github.com/justinjdev/fellowship/cli/internal/state"
	"github.com/justinjdev/fellowship/cli/internal/tome"
)

func TestCalculateProgress_MixedPhases(t *testing.T) {
	company := dashboard.CompanyEntry{
		Name:   "API Work",
		Quests: []string{"quest-endpoint", "quest-tests", "quest-docs"},
		Scouts: []string{"scout-review"},
	}

	quests := []dashboard.QuestStatus{
		{Name: "quest-endpoint", Phase: "Implement", GatePending: false},
		{Name: "quest-tests", Phase: "Complete", GatePending: false},
		{Name: "quest-docs", Phase: "Research", GatePending: true},
	}

	progress := CalculateProgress(company, quests)

	if progress.Name != "API Work" {
		t.Errorf("expected name 'API Work', got %q", progress.Name)
	}
	// Total includes quests + scouts
	if progress.Total != 4 {
		t.Errorf("expected total 4, got %d", progress.Total)
	}
	if progress.Completed != 1 {
		t.Errorf("expected 1 completed, got %d", progress.Completed)
	}
	// Implement+ includes Implement, Review, Complete
	if progress.InProgress != 2 {
		t.Errorf("expected 2 in_progress (Implement+), got %d", progress.InProgress)
	}
	if progress.Pending != 1 {
		t.Errorf("expected 1 pending gate, got %d", progress.Pending)
	}
}

func TestCalculateProgress_AllComplete(t *testing.T) {
	company := dashboard.CompanyEntry{
		Name:   "done-company",
		Quests: []string{"q1", "q2"},
	}
	quests := []dashboard.QuestStatus{
		{Name: "q1", Phase: "Complete"},
		{Name: "q2", Phase: "Complete"},
	}

	progress := CalculateProgress(company, quests)

	if progress.Completed != 2 {
		t.Errorf("expected 2 completed, got %d", progress.Completed)
	}
	if progress.Pending != 0 {
		t.Errorf("expected 0 pending, got %d", progress.Pending)
	}
}

func TestCalculateProgress_MissingQuests(t *testing.T) {
	company := dashboard.CompanyEntry{
		Name:   "sparse",
		Quests: []string{"exists", "missing"},
	}
	quests := []dashboard.QuestStatus{
		{Name: "exists", Phase: "Plan"},
	}

	progress := CalculateProgress(company, quests)

	// Missing quest should be gracefully skipped
	if progress.Completed != 0 {
		t.Errorf("expected 0 completed, got %d", progress.Completed)
	}
	if progress.InProgress != 0 {
		t.Errorf("expected 0 in_progress, got %d", progress.InProgress)
	}
}

func TestBatchApprove_MultipleQuests(t *testing.T) {
	d := db.OpenTest(t)
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		dashboard.InitFellowship(conn, "test", "/tmp", "main")
		dashboard.AddQuest(conn, dashboard.QuestEntry{Name: "q1", Worktree: "/tmp/wt1"})
		dashboard.AddQuest(conn, dashboard.QuestEntry{Name: "q2", Worktree: "/tmp/wt2"})
		dashboard.AddCompany(conn, "batch-test", []string{"q1", "q2"}, nil)

		state.Upsert(conn, &state.State{
			QuestName:   "q1",
			Phase:       "Research",
			GatePending: true,
		})
		state.Upsert(conn, &state.State{
			QuestName:   "q2",
			Phase:       "Plan",
			GatePending: true,
		})

		company := dashboard.CompanyEntry{
			Name:   "batch-test",
			Quests: []string{"q1", "q2"},
		}

		approved, errs := BatchApprove(conn, company)

		if len(errs) != 0 {
			t.Errorf("expected no errors, got %v", errs)
		}
		if len(approved) != 2 {
			t.Fatalf("expected 2 approved, got %d", len(approved))
		}

		// Verify phases were advanced
		s1, _ := state.Load(conn, "q1")
		if s1.Phase != "Plan" {
			t.Errorf("expected q1 phase 'Plan', got %q", s1.Phase)
		}
		if s1.GatePending {
			t.Error("expected q1 gate_pending to be false")
		}

		s2, _ := state.Load(conn, "q2")
		if s2.Phase != "Implement" {
			t.Errorf("expected q2 phase 'Implement', got %q", s2.Phase)
		}
		return nil
	})
}

func TestBatchApprove_NoPendingGates(t *testing.T) {
	d := db.OpenTest(t)
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		dashboard.InitFellowship(conn, "test", "/tmp", "main")
		dashboard.AddQuest(conn, dashboard.QuestEntry{Name: "q1", Worktree: "/tmp/wt"})

		state.Upsert(conn, &state.State{
			QuestName:   "q1",
			Phase:       "Implement",
			GatePending: false,
		})

		company := dashboard.CompanyEntry{
			Name:   "no-gates",
			Quests: []string{"q1"},
		}

		approved, errs := BatchApprove(conn, company)

		if len(errs) != 0 {
			t.Errorf("expected no errors, got %v", errs)
		}
		if len(approved) != 0 {
			t.Errorf("expected 0 approved (no-op), got %d", len(approved))
		}
		return nil
	})
}

func TestBatchApprove_MissingQuestState(t *testing.T) {
	d := db.OpenTest(t)
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		dashboard.InitFellowship(conn, "test", "/tmp", "main")
		// q1 has no quest_state row
		dashboard.AddQuest(conn, dashboard.QuestEntry{Name: "q1", Worktree: "/tmp/wt"})

		company := dashboard.CompanyEntry{
			Name:   "missing-state",
			Quests: []string{"q1", "q2"}, // q2 doesn't even exist in fellowship_quests
		}

		approved, errs := BatchApprove(conn, company)

		// Both should produce errors (can't load state)
		if len(approved) != 0 {
			t.Errorf("expected 0 approved, got %d", len(approved))
		}
		if len(errs) != 2 {
			t.Errorf("expected 2 errors, got %d: %v", len(errs), errs)
		}
		return nil
	})
}

func TestFindCompanyForQuest(t *testing.T) {
	companies := []dashboard.CompanyEntry{
		{Name: "API", Quests: []string{"q-api", "q-tests"}},
		{Name: "Docs", Quests: []string{"q-docs"}},
	}

	if got := FindCompanyForQuest(companies, "q-api"); got != "API" {
		t.Errorf("expected 'API', got %q", got)
	}
	if got := FindCompanyForQuest(companies, "q-docs"); got != "Docs" {
		t.Errorf("expected 'Docs', got %q", got)
	}
	if got := FindCompanyForQuest(companies, "q-other"); got != "" {
		t.Errorf("expected empty string for ungrouped quest, got %q", got)
	}
}

func TestProgressSummary(t *testing.T) {
	p := CompanyProgress{
		Name:       "API Work",
		Total:      3,
		InProgress: 2,
	}
	got := ProgressSummary(p)
	expected := "2/3 quests in Implement+"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestBatchApprove_HeraldLogging(t *testing.T) {
	d := db.OpenTest(t)
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		dashboard.InitFellowship(conn, "test", "/tmp", "main")
		dashboard.AddQuest(conn, dashboard.QuestEntry{Name: "q1", Worktree: "/tmp/wt1"})

		state.Upsert(conn, &state.State{
			QuestName:   "q1",
			Phase:       "Research",
			GatePending: true,
		})

		company := dashboard.CompanyEntry{
			Name:   "herald-test",
			Quests: []string{"q1"},
		}

		approved, errs := BatchApprove(conn, company)

		if len(errs) != 0 {
			t.Errorf("expected no errors, got %v", errs)
		}
		if len(approved) != 1 {
			t.Fatalf("expected 1 approved, got %d", len(approved))
		}

		tidings, err := herald.Read(conn, "q1", 0)
		if err != nil {
			t.Fatalf("reading herald: %v", err)
		}

		var foundApproved, foundTransition bool
		for _, td := range tidings {
			if td.Type == herald.GateApproved && td.Phase == "Research" {
				foundApproved = true
			}
			if td.Type == herald.PhaseTransition && td.Phase == "Plan" {
				foundTransition = true
			}
		}
		if !foundApproved {
			t.Error("expected GateApproved tiding for Research phase")
		}
		if !foundTransition {
			t.Error("expected PhaseTransition tiding for Plan phase")
		}
		return nil
	})
}

func TestBatchApprove_TomeRecording(t *testing.T) {
	d := db.OpenTest(t)
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		dashboard.InitFellowship(conn, "test", "/tmp", "main")
		dashboard.AddQuest(conn, dashboard.QuestEntry{Name: "q1", Worktree: "/tmp/wt1"})

		state.Upsert(conn, &state.State{
			QuestName:   "q1",
			Phase:       "Plan",
			GatePending: true,
		})

		company := dashboard.CompanyEntry{
			Name:   "tome-test",
			Quests: []string{"q1"},
		}

		approved, _ := BatchApprove(conn, company)
		if len(approved) != 1 {
			t.Fatalf("expected 1 approved, got %d", len(approved))
		}

		gates, err := tome.LoadGates(conn, "q1")
		if err != nil {
			t.Fatalf("loading gates: %v", err)
		}
		if len(gates) != 1 {
			t.Fatalf("expected 1 gate event, got %d", len(gates))
		}
		if gates[0].Action != "approved" {
			t.Errorf("expected action 'approved', got %q", gates[0].Action)
		}
		if gates[0].Phase != "Plan" {
			t.Errorf("expected phase 'Plan', got %q", gates[0].Phase)
		}

		phases, err := tome.LoadPhases(conn, "q1")
		if err != nil {
			t.Fatalf("loading phases: %v", err)
		}
		if len(phases) != 1 {
			t.Fatalf("expected 1 phase record, got %d", len(phases))
		}
		if phases[0].Phase != "Plan" {
			t.Errorf("expected phase 'Plan', got %q", phases[0].Phase)
		}
		return nil
	})
}

func TestList_NoCompanies(t *testing.T) {
	d := db.OpenTest(t)
	d.WithConn(context.Background(), func(conn *db.Conn) error {
		dashboard.InitFellowship(conn, "test", "/tmp", "main")
		// No companies — should print "No companies defined."
		err := List(conn)
		if err != nil {
			t.Fatalf("List() error: %v", err)
		}
		return nil
	})
}

func TestList_WithCompanies(t *testing.T) {
	d := db.OpenTest(t)
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		dashboard.InitFellowship(conn, "test", "/tmp", "main")
		dashboard.AddQuest(conn, dashboard.QuestEntry{Name: "q1", Worktree: "/tmp/wt1"})
		dashboard.AddCompany(conn, "team-alpha", []string{"q1"}, nil)

		err := List(conn)
		if err != nil {
			t.Fatalf("List() error: %v", err)
		}
		return nil
	})
}

func TestShow_CompanyNotFound(t *testing.T) {
	d := db.OpenTest(t)
	d.WithConn(context.Background(), func(conn *db.Conn) error {
		dashboard.InitFellowship(conn, "test", "/tmp", "main")
		err := Show(conn, "nonexistent")
		if err == nil {
			t.Fatal("expected error for nonexistent company")
		}
		return nil
	})
}

func TestShow_WithQuestState(t *testing.T) {
	d := db.OpenTest(t)
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		dashboard.InitFellowship(conn, "test", "/tmp", "main")
		dashboard.AddQuest(conn, dashboard.QuestEntry{Name: "q1", Worktree: "/tmp/wt1"})
		dashboard.AddCompany(conn, "team-alpha", []string{"q1"}, []string{})

		state.Upsert(conn, &state.State{
			QuestName:   "q1",
			Phase:       "Implement",
			GatePending: true,
		})

		err := Show(conn, "team-alpha")
		if err != nil {
			t.Fatalf("Show() error: %v", err)
		}
		return nil
	})
}

func TestApprove_CompanyNotFound(t *testing.T) {
	d := db.OpenTest(t)
	d.WithConn(context.Background(), func(conn *db.Conn) error {
		dashboard.InitFellowship(conn, "test", "/tmp", "main")
		err := Approve(conn, "nonexistent")
		if err == nil {
			t.Fatal("expected error for nonexistent company")
		}
		return nil
	})
}

func TestApprove_WithPendingGates(t *testing.T) {
	d := db.OpenTest(t)
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		dashboard.InitFellowship(conn, "test", "/tmp", "main")
		dashboard.AddQuest(conn, dashboard.QuestEntry{Name: "q1", Worktree: "/tmp/wt1"})
		dashboard.AddCompany(conn, "team-alpha", []string{"q1"}, nil)

		state.Upsert(conn, &state.State{
			QuestName:   "q1",
			Phase:       "Research",
			GatePending: true,
		})

		err := Approve(conn, "team-alpha")
		if err != nil {
			t.Fatalf("Approve() error: %v", err)
		}

		// Verify state was advanced
		s, _ := state.Load(conn, "q1")
		if s.Phase != "Plan" {
			t.Errorf("expected phase 'Plan', got %q", s.Phase)
		}
		return nil
	})
}

func TestLoadAndMarshalProgress(t *testing.T) {
	d := db.OpenTest(t)
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		dashboard.InitFellowship(conn, "test", "/tmp", "main")
		dashboard.AddQuest(conn, dashboard.QuestEntry{Name: "q1", Worktree: "/tmp/wt1"})
		dashboard.AddQuest(conn, dashboard.QuestEntry{Name: "q2", Worktree: "/tmp/wt2"})
		dashboard.AddCompany(conn, "team-alpha", []string{"q1", "q2"}, nil)

		state.Upsert(conn, &state.State{QuestName: "q1", Phase: "Implement"})
		state.Upsert(conn, &state.State{QuestName: "q2", Phase: "Complete"})

		data, err := LoadAndMarshalProgress(conn, "team-alpha")
		if err != nil {
			t.Fatalf("LoadAndMarshalProgress() error: %v", err)
		}

		var progress CompanyProgress
		if err := json.Unmarshal(data, &progress); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}

		if progress.Name != "team-alpha" {
			t.Errorf("Name = %q, want %q", progress.Name, "team-alpha")
		}
		if progress.Total != 2 {
			t.Errorf("Total = %d, want 2", progress.Total)
		}
		if progress.Completed != 1 {
			t.Errorf("Completed = %d, want 1", progress.Completed)
		}
		if progress.InProgress != 2 { // Implement + Complete both >= 3
			t.Errorf("InProgress = %d, want 2", progress.InProgress)
		}
		return nil
	})
}

func TestLoadAndMarshalProgress_NotFound(t *testing.T) {
	d := db.OpenTest(t)
	d.WithConn(context.Background(), func(conn *db.Conn) error {
		dashboard.InitFellowship(conn, "test", "/tmp", "main")
		_, err := LoadAndMarshalProgress(conn, "nonexistent")
		if err == nil {
			t.Fatal("expected error for nonexistent company")
		}
		return nil
	})
}
