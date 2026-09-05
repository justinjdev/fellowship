package group

import (
	"context"
	"testing"

	"github.com/justinjdev/fellowship/cli/internal/db"
	"github.com/justinjdev/fellowship/cli/internal/events"
	"github.com/justinjdev/fellowship/cli/internal/fellowship"
	"github.com/justinjdev/fellowship/cli/internal/history"
	"github.com/justinjdev/fellowship/cli/internal/state"
)

func TestCalculateProgress_MixedPhases(t *testing.T) {
	grp := fellowship.GroupEntry{
		Name:   "API Work",
		Quests: []string{"quest-endpoint", "quest-tests", "quest-docs"},
		Scouts: []string{"scout-review"},
	}

	quests := []fellowship.QuestStatus{
		{Name: "quest-endpoint", Phase: "Implement", GatePending: false, Status: "active"},
		{Name: "quest-tests", Phase: "Review", GatePending: false, Status: "completed"},
		{Name: "quest-docs", Phase: "Research", GatePending: true, Status: "active"},
	}

	progress := CalculateProgress(grp, quests)

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
	// Implement+ includes Implement and Review
	if progress.InProgress != 2 {
		t.Errorf("expected 2 in_progress (Implement+), got %d", progress.InProgress)
	}
	if progress.Pending != 1 {
		t.Errorf("expected 1 pending gate, got %d", progress.Pending)
	}
}

func TestCalculateProgress_AllComplete(t *testing.T) {
	grp := fellowship.GroupEntry{
		Name:   "done-grp",
		Quests: []string{"q1", "q2"},
	}
	quests := []fellowship.QuestStatus{
		{Name: "q1", Phase: "Review", Status: "completed"},
		{Name: "q2", Phase: "Review", Status: "completed"},
	}

	progress := CalculateProgress(grp, quests)

	if progress.Completed != 2 {
		t.Errorf("expected 2 completed, got %d", progress.Completed)
	}
	if progress.Pending != 0 {
		t.Errorf("expected 0 pending, got %d", progress.Pending)
	}
}

