package gate_test

import (
	"context"
	"testing"
	"time"

	"zombiezen.com/go/sqlite/sqlitex"

	"github.com/justinjdev/fellowship/cli/internal/db"
	"github.com/justinjdev/fellowship/cli/internal/events"
	"github.com/justinjdev/fellowship/cli/internal/gate"
	"github.com/justinjdev/fellowship/cli/internal/history"
	"github.com/justinjdev/fellowship/cli/internal/state"
)

// newQuest opens a store holding one quest in the given phase.
func newQuest(t *testing.T, phase string) *db.DB {
	t.Helper()
	d := db.OpenTest(t)
	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		return state.Upsert(conn, &state.State{QuestName: "quest-1", Phase: phase})
	}); err != nil {
		t.Fatal(err)
	}
	return d
}

// An approval is three records: the gate event, the completed phase, and the
// two events. Every approval path shares them, so a lead approval, a group
// batch approval and an auto-approved gate cannot report different histories.
func TestRecordApproval(t *testing.T) {
	d := newQuest(t, "Research")

	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		return gate.RecordApproval(conn, "quest-1", "Implement", "Review", "Batch approved for group c")
	}); err != nil {
		t.Fatalf("RecordApproval: %v", err)
	}

	if err := d.WithConn(context.Background(), func(conn *db.Conn) error {
		qt, err := history.Load(conn, "quest-1")
		if err != nil {
			return err
		}
		if len(qt.GateHistory) != 1 || qt.GateHistory[0].Action != "approved" || qt.GateHistory[0].Phase != "Implement" {
			t.Errorf("gates = %+v, want one approved Implement gate", qt.GateHistory)
		}
		if len(qt.GateHistory) == 1 && qt.GateHistory[0].Reason != "Batch approved for group c" {
			t.Errorf("gate reason = %q", qt.GateHistory[0].Reason)
		}
		if len(qt.PhasesCompleted) != 1 || qt.PhasesCompleted[0].Phase != "Implement" {
			t.Errorf("phases = %+v, want Implement completed", qt.PhasesCompleted)
		}

		tidings, err := events.Read(conn, "quest-1", 10)
		if err != nil {
			return err
		}
		if len(tidings) != 2 {
			t.Fatalf("tidings = %d, want 2", len(tidings))
		}
		types := []events.EventType{tidings[0].Type, tidings[1].Type}
		wantSeen := map[events.EventType]bool{events.GateApproved: false, events.PhaseTransition: false}
		for _, ty := range types {
			if _, ok := wantSeen[ty]; !ok {
				t.Errorf("unexpected tiding type %q", ty)
				continue
			}
			wantSeen[ty] = true
		}
		for ty, seen := range wantSeen {
			if !seen {
				t.Errorf("missing tiding %q", ty)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// RecordApproval must record how long the quest actually spent in the phase
// it left — measured from the quest's created_at, for its first phase
// completion — rather than a hardcoded 0.
func TestRecordApproval_DurationFromCreatedAt(t *testing.T) {
	d := newQuest(t, "Research")

	createdAt := time.Now().UTC().Add(-90 * time.Second).Format(time.RFC3339)
	if err := d.WithConn(context.Background(), func(conn *db.Conn) error {
		return sqlitex.Execute(conn,
			`UPDATE quest_state SET created_at = :created_at WHERE quest_name = 'quest-1'`,
			&sqlitex.ExecOptions{Named: map[string]any{":created_at": createdAt}})
	}); err != nil {
		t.Fatal(err)
	}

	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		return gate.RecordApproval(conn, "quest-1", "Research", "Plan", "")
	}); err != nil {
		t.Fatalf("RecordApproval: %v", err)
	}

	if err := d.WithConn(context.Background(), func(conn *db.Conn) error {
		phases, err := history.LoadPhases(conn, "quest-1")
		if err != nil {
			return err
		}
		if len(phases) != 1 {
			t.Fatalf("phases = %+v, want 1", phases)
		}
		// Allow slack for the time this test itself takes to run.
		if phases[0].DurationS < 85 || phases[0].DurationS > 120 {
			t.Errorf("DurationS = %d, want ~90", phases[0].DurationS)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// A quest's second phase completion measures from the first phase's
// completed_at, not from created_at again.
func TestRecordApproval_DurationFromPreviousPhase(t *testing.T) {
	d := newQuest(t, "Research")

	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		return gate.RecordApproval(conn, "quest-1", "Research", "Plan", "")
	}); err != nil {
		t.Fatalf("RecordApproval (Research): %v", err)
	}

	// Push the recorded Research completion into the past so the second
	// approval's duration is measurably nonzero and driven by it, not by
	// created_at.
	if err := d.WithConn(context.Background(), func(conn *db.Conn) error {
		return sqlitex.Execute(conn,
			`UPDATE quest_phases SET completed_at = :t WHERE quest_name = 'quest-1' AND phase = 'Research'`,
			&sqlitex.ExecOptions{Named: map[string]any{
				":t": time.Now().UTC().Add(-60 * time.Second).Format(time.RFC3339),
			}})
	}); err != nil {
		t.Fatal(err)
	}

	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		return gate.RecordApproval(conn, "quest-1", "Plan", "Implement", "")
	}); err != nil {
		t.Fatalf("RecordApproval (Plan): %v", err)
	}

	if err := d.WithConn(context.Background(), func(conn *db.Conn) error {
		phases, err := history.LoadPhases(conn, "quest-1")
		if err != nil {
			return err
		}
		if len(phases) != 2 {
			t.Fatalf("phases = %+v, want 2", phases)
		}
		if phases[1].Phase != "Plan" {
			t.Fatalf("phases[1] = %+v, want Plan", phases[1])
		}
		if phases[1].DurationS < 55 || phases[1].DurationS > 90 {
			t.Errorf("Plan DurationS = %d, want ~60", phases[1].DurationS)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// A rejection keeps the quest in phase, so it records the gate event and the
// event but no completed phase.
func TestRecordRejection(t *testing.T) {
	d := newQuest(t, "Review")

	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		return gate.RecordRejection(conn, "quest-1", "Review", "")
	}); err != nil {
		t.Fatalf("RecordRejection: %v", err)
	}

	if err := d.WithConn(context.Background(), func(conn *db.Conn) error {
		qt, err := history.Load(conn, "quest-1")
		if err != nil {
			return err
		}
		if len(qt.GateHistory) != 1 || qt.GateHistory[0].Action != "rejected" {
			t.Errorf("gates = %+v, want one rejected gate", qt.GateHistory)
		}
		if len(qt.PhasesCompleted) != 0 {
			t.Errorf("phases = %+v, want none — a rejection does not complete a phase", qt.PhasesCompleted)
		}
		tidings, err := events.Read(conn, "quest-1", 10)
		if err != nil {
			return err
		}
		if len(tidings) != 1 || tidings[0].Type != events.GateRejected {
			t.Errorf("tidings = %+v, want one gate_rejected", tidings)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
