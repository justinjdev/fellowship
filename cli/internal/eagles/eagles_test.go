package eagles

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/justinjdev/fellowship/cli/internal/db"
	"github.com/justinjdev/fellowship/cli/internal/gitutil"
	"github.com/justinjdev/fellowship/cli/internal/herald"
	"github.com/justinjdev/fellowship/cli/internal/state"
)

// seedQuest inserts a quest state and optionally herald tidings into the test DB.
func seedQuest(t *testing.T, d *db.DB, s *state.State) {
	t.Helper()
	if err := d.WithConn(context.Background(), func(conn *db.Conn) error {
		return state.Upsert(conn, s)
	}); err != nil {
		t.Fatalf("seeding quest %s: %v", s.QuestName, err)
	}
}

// seedTiding inserts a herald tiding.
func seedTiding(t *testing.T, d *db.DB, tiding herald.Tiding) {
	t.Helper()
	if err := d.WithConn(context.Background(), func(conn *db.Conn) error {
		return herald.Announce(conn, tiding)
	}); err != nil {
		t.Fatalf("seeding tiding for %s: %v", tiding.Quest, err)
	}
}

func TestClassifyHealthy(t *testing.T) {
	d := db.OpenTest(t)
	now := time.Now().UTC()

	seedQuest(t, d, &state.State{
		QuestName: "quest-api",
		TaskID:    "t1",
		TeamName:  "team",
		Phase:     "Implement",
	})
	seedTiding(t, d, herald.Tiding{
		Timestamp: now.Add(-2 * time.Minute).Format(time.RFC3339),
		Quest:     "quest-api",
		Type:      herald.PhaseTransition,
		Phase:     "Implement",
	})

	opts := Options{
		GateThreshold: 10 * time.Minute,
		ZombieTimeout: 15 * time.Minute,
		Now:           now,
	}

	var report *EaglesReport
	err := d.WithConn(context.Background(), func(conn *db.Conn) error {
		var err error
		report, err = Sweep(conn, opts)
		return err
	})
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}

	if len(report.Quests) != 1 {
		t.Fatalf("len(Quests) = %d, want 1", len(report.Quests))
	}

	qh := report.Quests[0]
	if qh.Health != Working {
		t.Errorf("Health = %q, want %q", qh.Health, Working)
	}
	if qh.Action != "none" {
		t.Errorf("Action = %q, want %q", qh.Action, "none")
	}
	if qh.Name != "quest-api" {
		t.Errorf("Name = %q, want %q", qh.Name, "quest-api")
	}
	if qh.Phase != "Implement" {
		t.Errorf("Phase = %q, want %q", qh.Phase, "Implement")
	}
}

func TestClassifyStalledWithGateID(t *testing.T) {
	d := db.OpenTest(t)
	now := time.Now().UTC()

	// Gate created 20 minutes ago
	gateTS := now.Add(-20 * time.Minute).Unix()
	gateID := fmt.Sprintf("gate-Plan-%d", gateTS)

	seedQuest(t, d, &state.State{
		QuestName:   "quest-auth",
		TaskID:      "t2",
		TeamName:    "team",
		Phase:       "Plan",
		GatePending: true,
		GateID:      &gateID,
	})

	opts := Options{
		GateThreshold: 10 * time.Minute,
		ZombieTimeout: 15 * time.Minute,
		Now:           now,
	}

	var report *EaglesReport
	err := d.WithConn(context.Background(), func(conn *db.Conn) error {
		var err error
		report, err = Sweep(conn, opts)
		return err
	})
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}

	if len(report.Quests) != 1 {
		t.Fatalf("len(Quests) = %d, want 1", len(report.Quests))
	}

	qh := report.Quests[0]
	if qh.Health != Stalled {
		t.Errorf("Health = %q, want %q", qh.Health, Stalled)
	}
	if qh.Action != "nudge" {
		t.Errorf("Action = %q, want %q", qh.Action, "nudge")
	}
	if qh.GatePendingSec < 1199 { // ~20 min
		t.Errorf("GatePendingSec = %d, expected >= 1199", qh.GatePendingSec)
	}
}