func TestCalculateProgress_MissingQuests(t *testing.T) {
	grp := fellowship.GroupEntry{
		Name:   "sparse",
		Quests: []string{"exists", "missing"},
	}
	quests := []fellowship.QuestStatus{
		{Name: "exists", Phase: "Plan"},
	}

	progress := CalculateProgress(grp, quests)

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
	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		if err := fellowship.InitFellowship(conn, "test", "/tmp", "main"); err != nil {
			return err
		}
		if err := fellowship.AddQuest(conn, fellowship.QuestEntry{Name: "q1", Worktree: "/tmp/wt1"}); err != nil {
			return err
		}
		if err := fellowship.AddQuest(conn, fellowship.QuestEntry{Name: "q2", Worktree: "/tmp/wt2"}); err != nil {
			return err
		}
		if err := fellowship.AddGroup(conn, "batch-test", []string{"q1", "q2"}, nil); err != nil {
			return err
		}

		if err := state.Upsert(conn, &state.State{
			QuestName:   "q1",
			Phase:       "Research",
			GatePending: true,
		}); err != nil {
			return err
		}
		if err := state.Upsert(conn, &state.State{
			QuestName:   "q2",
			Phase:       "Plan",
			GatePending: true,
		}); err != nil {
			return err
		}

		grp := fellowship.GroupEntry{
			Name:   "batch-test",
			Quests: []string{"q1", "q2"},
		}

		approved, errs := BatchApprove(conn, grp)

		if len(errs) != 0 {
			t.Errorf("expected no errors, got %v", errs)
		}
		if len(approved) != 2 {
			t.Fatalf("expected 2 approved, got %d", len(approved))
		}

		// Verify phases were advanced
		s1, err := state.Load(conn, "q1")
		if err != nil {
			t.Fatal(err)
		}
		if s1.Phase != "Plan" {
			t.Errorf("expected q1 phase 'Plan', got %q", s1.Phase)
		}
		if s1.GatePending {
			t.Error("expected q1 gate_pending to be false")
		}

		s2, err := state.Load(conn, "q2")
		if err != nil {
			t.Fatal(err)
		}
		if s2.Phase != "Implement" {
			t.Errorf("expected q2 phase 'Implement', got %q", s2.Phase)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestBatchApprove_NoPendingGates(t *testing.T) {
	d := db.OpenTest(t)
	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		if err := fellowship.InitFellowship(conn, "test", "/tmp", "main"); err != nil {
			return err
		}
		if err := fellowship.AddQuest(conn, fellowship.QuestEntry{Name: "q1", Worktree: "/tmp/wt"}); err != nil {
			return err
		}

		if err := state.Upsert(conn, &state.State{
			QuestName:   "q1",
			Phase:       "Implement",
			GatePending: false,
		}); err != nil {
			return err
		}

		grp := fellowship.GroupEntry{
			Name:   "no-gates",
			Quests: []string{"q1"},
		}

		approved, errs := BatchApprove(conn, grp)

		if len(errs) != 0 {
			t.Errorf("expected no errors, got %v", errs)
		}
		if len(approved) != 0 {
			t.Errorf("expected 0 approved (no-op), got %d", len(approved))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestBatchApprove_MissingQuestState(t *testing.T) {
	d := db.OpenTest(t)
	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		if err := fellowship.InitFellowship(conn, "test", "/tmp", "main"); err != nil {
			return err
		}
		// q1 has no quest_state row
		if err := fellowship.AddQuest(conn, fellowship.QuestEntry{Name: "q1", Worktree: "/tmp/wt"}); err != nil {
			return err
		}

		grp := fellowship.GroupEntry{
			Name:   "missing-state",
			Quests: []string{"q1", "q2"}, // q2 doesn't even exist in fellowship_quests
		}

		approved, errs := BatchApprove(conn, grp)

		// Both should produce errors (can't load state)
		if len(approved) != 0 {
			t.Errorf("expected 0 approved, got %d", len(approved))
		}
		if len(errs) != 2 {
			t.Errorf("expected 2 errors, got %d: %v", len(errs), errs)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestBatchApprove_EventLogging(t *testing.T) {
	d := db.OpenTest(t)
	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		if err := fellowship.InitFellowship(conn, "test", "/tmp", "main"); err != nil {
			return err
		}
		if err := fellowship.AddQuest(conn, fellowship.QuestEntry{Name: "q1", Worktree: "/tmp/wt1"}); err != nil {
			return err
		}

		if err := state.Upsert(conn, &state.State{
			QuestName:   "q1",
			Phase:       "Research",
			GatePending: true,
		}); err != nil {
			return err
		}

		grp := fellowship.GroupEntry{
			Name:   "events-test",
			Quests: []string{"q1"},
		}

		approved, errs := BatchApprove(conn, grp)

		if len(errs) != 0 {
			t.Errorf("expected no errors, got %v", errs)
		}
		if len(approved) != 1 {
			t.Fatalf("expected 1 approved, got %d", len(approved))
		}

		tidings, err := events.Read(conn, "q1", 0)
		if err != nil {
			t.Fatalf("reading events: %v", err)
		}

		var foundApproved, foundTransition bool
		for _, td := range tidings {
			if td.Type == events.GateApproved && td.Phase == "Research" {
				foundApproved = true
			}
			if td.Type == events.PhaseTransition && td.Phase == "Plan" {
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
	}); err != nil {
		t.Fatal(err)
	}
}

func TestBatchApprove_HistoryRecording(t *testing.T) {
	d := db.OpenTest(t)
	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		if err := fellowship.InitFellowship(conn, "test", "/tmp", "main"); err != nil {
			return err
		}
		if err := fellowship.AddQuest(conn, fellowship.QuestEntry{Name: "q1", Worktree: "/tmp/wt1"}); err != nil {
			return err
		}

		if err := state.Upsert(conn, &state.State{
			QuestName:   "q1",
			Phase:       "Plan",
			GatePending: true,
		}); err != nil {
			return err
		}

		grp := fellowship.GroupEntry{
			Name:   "history-test",
			Quests: []string{"q1"},
		}

		approved, _ := BatchApprove(conn, grp)
		if len(approved) != 1 {
			t.Fatalf("expected 1 approved, got %d", len(approved))
		}

		gates, err := history.LoadGates(conn, "q1")
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

		phases, err := history.LoadPhases(conn, "q1")
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
	}); err != nil {
		t.Fatal(err)
	}
}

func TestList_NoGroups(t *testing.T) {
	d := db.OpenTest(t)
	if err := d.WithConn(context.Background(), func(conn *db.Conn) error {
		if err := fellowship.InitFellowship(conn, "test", "/tmp", "main"); err != nil {
			return err
		}
		// No groups — should print "No groups defined."
		err := List(conn)
		if err != nil {
			t.Fatalf("List() error: %v", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestList_WithGroups(t *testing.T) {
	d := db.OpenTest(t)
	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		if err := fellowship.InitFellowship(conn, "test", "/tmp", "main"); err != nil {
			return err
		}
		if err := fellowship.AddQuest(conn, fellowship.QuestEntry{Name: "q1", Worktree: "/tmp/wt1"}); err != nil {
			return err
		}
		if err := fellowship.AddGroup(conn, "team-alpha", []string{"q1"}, nil); err != nil {
			return err
		}

		err := List(conn)
		if err != nil {
			t.Fatalf("List() error: %v", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestShow_GroupNotFound(t *testing.T) {
	d := db.OpenTest(t)
	if err := d.WithConn(context.Background(), func(conn *db.Conn) error {
		if err := fellowship.InitFellowship(conn, "test", "/tmp", "main"); err != nil {
			return err
		}
		err := Show(conn, "nonexistent")
		if err == nil {
			t.Fatal("expected error for nonexistent grp")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestShow_WithQuestState(t *testing.T) {
	d := db.OpenTest(t)
	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		if err := fellowship.InitFellowship(conn, "test", "/tmp", "main"); err != nil {
			return err
		}
		if err := fellowship.AddQuest(conn, fellowship.QuestEntry{Name: "q1", Worktree: "/tmp/wt1"}); err != nil {
			return err
		}
		if err := fellowship.AddGroup(conn, "team-alpha", []string{"q1"}, []string{}); err != nil {
			return err
		}

		if err := state.Upsert(conn, &state.State{
			QuestName:   "q1",
			Phase:       "Implement",
			GatePending: true,
		}); err != nil {
			return err
		}

		err := Show(conn, "team-alpha")
		if err != nil {
			t.Fatalf("Show() error: %v", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestLoadDetail_WithQuestState(t *testing.T) {
	d := db.OpenTest(t)
	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		if err := fellowship.InitFellowship(conn, "test", "/tmp", "main"); err != nil {
			return err
		}
		if err := fellowship.AddQuest(conn, fellowship.QuestEntry{Name: "q1", Worktree: "/tmp/wt1"}); err != nil {
			return err
		}
		if err := fellowship.AddGroup(conn, "team-alpha", []string{"q1"}, []string{"s1"}); err != nil {
			return err
		}
		if err := state.Upsert(conn, &state.State{
			QuestName:   "q1",
			Phase:       "Implement",
			GatePending: true,
		}); err != nil {
			return err
		}

		detail, err := LoadDetail(conn, "team-alpha")
		if err != nil {
			t.Fatalf("LoadDetail() error: %v", err)
		}
		if detail.Name != "team-alpha" {
			t.Errorf("Name = %q, want %q", detail.Name, "team-alpha")
		}
		if len(detail.Quests) != 1 || detail.Quests[0].Name != "q1" {
			t.Fatalf("Quests = %+v, want one entry named q1", detail.Quests)
		}
		if detail.Quests[0].Phase != "Implement" || !detail.Quests[0].GatePending {
			t.Errorf("Quests[0] = %+v, want Phase=Implement GatePending=true", detail.Quests[0])
		}
		if len(detail.Scouts) != 1 || detail.Scouts[0] != "s1" {
			t.Errorf("Scouts = %v, want [s1]", detail.Scouts)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestLoadDetail_GroupNotFound(t *testing.T) {
	d := db.OpenTest(t)
	if err := d.WithConn(context.Background(), func(conn *db.Conn) error {
		if err := fellowship.InitFellowship(conn, "test", "/tmp", "main"); err != nil {
			return err
		}
		if _, err := LoadDetail(conn, "nonexistent"); err == nil {
			t.Fatal("expected error for nonexistent grp")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestApprove_GroupNotFound(t *testing.T) {
	d := db.OpenTest(t)
	if err := d.WithConn(context.Background(), func(conn *db.Conn) error {
		if err := fellowship.InitFellowship(conn, "test", "/tmp", "main"); err != nil {
			return err
		}
		err := Approve(conn, "nonexistent")
		if err == nil {
			t.Fatal("expected error for nonexistent grp")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestApprove_WithPendingGates(t *testing.T) {
	d := db.OpenTest(t)
	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		if err := fellowship.InitFellowship(conn, "test", "/tmp", "main"); err != nil {
			return err
		}
		if err := fellowship.AddQuest(conn, fellowship.QuestEntry{Name: "q1", Worktree: "/tmp/wt1"}); err != nil {
			return err
		}
		if err := fellowship.AddGroup(conn, "team-alpha", []string{"q1"}, nil); err != nil {
			return err
		}

		if err := state.Upsert(conn, &state.State{
			QuestName:   "q1",
			Phase:       "Research",
			GatePending: true,
		}); err != nil {
			return err
		}

		err := Approve(conn, "team-alpha")
		if err != nil {
			t.Fatalf("Approve() error: %v", err)
		}

		// Verify state was advanced
		s, err := state.Load(conn, "q1")
		if err != nil {
			t.Fatal(err)
		}
		if s.Phase != "Plan" {
			t.Errorf("expected phase 'Plan', got %q", s.Phase)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// `group show --json` (LoadDetail) wires in CalculateProgress via
// fellowship.DiscoverQuests, so its Progress field must reflect the same
// completed/in-progress counts CalculateProgress computes directly.
func TestLoadDetail_Progress(t *testing.T) {
	d := db.OpenTest(t)
	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		if err := fellowship.InitFellowship(conn, "test", "/tmp", "main"); err != nil {
			return err
		}
		if err := fellowship.AddQuest(conn, fellowship.QuestEntry{Name: "q1", Worktree: "/tmp/wt1"}); err != nil {
			return err
		}
		if err := fellowship.AddQuest(conn, fellowship.QuestEntry{Name: "q2", Worktree: "/tmp/wt2"}); err != nil {
			return err
		}
		if err := fellowship.AddGroup(conn, "team-alpha", []string{"q1", "q2"}, nil); err != nil {
			return err
		}

		if err := state.Upsert(conn, &state.State{QuestName: "q1", Phase: "Implement"}); err != nil {
			return err
		}
		if err := state.Upsert(conn, &state.State{QuestName: "q2", Phase: "Review"}); err != nil {
			return err
		}
		// Review is terminal — the entry status is what marks q2 finished.
		if err := history.SetStatus(conn, "q2", "completed"); err != nil {
			return err
		}

		detail, err := LoadDetail(conn, "team-alpha")
		if err != nil {
			t.Fatalf("LoadDetail() error: %v", err)
		}

		progress := detail.Progress
		if progress.Name != "team-alpha" {
			t.Errorf("Name = %q, want %q", progress.Name, "team-alpha")
		}
		if progress.Total != 2 {
			t.Errorf("Total = %d, want 2", progress.Total)
		}
		if progress.Completed != 1 {
			t.Errorf("Completed = %d, want 1", progress.Completed)
		}
		if progress.InProgress != 2 { // Implement and Review both rank >= Implement
			t.Errorf("InProgress = %d, want 2", progress.InProgress)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
