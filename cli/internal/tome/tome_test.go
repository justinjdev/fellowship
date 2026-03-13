package tome_test

import (
	"context"
	"testing"

	"github.com/justinjdev/fellowship/cli/internal/db"
	"github.com/justinjdev/fellowship/cli/internal/state"
	"github.com/justinjdev/fellowship/cli/internal/tome"
	"zombiezen.com/go/sqlite/sqlitex"
)

func seedQuest(t *testing.T, d *db.DB, name string) {
	t.Helper()
	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		return state.Upsert(conn, &state.State{QuestName: name, Phase: "Research"})
	}); err != nil {
		t.Fatal(err)
	}
}

func TestRecordPhase(t *testing.T) {
	d := db.OpenTest(t)
	seedQuest(t, d, "q1")

	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		if err := tome.RecordPhase(conn, "q1", "Research", 120); err != nil {
			t.Fatal(err)
		}
		phases, err := tome.LoadPhases(conn, "q1")
		if err != nil {
			t.Fatal(err)
		}
		if len(phases) != 1 || phases[0].Phase != "Research" {
			t.Errorf("unexpected phases: %+v", phases)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestRecordGate(t *testing.T) {
	d := db.OpenTest(t)
	seedQuest(t, d, "q1")

	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		if err := tome.RecordGate(conn, "q1", "Research", "submitted", ""); err != nil {
			t.Fatal(err)
		}
		if err := tome.RecordGate(conn, "q1", "Research", "approved", ""); err != nil {
			t.Fatal(err)
		}

		gates, err := tome.LoadGates(conn, "q1")
		if err != nil {
			t.Fatal(err)
		}
		if len(gates) != 2 {
			t.Fatalf("expected 2 gates, got %d", len(gates))
		}
		if gates[0].Action != "submitted" {
			t.Errorf("expected submitted, got %s", gates[0].Action)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestRecordFiles(t *testing.T) {
	d := db.OpenTest(t)
	seedQuest(t, d, "q1")

	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		if err := tome.RecordFiles(conn, "q1", []string{"src/main.go", "src/util.go"}); err != nil {
			t.Fatal(err)
		}
		if err := tome.RecordFiles(conn, "q1", []string{"src/main.go", "src/new.go"}); err != nil {
			t.Fatal(err)
		}

		files, err := tome.LoadFiles(conn, "q1")
		if err != nil {
			t.Fatal(err)
		}
		if len(files) != 3 {
			t.Fatalf("expected 3 unique files, got %d: %v", len(files), files)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestLoad(t *testing.T) {
	d := db.OpenTest(t)
	seedQuest(t, d, "q1")

	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		if err := tome.RecordPhase(conn, "q1", "Onboard", 60); err != nil {
			t.Fatal(err)
		}
		if err := tome.RecordGate(conn, "q1", "Onboard", "approved", ""); err != nil {
			t.Fatal(err)
		}
		if err := tome.RecordFiles(conn, "q1", []string{"a.go"}); err != nil {
			t.Fatal(err)
		}

		qt, err := tome.Load(conn, "q1")
		if err != nil {
			t.Fatal(err)
		}
		if len(qt.PhasesCompleted) != 1 {
			t.Errorf("expected 1 phase, got %d", len(qt.PhasesCompleted))
		}
		if len(qt.GateHistory) != 1 {
			t.Errorf("expected 1 gate, got %d", len(qt.GateHistory))
		}
		if len(qt.FilesTouched) != 1 {
			t.Errorf("expected 1 file, got %d", len(qt.FilesTouched))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestLoad_NoData(t *testing.T) {
	d := db.OpenTest(t)
	seedQuest(t, d, "q1")

	if err := d.WithConn(context.Background(), func(conn *db.Conn) error {
		qt, err := tome.Load(conn, "q1")
		if err != nil {
			t.Fatal(err)
		}
		if qt.QuestName != "q1" {
			t.Errorf("expected q1, got %s", qt.QuestName)
		}
		if qt.Status != "active" {
			t.Errorf("expected active status, got %s", qt.Status)
		}
		if len(qt.PhasesCompleted) != 0 {
			t.Errorf("expected 0 phases, got %d", len(qt.PhasesCompleted))
		}
		if len(qt.GateHistory) != 0 {
			t.Errorf("expected 0 gates, got %d", len(qt.GateHistory))
		}
		if len(qt.FilesTouched) != 0 {
			t.Errorf("expected 0 files, got %d", len(qt.FilesTouched))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestRecordSkippedPhases(t *testing.T) {
	d := db.OpenTest(t)
	seedQuest(t, d, "q1")

	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		if err := tome.RecordSkippedPhases(conn, "q1", []string{"Onboard", "Research", "Plan"}, "pre-existing plan"); err != nil {
			t.Fatal(err)
		}

		phases, err := tome.LoadPhases(conn, "q1")
		if err != nil {
			t.Fatal(err)
		}
		if len(phases) != 3 {
			t.Fatalf("expected 3 phases, got %d", len(phases))
		}

		gates, err := tome.LoadGates(conn, "q1")
		if err != nil {
			t.Fatal(err)
		}
		if len(gates) != 3 {
			t.Fatalf("expected 3 gates, got %d", len(gates))
		}

		for i, phase := range []string{"Onboard", "Research", "Plan"} {
			if gates[i].Phase != phase {
				t.Errorf("gates[%d].Phase = %q, want %q", i, gates[i].Phase, phase)
			}
			if gates[i].Action != "skipped" {
				t.Errorf("gates[%d].Action = %q, want skipped", i, gates[i].Action)
			}
			if gates[i].Reason != "pre-existing plan" {
				t.Errorf("gates[%d].Reason = %q, want 'pre-existing plan'", i, gates[i].Reason)
			}
			if phases[i].Phase != phase {
				t.Errorf("phases[%d].Phase = %q, want %q", i, phases[i].Phase, phase)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestSetStatus(t *testing.T) {
	d := db.OpenTest(t)
	seedQuest(t, d, "q1")

	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		// Insert a fellowship_quests row for SetStatus to update.
		if err := tome.SetStatus(conn, "q1", "completed"); err != nil {
			t.Fatal(err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	// Insert fellowship_quests row and test SetStatus.
	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		// Manually insert a fellowship_quests row.
		if err := sqlitex.Execute(conn, `INSERT INTO fellowship_quests (name, status) VALUES ('q1', 'active')`, nil); err != nil {
			t.Fatal(err)
		}
		if err := tome.SetStatus(conn, "q1", "completed"); err != nil {
			t.Fatal(err)
		}

		qt, err := tome.Load(conn, "q1")
		if err != nil {
			t.Fatal(err)
		}
		if qt.Status != "completed" {
			t.Errorf("expected completed, got %s", qt.Status)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