func TestClassifyStalledGatePendingWithinThreshold(t *testing.T) {
	d := db.OpenTest(t)
	now := time.Now().UTC()

	// Gate created 5 minutes ago — within threshold
	gateTS := now.Add(-5 * time.Minute).Unix()
	gateID := fmt.Sprintf("gate-Plan-%d", gateTS)

	seedQuest(t, d, &state.State{
		QuestName:   "quest-fresh",
		TaskID:      "t3",
		TeamName:    "team",
		Phase:       "Plan",
		GatePending: true,
		GateID:      &gateID,
	})
	seedTiding(t, d, herald.Tiding{
		Timestamp: now.Add(-1 * time.Minute).Format(time.RFC3339),
		Quest:     "quest-fresh",
		Type:      herald.GateSubmitted,
		Phase:     "Plan",
	})

	opts := Options{
		GateThreshold: 10 * time.Minute,
		ZombieTimeout: 15 * time.Minute,
		Now:           now,
	}

	var report *EaglesReport
	err := d.WithConn(context.Background(), func(conn *db.Conn) error {
		var err error
		report, err = Sweep(conn, opts)
		return err
	})
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}

	qh := report.Quests[0]
	if qh.Health != Working {
		t.Errorf("Health = %q, want %q (gate pending within threshold)", qh.Health, Working)
	}
}

func TestClassifyZombie(t *testing.T) {
	d := db.OpenTest(t)
	now := time.Now().UTC()

	seedQuest(t, d, &state.State{
		QuestName: "quest-dead",
		TaskID:    "t4",
		TeamName:  "team",
		Phase:     "Implement",
	})
	// Last activity was 30 minutes ago
	seedTiding(t, d, herald.Tiding{
		Timestamp: now.Add(-30 * time.Minute).Format(time.RFC3339),
		Quest:     "quest-dead",
		Type:      herald.PhaseTransition,
		Phase:     "Implement",
	})

	opts := Options{
		GateThreshold: 10 * time.Minute,
		ZombieTimeout: 15 * time.Minute,
		Now:           now,
	}

	var report *EaglesReport
	err := d.WithConn(context.Background(), func(conn *db.Conn) error {
		var err error
		report, err = Sweep(conn, opts)
		return err
	})
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}

	qh := report.Quests[0]
	if qh.Health != Zombie {
		t.Errorf("Health = %q, want %q", qh.Health, Zombie)
	}
	if qh.Action != "nudge" {
		t.Errorf("Action = %q, want %q (no checkpoint)", qh.Action, "nudge")
	}
}

func TestClassifyZombieWithCheckpoint(t *testing.T) {
	d := db.OpenTest(t)
	now := time.Now().UTC()

	seedQuest(t, d, &state.State{
		QuestName: "quest-resumable",
		TaskID:    "t5",
		TeamName:  "team",
		Phase:     "Implement",
	})
	// Last activity was 30 minutes ago
	seedTiding(t, d, herald.Tiding{
		Timestamp: now.Add(-30 * time.Minute).Format(time.RFC3339),
		Quest:     "quest-resumable",
		Type:      herald.PhaseTransition,
		Phase:     "Implement",
	})
	// Has a lembas_completed checkpoint
	seedTiding(t, d, herald.Tiding{
		Timestamp: now.Add(-30 * time.Minute).Format(time.RFC3339),
		Quest:     "quest-resumable",
		Type:      herald.LembasCompleted,
		Phase:     "Implement",
	})

	opts := Options{
		GateThreshold: 10 * time.Minute,
		ZombieTimeout: 15 * time.Minute,
		Now:           now,
	}

	var report *EaglesReport
	err := d.WithConn(context.Background(), func(conn *db.Conn) error {
		var err error
		report, err = Sweep(conn, opts)
		return err
	})
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}

	qh := report.Quests[0]
	if qh.Health != Zombie {
		t.Errorf("Health = %q, want %q", qh.Health, Zombie)
	}
	if qh.Action != "respawn" {
		t.Errorf("Action = %q, want %q (has checkpoint)", qh.Action, "respawn")
	}
	if !qh.HasCheckpoint {
		t.Error("HasCheckpoint = false, want true")
	}
}

func TestClassifyComplete(t *testing.T) {
	d := db.OpenTest(t)
	now := time.Now().UTC()

	seedQuest(t, d, &state.State{
		QuestName: "quest-done",
		TaskID:    "t6",
		TeamName:  "team",
		Phase:     "Complete",
	})

	opts := Options{
		GateThreshold: 10 * time.Minute,
		ZombieTimeout: 15 * time.Minute,
		Now:           now,
	}

	var report *EaglesReport
	err := d.WithConn(context.Background(), func(conn *db.Conn) error {
		var err error
		report, err = Sweep(conn, opts)
		return err
	})
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}

	qh := report.Quests[0]
	if qh.Health != Complete {
		t.Errorf("Health = %q, want %q", qh.Health, Complete)
	}
	if qh.Action != "none" {
		t.Errorf("Action = %q, want %q", qh.Action, "none")
	}
}

