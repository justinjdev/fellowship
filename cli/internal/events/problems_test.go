package events

import (
	"context"
	"fmt"
	"testing"
	"time"

	"zombiezen.com/go/sqlite/sqlitex"

	"github.com/justinjdev/fellowship/cli/internal/db"
)

// fixedNow is the clock DetectProblems is told to use in these tests, in
// place of time.Now() offsets — it makes every "N minutes ago" fixture
// deterministic instead of depending on how long the test takes to run.
var fixedNow = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

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
		oldTimestamp := fixedNow.Add(-15 * time.Minute).Unix()
		gateID := fmt.Sprintf("gate-Plan-%d", oldTimestamp)
		insertQuestState(t, conn, "q1", "Plan", true, gateID)

		problems, err := DetectProblems(conn, Options{Now: fixedNow})
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
		recentTimestamp := fixedNow.Add(-2 * time.Minute).Unix()
		gateID := fmt.Sprintf("gate-Plan-%d", recentTimestamp)
		insertQuestState(t, conn, "q1", "Plan", true, gateID)

		problems, err := DetectProblems(conn, Options{Now: fixedNow})
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

		oldTime := fixedNow.Add(-20 * time.Minute).Format(time.RFC3339)
		if err := Record(conn, Event{
			Timestamp: oldTime,
			Quest:     "q1",
			Type:      MetadataUpdated,
		}); err != nil {
			t.Fatal(err)
		}

		problems, err := DetectProblems(conn, Options{Now: fixedNow})
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
				if p.Message != "No activity for 20m" {
					t.Errorf("zombie message = %q, want %q", p.Message, "No activity for 20m")
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
		// Review is terminal, so a finished quest is one whose fellowship
		// entry says completed — not one in a phase past the others.
		insertQuestState(t, conn, "q1", "Review", false, "")
		if err := sqlitex.Execute(conn,
			`INSERT INTO fellowship_quests (name, status) VALUES ('q1', 'completed')`, nil); err != nil {
			t.Fatal(err)
		}

		oldTime := fixedNow.Add(-20 * time.Minute).Format(time.RFC3339)
		if err := Record(conn, Event{
			Timestamp: oldTime,
			Quest:     "q1",
			Type:      MetadataUpdated,
		}); err != nil {
			t.Fatal(err)
		}

		problems, err := DetectProblems(conn, Options{Now: fixedNow})
		if err != nil {
			t.Fatalf("DetectProblems: %v", err)
		}

		for _, p := range problems {
			if p.Type == "zombie" {
				t.Errorf("unexpected zombie problem for a completed quest: %v", p)
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

		now := fixedNow.Format(time.RFC3339)
		if err := Record(conn, Event{Timestamp: now, Quest: "q1", Type: GateRejected, Phase: "Plan"}); err != nil {
			t.Fatal(err)
		}
		if err := Record(conn, Event{Timestamp: now, Quest: "q1", Type: GateRejected, Phase: "Plan"}); err != nil {
			t.Fatal(err)
		}

		problems, err := DetectProblems(conn, Options{Now: fixedNow})
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

		now := fixedNow.Format(time.RFC3339)
		if err := Record(conn, Event{Timestamp: now, Quest: "q1", Type: GateRejected, Phase: "Plan"}); err != nil {
			t.Fatal(err)
		}

		problems, err := DetectProblems(conn, Options{Now: fixedNow})
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

		now := fixedNow.Format(time.RFC3339)
		if err := Record(conn, Event{
			Timestamp: now,
			Quest:     "q1",
			Type:      GateApproved,
			Phase:     "Plan",
		}); err != nil {
			t.Fatal(err)
		}

		problems, err := DetectProblems(conn, Options{Now: fixedNow})
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
