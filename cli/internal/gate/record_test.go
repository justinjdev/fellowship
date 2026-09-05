package gate_test

import (
	"context"
	"testing"

	"github.com/justinjdev/fellowship/cli/internal/db"
	"github.com/justinjdev/fellowship/cli/internal/gate"
	"github.com/justinjdev/fellowship/cli/internal/herald"
	"github.com/justinjdev/fellowship/cli/internal/state"
	"github.com/justinjdev/fellowship/cli/internal/tome"
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
// two heralds. Every approval path shares them, so a lead approval, a company
// batch approval and an auto-approved gate cannot report different histories.
func TestRecordApproval(t *testing.T) {
	d := newQuest(t, "Research")

	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		return gate.RecordApproval(conn, "quest-1", "Implement", "Review", "Batch approved for company c")
	}); err != nil {
		t.Fatalf("RecordApproval: %v", err)
	}

	if err := d.WithConn(context.Background(), func(conn *db.Conn) error {
		qt, err := tome.Load(conn, "quest-1")
		if err != nil {
			return err
		}
		if len(qt.GateHistory) != 1 || qt.GateHistory[0].Action != "approved" || qt.GateHistory[0].Phase != "Implement" {
			t.Errorf("gates = %+v, want one approved Implement gate", qt.GateHistory)
		}
		if len(qt.GateHistory) == 1 && qt.GateHistory[0].Reason != "Batch approved for company c" {
			t.Errorf("gate reason = %q", qt.GateHistory[0].Reason)
		}
		if len(qt.PhasesCompleted) != 1 || qt.PhasesCompleted[0].Phase != "Implement" {
			t.Errorf("phases = %+v, want Implement completed", qt.PhasesCompleted)
		}

		tidings, err := herald.Read(conn, "quest-1", 10)
		if err != nil {
			return err
		}
		if len(tidings) != 2 {
			t.Fatalf("tidings = %d, want 2", len(tidings))
		}
		types := []herald.TidingType{tidings[0].Type, tidings[1].Type}
		wantSeen := map[herald.TidingType]bool{herald.GateApproved: false, herald.PhaseTransition: false}
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

// A rejection keeps the quest in phase, so it records the gate event and the
// herald but no completed phase.
func TestRecordRejection(t *testing.T) {
	d := newQuest(t, "Review")

	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		return gate.RecordRejection(conn, "quest-1", "Review", "")
	}); err != nil {
		t.Fatalf("RecordRejection: %v", err)
	}

	if err := d.WithConn(context.Background(), func(conn *db.Conn) error {
		qt, err := tome.Load(conn, "quest-1")
		if err != nil {
			return err
		}
		if len(qt.GateHistory) != 1 || qt.GateHistory[0].Action != "rejected" {
			t.Errorf("gates = %+v, want one rejected gate", qt.GateHistory)
		}
		if len(qt.PhasesCompleted) != 0 {
			t.Errorf("phases = %+v, want none — a rejection does not complete a phase", qt.PhasesCompleted)
		}
		tidings, err := herald.Read(conn, "quest-1", 10)
		if err != nil {
			return err
		}
		if len(tidings) != 1 || tidings[0].Type != herald.GateRejected {
			t.Errorf("tidings = %+v, want one gate_rejected", tidings)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