func TestClassifyIdle(t *testing.T) {
	d := db.OpenTest(t)
	now := time.Now().UTC()

	seedQuest(t, d, &state.State{
		QuestName: "",
		Phase:     "Onboard",
	})

	opts := Options{
		GateThreshold: 10 * time.Minute,
		ZombieTimeout: 15 * time.Minute,
		Now:           now,
	}

	var report *EaglesReport
	err := d.WithConn(context.Background(), func(conn *db.Conn) error {
		var err error
		report, err = Sweep(conn, opts)
		return err
	})
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}

	if len(report.Quests) != 1 {
		t.Fatalf("len(Quests) = %d, want 1", len(report.Quests))
	}
	qh := report.Quests[0]
	if qh.Health != Idle {
		t.Errorf("Health = %q, want %q", qh.Health, Idle)
	}
	if qh.Action != "none" {
		t.Errorf("Action = %q, want %q", qh.Action, "none")
	}
}

func TestClassifyStalledNoGateID(t *testing.T) {
	d := db.OpenTest(t)
	now := time.Now().UTC()

	seedQuest(t, d, &state.State{
		QuestName:   "quest-stuck",
		TaskID:      "t7",
		TeamName:    "team",
		Phase:       "Review",
		GatePending: true,
		GateID:      nil,
	})

	opts := Options{
		GateThreshold: 10 * time.Minute,
		ZombieTimeout: 15 * time.Minute,
		Now:           now,
	}

	var report *EaglesReport
	err := d.WithConn(context.Background(), func(conn *db.Conn) error {
		var err error
		report, err = Sweep(conn, opts)
		return err
	})
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}

	qh := report.Quests[0]
	if qh.Health != Stalled {
		t.Errorf("Health = %q, want %q", qh.Health, Stalled)
	}
	if qh.Action != "nudge" {
		t.Errorf("Action = %q, want %q", qh.Action, "nudge")
	}
}

func TestGateAge(t *testing.T) {
	now := time.Unix(1700000600, 0)

	tests := []struct {
		gateID string
		want   int
	}{
		{"gate-Plan-1700000000", 600},
		{"gate-Implement-1700000600", 0},
		{"gate-Review-1700001000", 0}, // future gate
		{"invalid", 0},
		{"gate-Plan", 0}, // too few parts
	}

	for _, tt := range tests {
		t.Run(tt.gateID, func(t *testing.T) {
			got := gitutil.GateAge(tt.gateID, now)
			if got != tt.want {
				t.Errorf("gitutil.GateAge(%q) = %d, want %d", tt.gateID, got, tt.want)
			}
		})
	}
}

func TestWriteReport(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	report := &EaglesReport{
		Timestamp: "2025-01-15T10:30:00Z",
		Quests: []QuestHealth{
			{
				Name:          "quest-api",
				Worktree:      "/tmp/wt/quest-api",
				Phase:         "Implement",
				Health:        Working,
				HasCheckpoint: false,
				LastActivity:  "2025-01-15T10:25:00Z",
				Action:        "none",
			},
			{
				Name:           "quest-auth",
				Worktree:       "/tmp/wt/quest-auth",
				Phase:          "Plan",
				Health:         Stalled,
				GatePendingSec: 1200,
				HasCheckpoint:  true,
				LastActivity:   "2025-01-15T10:10:00Z",
				Action:         "nudge",
			},
		},
		Problems: 1,
	}

	if err := WriteReport(root, report); err != nil {
		t.Fatalf("WriteReport: %v", err)
	}

	path := filepath.Join(root, ".fellowship", "eagles-report.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading report: %v", err)
	}

	var loaded EaglesReport
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("unmarshaling report: %v", err)
	}

	if loaded.Timestamp != report.Timestamp {
		t.Errorf("Timestamp = %q, want %q", loaded.Timestamp, report.Timestamp)
	}
	if len(loaded.Quests) != 2 {
		t.Fatalf("len(Quests) = %d, want 2", len(loaded.Quests))
	}
	if loaded.Problems != 1 {
		t.Errorf("Problems = %d, want 1", loaded.Problems)
	}
	if loaded.Quests[0].Health != Working {
		t.Errorf("Quests[0].Health = %q, want %q", loaded.Quests[0].Health, Working)
	}
	if loaded.Quests[1].Health != Stalled {
		t.Errorf("Quests[1].Health = %q, want %q", loaded.Quests[1].Health, Stalled)
	}
}

