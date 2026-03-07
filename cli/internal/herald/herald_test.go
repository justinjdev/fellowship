package herald

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAnnounceCreatesFileAndAppends(t *testing.T) {
	dir := t.TempDir()

	tid1 := Tiding{
		Timestamp: "2025-01-15T10:00:00Z",
		Quest:     "quest-login",
		Type:      GateSubmitted,
		Phase:     "Plan",
		Detail:    "Gate submitted for review",
	}
	if err := Announce(dir, tid1); err != nil {
		t.Fatalf("Announce first tiding: %v", err)
	}

	// Verify file was created
	path := filepath.Join(dir, "tmp", heraldFile)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("herald file not created: %v", err)
	}

	tid2 := Tiding{
		Timestamp: "2025-01-15T10:05:00Z",
		Quest:     "quest-login",
		Type:      GateApproved,
		Phase:     "Plan",
		Detail:    "Gate approved",
	}
	if err := Announce(dir, tid2); err != nil {
		t.Fatalf("Announce second tiding: %v", err)
	}

	// Read back and verify
	tidings, err := Read(dir, 0)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(tidings) != 2 {
		t.Fatalf("got %d tidings, want 2", len(tidings))
	}
	if tidings[0].Type != GateSubmitted {
		t.Errorf("tidings[0].Type = %q, want %q", tidings[0].Type, GateSubmitted)
	}
	if tidings[1].Type != GateApproved {
		t.Errorf("tidings[1].Type = %q, want %q", tidings[1].Type, GateApproved)
	}
}

func TestReadReturnsLatestN(t *testing.T) {
	dir := t.TempDir()

	for i := 0; i < 10; i++ {
		tid := Tiding{
			Timestamp: "2025-01-15T10:00:00Z",
			Quest:     "quest-login",
			Type:      MetadataUpdated,
			Detail:    "tiding",
		}
		if err := Announce(dir, tid); err != nil {
			t.Fatalf("Announce: %v", err)
		}
	}

	tidings, err := Read(dir, 3)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(tidings) != 3 {
		t.Fatalf("got %d tidings, want 3", len(tidings))
	}
}

func TestReadNoFile(t *testing.T) {
	dir := t.TempDir()

	tidings, err := Read(dir, 10)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(tidings) != 0 {
		t.Fatalf("got %d tidings, want 0", len(tidings))
	}
}

func TestReadAllAggregatesAcrossWorktrees(t *testing.T) {
	wt1 := t.TempDir()
	wt2 := t.TempDir()

	Announce(wt1, Tiding{
		Timestamp: "2025-01-15T10:00:00Z",
		Quest:     "quest-a",
		Type:      GateSubmitted,
	})
	Announce(wt1, Tiding{
		Timestamp: "2025-01-15T10:10:00Z",
		Quest:     "quest-a",
		Type:      GateApproved,
	})
	Announce(wt2, Tiding{
		Timestamp: "2025-01-15T10:05:00Z",
		Quest:     "quest-b",
		Type:      PhaseTransition,
	})

	tidings, err := ReadAll([]string{wt1, wt2}, 10)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(tidings) != 3 {
		t.Fatalf("got %d tidings, want 3", len(tidings))
	}

	// Should be sorted descending by timestamp
	if tidings[0].Timestamp != "2025-01-15T10:10:00Z" {
		t.Errorf("tidings[0].Timestamp = %q, want 2025-01-15T10:10:00Z", tidings[0].Timestamp)
	}
	if tidings[1].Timestamp != "2025-01-15T10:05:00Z" {
		t.Errorf("tidings[1].Timestamp = %q, want 2025-01-15T10:05:00Z", tidings[1].Timestamp)
	}
	if tidings[2].Timestamp != "2025-01-15T10:00:00Z" {
		t.Errorf("tidings[2].Timestamp = %q, want 2025-01-15T10:00:00Z", tidings[2].Timestamp)
	}
}

func TestReadAllWithLimit(t *testing.T) {
	wt1 := t.TempDir()
	wt2 := t.TempDir()

	for i := 0; i < 5; i++ {
		Announce(wt1, Tiding{Timestamp: "2025-01-15T10:00:00Z", Quest: "a", Type: GateSubmitted})
		Announce(wt2, Tiding{Timestamp: "2025-01-15T10:01:00Z", Quest: "b", Type: GateSubmitted})
	}

	tidings, err := ReadAll([]string{wt1, wt2}, 3)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(tidings) != 3 {
		t.Fatalf("got %d tidings, want 3", len(tidings))
	}
}
