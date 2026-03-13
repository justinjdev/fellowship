package herald

import (
	"context"
	"fmt"
	"testing"
	"time"

	"zombiezen.com/go/sqlite/sqlitex"

	"github.com/justinjdev/fellowship/cli/internal/db"
)

func insertQuestState(t *testing.T, conn *db.Conn, questName, phase string, gatePending bool, gateID string) {
	t.Helper()
	gp := 0
	if gatePending {
		gp = 1
	}
	var gateIDArg any
	if gateID != "" {
		gateIDArg = gateID
	}
	if err := sqlitex.Execute(conn,
		`INSERT INTO quest_state (quest_name, phase, gate_pending, gate_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`,
		&sqlitex.ExecOptions{
			Args: []any{questName, phase, gp, gateIDArg},
		},
	); err != nil {
		t.Fatal(err)
	}
}

func TestStalledDetection(t *testing.T) {
	d := db.OpenTest(t)
	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		oldTimestamp := time.Now().Add(-15 * time.Minute).Unix()
		gateID := fmt.Sprintf("gate-Plan-%d", oldTimestamp)
		insertQuestState(t, conn, "q1", "Plan", true, gateID)

		problems, err := DetectProblems(conn)
		if err != nil {
			t.Fatalf("DetectProblems: %v", err)
		}

		var found bool
		for _, p := range problems {
			if p.Type == "stalled" {
				found = true
				if p.Severity != Warning {
					t.Errorf("stalled severity = %q, want %q", p.Severity, Warning)
				}
			}
		}
		if !found {
			t.Errorf("expected stalled problem, got %v", problems)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestStalledNotDetectedWhenRecent(t *testing.T) {
	d := db.OpenTest(t)
	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		recentTimestamp := time.Now().Add(-2 * time.Minute).Unix()
		gateID := fmt.Sprintf("gate-Plan-%d", recentTimestamp)
		insertQuestState(t, conn, "q1", "Plan", true, gateID)

		problems, err := DetectProblems(conn)
		if err != nil {
			t.Fatalf("DetectProblems: %v", err)
		}

		for _, p := range problems {
			if p.Type == "stalled" {
				t.Errorf("unexpected stalled problem: %v", p)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestZombieDetection(t *testing.T) {
	d := db.OpenTest(t)
	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		insertQuestState(t, conn, "q1", "Implement", false, "")

		oldTime := time.Now().Add(-20 * time.Minute).UTC().Format(time.RFC3339)
		if err := Announce(conn, Tiding{
			Timestamp: oldTime,
			Quest:     "q1",
			Type:      MetadataUpdated,
		}); err != nil {
			t.Fatal(err)
		}

		problems, err := DetectProblems(conn)
		if err != nil {
			t.Fatalf("DetectProblems: %v", err)
		}

		var found bool
		for _, p := range problems {
			if p.Type == "zombie" {
				found = true
				if p.Severity != Critical {
					t.Errorf("zombie severity = %q, want %q", p.Severity, Critical)
				}
			}
		}
		if !found {
			t.Errorf("expected zombie problem, got %v", problems)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestZombieNotDetectedWhenComplete(t *testing.T) {
	d := db.OpenTest(t)
	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		insertQuestState(t, conn, "q1", "Complete", false, "")

		oldTime := time.Now().Add(-20 * time.Minute).UTC().Format(time.RFC3339)
		if err := Announce(conn, Tiding{
			Timestamp: oldTime,
			Quest:     "q1",
			Type:      MetadataUpdated,
		}); err != nil {
			t.Fatal(err)
		}

		problems, err := DetectProblems(conn)
		if err != nil {
			t.Fatalf("DetectProblems: %v", err)
		}

		for _, p := range problems {
			if p.Type == "zombie" {
				t.Errorf("unexpected zombie problem for Complete quest: %v", p)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestStrugglingDetection(t *testing.T) {
	d := db.OpenTest(t)
	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		insertQuestState(t, conn, "q1", "Plan", false, "")

		now := time.Now().UTC().Format(time.RFC3339)
		if err := Announce(conn, Tiding{Timestamp: now, Quest: "q1", Type: GateRejected, Phase: "Plan"}); err != nil {
			t.Fatal(err)
		}
		if err := Announce(conn, Tiding{Timestamp: now, Quest: "q1", Type: GateRejected, Phase: "Plan"}); err != nil {
			t.Fatal(err)
		}

		problems, err := DetectProblems(conn)
		if err != nil {
			t.Fatalf("DetectProblems: %v", err)
		}

		var found bool
		for _, p := range problems {
			if p.Type == "struggling" {
				found = true
				if p.Severity != Warning {
					t.Errorf("struggling severity = %q, want %q", p.Severity, Warning)
				}
			}
		}
		if !found {
			t.Errorf("expected struggling problem, got %v", problems)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestStrugglingNotDetectedWithOneRejection(t *testing.T) {
	d := db.OpenTest(t)
	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		insertQuestState(t, conn, "q1", "Plan", false, "")

		now := time.Now().UTC().Format(time.RFC3339)
		if err := Announce(conn, Tiding{Timestamp: now, Quest: "q1", Type: GateRejected, Phase: "Plan"}); err != nil {
			t.Fatal(err)
		}

		problems, err := DetectProblems(conn)
		if err != nil {
			t.Fatalf("DetectProblems: %v", err)
		}

		for _, p := range problems {
			if p.Type == "struggling" {
				t.Errorf("unexpected struggling problem with only 1 rejection: %v", p)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestNoProblemsForHealthyQuest(t *testing.T) {
	d := db.OpenTest(t)
	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		insertQuestState(t, conn, "q1", "Implement", false, "")

		now := time.Now().UTC().Format(time.RFC3339)
		if err := Announce(conn, Tiding{
			Timestamp: now,
			Quest:     "q1",
			Type:      GateApproved,
			Phase:     "Plan",
		}); err != nil {
			t.Fatal(err)
		}

		problems, err := DetectProblems(conn)
		if err != nil {
			t.Fatalf("DetectProblems: %v", err)
		}

		if len(problems) != 0 {
			t.Errorf("expected no problems, got %v", problems)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
