package events

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAppendCreatesFileAndAppends(t *testing.T) {
	dir := t.TempDir()

	evt1 := Event{
		Timestamp: "2025-01-15T10:00:00Z",
		Quest:     "quest-login",
		Type:      GateSubmitted,
		Phase:     "Plan",
		Detail:    "Gate submitted for review",
	}
	if err := Append(dir, evt1); err != nil {
		t.Fatalf("Append first event: %v", err)
	}

	// Verify file was created
	path := filepath.Join(dir, "tmp", eventsFile)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("events file not created: %v", err)
	}

	evt2 := Event{
		Timestamp: "2025-01-15T10:05:00Z",
		Quest:     "quest-login",
		Type:      GateApproved,
		Phase:     "Plan",
		Detail:    "Gate approved",
	}
	if err := Append(dir, evt2); err != nil {
		t.Fatalf("Append second event: %v", err)
	}

	// Read back and verify
	evts, err := Read(dir, 0)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(evts) != 2 {
		t.Fatalf("got %d events, want 2", len(evts))
	}
	if evts[0].Type != GateSubmitted {
		t.Errorf("evts[0].Type = %q, want %q", evts[0].Type, GateSubmitted)
	}
	if evts[1].Type != GateApproved {
		t.Errorf("evts[1].Type = %q, want %q", evts[1].Type, GateApproved)
	}
}

func TestReadReturnsLatestN(t *testing.T) {
	dir := t.TempDir()

	for i := 0; i < 10; i++ {
		evt := Event{
			Timestamp: "2025-01-15T10:00:00Z",
			Quest:     "quest-login",
			Type:      MetadataUpdated,
			Detail:    "event",
		}
		if err := Append(dir, evt); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}

	evts, err := Read(dir, 3)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(evts) != 3 {
		t.Fatalf("got %d events, want 3", len(evts))
	}
}

func TestReadNoFile(t *testing.T) {
	dir := t.TempDir()

	evts, err := Read(dir, 10)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(evts) != 0 {
		t.Fatalf("got %d events, want 0", len(evts))
	}
}

func TestReadAllAggregatesAcrossWorktrees(t *testing.T) {
	wt1 := t.TempDir()
	wt2 := t.TempDir()

	Append(wt1, Event{
		Timestamp: "2025-01-15T10:00:00Z",
		Quest:     "quest-a",
		Type:      GateSubmitted,
	})
	Append(wt1, Event{
		Timestamp: "2025-01-15T10:10:00Z",
		Quest:     "quest-a",
		Type:      GateApproved,
	})
	Append(wt2, Event{
		Timestamp: "2025-01-15T10:05:00Z",
		Quest:     "quest-b",
		Type:      PhaseTransition,
	})

	evts, err := ReadAll([]string{wt1, wt2}, 10)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(evts) != 3 {
		t.Fatalf("got %d events, want 3", len(evts))
	}

	// Should be sorted descending by timestamp
	if evts[0].Timestamp != "2025-01-15T10:10:00Z" {
		t.Errorf("evts[0].Timestamp = %q, want 2025-01-15T10:10:00Z", evts[0].Timestamp)
	}
	if evts[1].Timestamp != "2025-01-15T10:05:00Z" {
		t.Errorf("evts[1].Timestamp = %q, want 2025-01-15T10:05:00Z", evts[1].Timestamp)
	}
	if evts[2].Timestamp != "2025-01-15T10:00:00Z" {
		t.Errorf("evts[2].Timestamp = %q, want 2025-01-15T10:00:00Z", evts[2].Timestamp)
	}
}

func TestReadAllWithLimit(t *testing.T) {
	wt1 := t.TempDir()
	wt2 := t.TempDir()

	for i := 0; i < 5; i++ {
		Append(wt1, Event{Timestamp: "2025-01-15T10:00:00Z", Quest: "a", Type: GateSubmitted})
		Append(wt2, Event{Timestamp: "2025-01-15T10:01:00Z", Quest: "b", Type: GateSubmitted})
	}

	evts, err := ReadAll([]string{wt1, wt2}, 3)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(evts) != 3 {
		t.Fatalf("got %d events, want 3", len(evts))
	}
}
