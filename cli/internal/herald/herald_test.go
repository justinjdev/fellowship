package herald

import (
	"context"
	"fmt"
	"testing"

	"zombiezen.com/go/sqlite/sqlitex"

	"github.com/justinjdev/fellowship/cli/internal/db"
)

func TestAnnounceAndRead(t *testing.T) {
	d := db.OpenTest(t)
	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		if err := Announce(conn, Tiding{
			Timestamp: "2026-01-01T00:00:00Z",
			Quest:     "q1",
			Type:      GateSubmitted,
			Phase:     "Research",
		}); err != nil {
			t.Fatal(err)
		}
		if err := Announce(conn, Tiding{
			Timestamp: "2026-01-01T00:01:00Z",
			Quest:     "q1",
			Type:      GateApproved,
			Phase:     "Research",
		}); err != nil {
			t.Fatal(err)
		}

		tidings, err := Read(conn, "q1", 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(tidings) != 2 {
			t.Fatalf("expected 2, got %d", len(tidings))
		}
		if tidings[0].Type != GateSubmitted {
			t.Errorf("tidings[0].Type = %q, want %q", tidings[0].Type, GateSubmitted)
		}
		if tidings[1].Type != GateApproved {
			t.Errorf("tidings[1].Type = %q, want %q", tidings[1].Type, GateApproved)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestReadReturnsLatestN(t *testing.T) {
	d := db.OpenTest(t)
	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		for i := 0; i < 10; i++ {
			if err := Announce(conn, Tiding{
				Timestamp: fmt.Sprintf("2026-01-01T00:%02d:00Z", i),
				Quest:     "q1",
				Type:      MetadataUpdated,
				Detail:    fmt.Sprintf("tiding-%d", i),
			}); err != nil {
				t.Fatal(err)
			}
		}

		tidings, err := Read(conn, "q1", 3)
		if err != nil {
			t.Fatal(err)
		}
		if len(tidings) != 3 {
			t.Fatalf("got %d tidings, want 3", len(tidings))
		}
		// Should be last 3 in ascending order
		if tidings[0].Detail != "tiding-7" {
			t.Errorf("tidings[0].Detail = %q, want tiding-7", tidings[0].Detail)
		}
		if tidings[2].Detail != "tiding-9" {
			t.Errorf("tidings[2].Detail = %q, want tiding-9", tidings[2].Detail)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestReadNoData(t *testing.T) {
	d := db.OpenTest(t)
	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		tidings, err := Read(conn, "nonexistent", 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(tidings) != 0 {
			t.Fatalf("got %d tidings, want 0", len(tidings))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestReadAll_Limit(t *testing.T) {
	d := db.OpenTest(t)
	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		for i := 0; i < 5; i++ {
			if err := Announce(conn, Tiding{
				Timestamp: fmt.Sprintf("2026-01-01T00:%02d:00Z", i),
				Quest:     "q1",
				Type:      PhaseTransition,
			}); err != nil {
				t.Fatal(err)
			}
		}
		tidings, err := ReadAll(conn, 3)
		if err != nil {
			t.Fatal(err)
		}
		if len(tidings) != 3 {
			t.Fatalf("expected 3, got %d", len(tidings))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestReadAllAcrossQuests(t *testing.T) {
	d := db.OpenTest(t)
	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		if err := Announce(conn, Tiding{
			Timestamp: "2026-01-01T00:00:00Z",
			Quest:     "q1",
			Type:      GateSubmitted,
		}); err != nil {
			t.Fatal(err)
		}
		if err := Announce(conn, Tiding{
			Timestamp: "2026-01-01T00:05:00Z",
			Quest:     "q2",
			Type:      PhaseTransition,
		}); err != nil {
			t.Fatal(err)
		}
		if err := Announce(conn, Tiding{
			Timestamp: "2026-01-01T00:10:00Z",
			Quest:     "q1",
			Type:      GateApproved,
		}); err != nil {
			t.Fatal(err)
		}

		tidings, err := ReadAll(conn, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(tidings) != 3 {
			t.Fatalf("got %d tidings, want 3", len(tidings))
		}
		// Ascending order by id (insertion order)
		if tidings[0].Quest != "q1" || tidings[0].Type != GateSubmitted {
			t.Errorf("tidings[0] = %+v, want q1/gate_submitted", tidings[0])
		}
		if tidings[1].Quest != "q2" {
			t.Errorf("tidings[1].Quest = %q, want q2", tidings[1].Quest)
		}
		if tidings[2].Quest != "q1" || tidings[2].Type != GateApproved {
			t.Errorf("tidings[2] = %+v, want q1/gate_approved", tidings[2])
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestDetectProblems_Struggling(t *testing.T) {
	d := db.OpenTest(t)
	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		// Create a quest in Research phase
		if err := sqlitex.Execute(conn,
			`INSERT INTO quest_state (quest_name, phase, gate_pending, created_at, updated_at)
			 VALUES ('q1', 'Research', 0, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`, nil); err != nil {
			t.Fatal(err)
		}

		// Add 2 rejections in Research phase
		if err := Announce(conn, Tiding{Timestamp: "2026-01-01T00:01:00Z", Quest: "q1", Type: GateRejected, Phase: "Research"}); err != nil {
			t.Fatal(err)
		}
		if err := Announce(conn, Tiding{Timestamp: "2026-01-01T00:02:00Z", Quest: "q1", Type: GateRejected, Phase: "Research"}); err != nil {
			t.Fatal(err)
		}

		problems, err := DetectProblems(conn)
		if err != nil {
			t.Fatalf("DetectProblems: %v", err)
		}
		found := false
		for _, p := range problems {
			if p.Type == "struggling" && p.Quest == "q1" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected struggling problem for q1, got %+v", problems)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestDetectProblems_NoProblems(t *testing.T) {
	d := db.OpenTest(t)
	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		// Quest in Complete phase should not be checked
		if err := sqlitex.Execute(conn,
			`INSERT INTO quest_state (quest_name, phase, gate_pending, created_at, updated_at)
			 VALUES ('q1', 'Complete', 0, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`, nil); err != nil {
			t.Fatal(err)
		}

		problems, err := DetectProblems(conn)
		if err != nil {
			t.Fatalf("DetectProblems: %v", err)
		}
		if len(problems) != 0 {
			t.Errorf("expected 0 problems, got %+v", problems)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
