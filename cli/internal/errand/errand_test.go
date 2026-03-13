package errand_test

import (
	"context"
	"testing"

	"github.com/justinjdev/fellowship/cli/internal/db"
	"github.com/justinjdev/fellowship/cli/internal/errand"
	"github.com/justinjdev/fellowship/cli/internal/state"
)

func seedQuest(t *testing.T, d *db.DB, name string) {
	t.Helper()
	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		return state.Upsert(conn, &state.State{QuestName: name, Phase: "Implement"})
	}); err != nil {
		t.Fatal(err)
	}
}

func TestAddAndList(t *testing.T) {
	d := db.OpenTest(t)
	seedQuest(t, d, "q1")

	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		id, err := errand.Add(conn, "q1", "Build auth module", "Implement")
		if err != nil {
			t.Fatal(err)
		}
		if id != "w-001" {
			t.Errorf("expected w-001, got %s", id)
		}

		items, err := errand.List(conn, "q1")
		if err != nil {
			t.Fatal(err)
		}
		if len(items) != 1 {
			t.Fatalf("expected 1, got %d", len(items))
		}
		if items[0].Description != "Build auth module" {
			t.Error("description mismatch")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestAddSequentialIDs(t *testing.T) {
	d := db.OpenTest(t)
	seedQuest(t, d, "q1")

	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		id1, err := errand.Add(conn, "q1", "first", "Implement")
		if err != nil {
			t.Fatal(err)
		}
		id2, err := errand.Add(conn, "q1", "second", "Implement")
		if err != nil {
			t.Fatal(err)
		}
		id3, err := errand.Add(conn, "q1", "third", "Review")
		if err != nil {
			t.Fatal(err)
		}

		if id1 != "w-001" {
			t.Errorf("first ID = %q, want w-001", id1)
		}
		if id2 != "w-002" {
			t.Errorf("second ID = %q, want w-002", id2)
		}
		if id3 != "w-003" {
			t.Errorf("third ID = %q, want w-003", id3)
		}

		items, err := errand.List(conn, "q1")
		if err != nil {
			t.Fatal(err)
		}
		if len(items) != 3 {
			t.Errorf("Items count = %d, want 3", len(items))
		}
		if items[0].Phase != "Implement" {
			t.Errorf("Item 0 Phase = %q, want Implement", items[0].Phase)
		}
		if items[2].Phase != "Review" {
			t.Errorf("Item 2 Phase = %q, want Review", items[2].Phase)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateStatus(t *testing.T) {
	d := db.OpenTest(t)
	seedQuest(t, d, "q1")

	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		if _, err := errand.Add(conn, "q1", "Task 1", ""); err != nil {
			t.Fatal(err)
		}
		if err := errand.UpdateStatus(conn, "q1", "w-001", errand.Done); err != nil {
			t.Fatal(err)
		}

		items, err := errand.List(conn, "q1")
		if err != nil {
			t.Fatal(err)
		}
		if items[0].Status != errand.Done {
			t.Errorf("expected done, got %s", items[0].Status)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateStatusNotFound(t *testing.T) {
	d := db.OpenTest(t)
	seedQuest(t, d, "q1")

	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		err := errand.UpdateStatus(conn, "q1", "w-999", errand.Done)
		if err == nil {
			t.Fatal("expected error for nonexistent errand")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestProgress(t *testing.T) {
	d := db.OpenTest(t)
	seedQuest(t, d, "q1")

	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		if _, err := errand.Add(conn, "q1", "A", ""); err != nil {
			t.Fatal(err)
		}
		if _, err := errand.Add(conn, "q1", "B", ""); err != nil {
			t.Fatal(err)
		}
		if err := errand.UpdateStatus(conn, "q1", "w-001", errand.Done); err != nil {
			t.Fatal(err)
		}

		done, total, err := errand.Progress(conn, "q1")
		if err != nil {
			t.Fatal(err)
		}
		if done != 1 || total != 2 {
			t.Errorf("expected 1/2, got %d/%d", done, total)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestPendingErrands(t *testing.T) {
	d := db.OpenTest(t)
	seedQuest(t, d, "q1")

	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		if _, err := errand.Add(conn, "q1", "A", ""); err != nil {
			t.Fatal(err)
		}
		if _, err := errand.Add(conn, "q1", "B", ""); err != nil {
			t.Fatal(err)
		}
		if err := errand.UpdateStatus(conn, "q1", "w-001", errand.Done); err != nil {
			t.Fatal(err)
		}

		pending, err := errand.PendingErrands(conn, "q1")
		if err != nil {
			t.Fatal(err)
		}
		if len(pending) != 1 {
			t.Fatalf("expected 1 pending, got %d", len(pending))
		}
		if pending[0].ID != "w-002" {
			t.Errorf("expected w-002, got %s", pending[0].ID)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestValidStatus(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"pending", true},
		{"in_progress", true},
		{"done", true},
		{"blocked", true},
		{"skipped", true},
		{"invalid", false},
		{"", false},
	}
	for _, tt := range tests {
		_, ok := errand.ValidStatus(tt.input)
		if ok != tt.valid {
			t.Errorf("ValidStatus(%q) = %v, want %v", tt.input, ok, tt.valid)
		}
	}
}