func TestFormatTable(t *testing.T) {
	report := &EaglesReport{
		Timestamp: "2025-01-15T10:30:00Z",
		Quests: []QuestHealth{
			{
				Name:         "quest-api",
				Worktree:     "/tmp/wt/quest-api",
				Phase:        "Implement",
				Health:       Working,
				LastActivity: "2025-01-15T10:25:00Z",
				Action:       "none",
			},
		},
		Problems: 0,
	}

	output := FormatTable(report)
	if output == "" {
		t.Fatal("FormatTable returned empty string")
	}

	for _, want := range []string{"Fellowship Eagles Report", "quest-api", "Implement", "working", "none", "Problems: 0"} {
		if !contains(output, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestProblemCount(t *testing.T) {
	report := &EaglesReport{
		Timestamp: "2025-01-15T10:30:00Z",
		Quests:    []QuestHealth{},
	}

	healths := []struct {
		health  HealthState
		problem bool
	}{
		{Working, false},
		{Complete, false},
		{Stalled, true},
		{Zombie, true},
		{Idle, true},
	}

	for _, h := range healths {
		qh := QuestHealth{Health: h.health}
		report.Quests = append(report.Quests, qh)
		if h.problem {
			report.Problems++
		}
	}

	if report.Problems != 3 {
		t.Errorf("Problems = %d, want 3", report.Problems)
	}
}

func TestSweepMultipleQuests(t *testing.T) {
	d := db.OpenTest(t)
	now := time.Now().UTC()

	// Seed multiple quests with different states
	seedQuest(t, d, &state.State{
		QuestName: "quest-a",
		Phase:     "Implement",
	})
	seedTiding(t, d, herald.Tiding{
		Timestamp: now.Add(-1 * time.Minute).Format(time.RFC3339),
		Quest:     "quest-a",
		Type:      herald.PhaseTransition,
		Phase:     "Implement",
	})

	seedQuest(t, d, &state.State{
		QuestName: "quest-b",
		Phase:     "Complete",
	})

	gateID := fmt.Sprintf("gate-Plan-%d", now.Add(-20*time.Minute).Unix())
	seedQuest(t, d, &state.State{
		QuestName:   "quest-c",
		Phase:       "Plan",
		GatePending: true,
		GateID:      &gateID,
	})

	opts := Options{
		GateThreshold: 10 * time.Minute,
		ZombieTimeout: 15 * time.Minute,
		Now:           now,
	}

	var report *EaglesReport
	err := d.WithConn(context.Background(), func(conn *db.Conn) error {
		var err error
		report, err = Sweep(conn, opts)
		return err
	})
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}

	if len(report.Quests) != 3 {
		t.Fatalf("len(Quests) = %d, want 3", len(report.Quests))
	}

	// Find each quest by name
	healthMap := map[string]HealthState{}
	for _, q := range report.Quests {
		healthMap[q.Name] = q.Health
	}

	if healthMap["quest-a"] != Working {
		t.Errorf("quest-a: Health = %q, want %q", healthMap["quest-a"], Working)
	}
	if healthMap["quest-b"] != Complete {
		t.Errorf("quest-b: Health = %q, want %q", healthMap["quest-b"], Complete)
	}
	if healthMap["quest-c"] != Stalled {
		t.Errorf("quest-c: Health = %q, want %q", healthMap["quest-c"], Stalled)
	}

	if report.Problems != 1 {
		t.Errorf("Problems = %d, want 1", report.Problems)
	}
}

func TestSweepEmptyDB(t *testing.T) {
	d := db.OpenTest(t)
	now := time.Now().UTC()

	opts := Options{
		GateThreshold: 10 * time.Minute,
		ZombieTimeout: 15 * time.Minute,
		Now:           now,
	}

	var report *EaglesReport
	err := d.WithConn(context.Background(), func(conn *db.Conn) error {
		var err error
		report, err = Sweep(conn, opts)
		return err
	})
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}

	if len(report.Quests) != 0 {
		t.Errorf("len(Quests) = %d, want 0", len(report.Quests))
	}
	if report.Problems != 0 {
		t.Errorf("Problems = %d, want 0", report.Problems)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
