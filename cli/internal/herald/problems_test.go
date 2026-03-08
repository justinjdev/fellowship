package herald

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeQuestState(t *testing.T, dir string, phase string, gatePending bool, gateID *string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	dataDir := filepath.Join(dir, ".fellowship")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("creating data dir: %v", err)
	}

	state := map[string]interface{}{
		"version":            1,
		"quest_name":         filepath.Base(dir),
		"phase":              phase,
		"gate_pending":       gatePending,
		"gate_id":            gateID,
		"lembas_completed":   false,
		"metadata_updated":   false,
		"auto_approve_gates": []string{},
	}
	data, _ := json.MarshalIndent(state, "", "  ")
	if err := os.WriteFile(filepath.Join(dataDir, "quest-state.json"), data, 0644); err != nil {
		t.Fatalf("writing quest-state.json: %v", err)
	}
}

func TestStalledDetection(t *testing.T) {
	dir := t.TempDir()
	oldTimestamp := time.Now().Add(-15 * time.Minute).Unix()
	gateID := fmt.Sprintf("gate-Plan-%d", oldTimestamp)
	writeQuestState(t, dir, "Plan", true, &gateID)

	problems := DetectProblems([]string{dir})

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
}

func TestStalledNotDetectedWhenRecent(t *testing.T) {
	dir := t.TempDir()
	recentTimestamp := time.Now().Add(-2 * time.Minute).Unix()
	gateID := fmt.Sprintf("gate-Plan-%d", recentTimestamp)
	writeQuestState(t, dir, "Plan", true, &gateID)

	problems := DetectProblems([]string{dir})

	for _, p := range problems {
		if p.Type == "stalled" {
			t.Errorf("unexpected stalled problem: %v", p)
		}
	}
}

func TestZombieDetection(t *testing.T) {
	dir := t.TempDir()
	writeQuestState(t, dir, "Implement", false, nil)

	// Write an old tiding
	oldTime := time.Now().Add(-20 * time.Minute).UTC().Format(time.RFC3339)
	Announce(dir, Tiding{
		Timestamp: oldTime,
		Quest:     "test-quest",
		Type:      MetadataUpdated,
	})

	problems := DetectProblems([]string{dir})

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
}

func TestZombieNotDetectedWhenComplete(t *testing.T) {
	dir := t.TempDir()
	writeQuestState(t, dir, "Complete", false, nil)

	oldTime := time.Now().Add(-20 * time.Minute).UTC().Format(time.RFC3339)
	Announce(dir, Tiding{
		Timestamp: oldTime,
		Quest:     "test-quest",
		Type:      MetadataUpdated,
	})

	problems := DetectProblems([]string{dir})

	for _, p := range problems {
		if p.Type == "zombie" {
			t.Errorf("unexpected zombie problem for Complete quest: %v", p)
		}
	}
}

func TestStrugglingDetection(t *testing.T) {
	dir := t.TempDir()
	writeQuestState(t, dir, "Plan", false, nil)

	now := time.Now().UTC().Format(time.RFC3339)
	// Two rejections for the same phase
	Announce(dir, Tiding{
		Timestamp: now,
		Quest:     "test-quest",
		Type:      GateRejected,
		Phase:     "Plan",
	})
	Announce(dir, Tiding{
		Timestamp: now,
		Quest:     "test-quest",
		Type:      GateRejected,
		Phase:     "Plan",
	})

	problems := DetectProblems([]string{dir})

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
}

func TestStrugglingNotDetectedWithOneRejection(t *testing.T) {
	dir := t.TempDir()
	writeQuestState(t, dir, "Plan", false, nil)

	now := time.Now().UTC().Format(time.RFC3339)
	Announce(dir, Tiding{
		Timestamp: now,
		Quest:     "test-quest",
		Type:      GateRejected,
		Phase:     "Plan",
	})

	problems := DetectProblems([]string{dir})

	for _, p := range problems {
		if p.Type == "struggling" {
			t.Errorf("unexpected struggling problem with only 1 rejection: %v", p)
		}
	}
}

func TestNoProblemsForHealthyQuest(t *testing.T) {
	dir := t.TempDir()
	writeQuestState(t, dir, "Implement", false, nil)

	now := time.Now().UTC().Format(time.RFC3339)
	Announce(dir, Tiding{
		Timestamp: now,
		Quest:     "test-quest",
		Type:      GateApproved,
		Phase:     "Plan",
	})

	problems := DetectProblems([]string{dir})

	if len(problems) != 0 {
		t.Errorf("expected no problems, got %v", problems)
	}
}
