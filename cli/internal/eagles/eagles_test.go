package eagles

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/justinjdev/fellowship/cli/internal/gitutil"
)

// writeQuestState creates a quest-state.json in worktree/tmp.
func writeQuestState(t *testing.T, worktree string, phase string, gatePending bool, gateID *string, questName string) {
	t.Helper()
	dir := filepath.Join(worktree, "tmp")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("creating tmp dir: %v", err)
	}

	s := map[string]interface{}{
		"version":            1,
		"quest_name":         questName,
		"task_id":            "t1",
		"team_name":          "team",
		"phase":              phase,
		"gate_pending":       gatePending,
		"gate_id":            gateID,
		"lembas_completed":   false,
		"metadata_updated":   false,
		"auto_approve_gates": []string{},
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		t.Fatalf("marshaling state: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "quest-state.json"), data, 0644); err != nil {
		t.Fatalf("writing quest-state.json: %v", err)
	}
}

// touchFile creates a file with the given modification time.
func touchFile(t *testing.T, path string, modTime time.Time) {
	t.Helper()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("creating dir %s: %v", dir, err)
	}
	if err := os.WriteFile(path, []byte("content"), 0644); err != nil {
		t.Fatalf("writing file %s: %v", path, err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("changing times for %s: %v", path, err)
	}
}

func TestClassifyHealthy(t *testing.T) {
	worktree := t.TempDir()
	writeQuestState(t, worktree, "Implement", false, nil, "quest-api")

	now := time.Now()
	// Create a recently modified file
	touchFile(t, filepath.Join(worktree, "src", "main.go"), now.Add(-2*time.Minute))

	opts := Options{
		GateThreshold: 10 * time.Minute,
		ZombieTimeout: 15 * time.Minute,
		Now:           now,
	}

	qh, err := classifyQuest(worktree, opts)
	if err != nil {
		t.Fatalf("classifyQuest: %v", err)
	}

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
	worktree := t.TempDir()
	now := time.Now()

	// Gate created 20 minutes ago
	gateTS := now.Add(-20 * time.Minute).Unix()
	gateID := fmt.Sprintf("gate-Plan-%d", gateTS)
	writeQuestState(t, worktree, "Plan", true, &gateID, "quest-auth")

	touchFile(t, filepath.Join(worktree, "src", "plan.md"), now.Add(-1*time.Minute))

	opts := Options{
		GateThreshold: 10 * time.Minute,
		ZombieTimeout: 15 * time.Minute,
		Now:           now,
	}

	qh, err := classifyQuest(worktree, opts)
	if err != nil {
		t.Fatalf("classifyQuest: %v", err)
	}

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
	worktree := t.TempDir()
	now := time.Now()

	// Gate created 5 minutes ago — within threshold
	gateTS := now.Add(-5 * time.Minute).Unix()
	gateID := fmt.Sprintf("gate-Plan-%d", gateTS)
	writeQuestState(t, worktree, "Plan", true, &gateID, "quest-fresh")

	touchFile(t, filepath.Join(worktree, "src", "plan.md"), now.Add(-1*time.Minute))

	opts := Options{
		GateThreshold: 10 * time.Minute,
		ZombieTimeout: 15 * time.Minute,
		Now:           now,
	}

	qh, err := classifyQuest(worktree, opts)
	if err != nil {
		t.Fatalf("classifyQuest: %v", err)
	}

	if qh.Health != Working {
		t.Errorf("Health = %q, want %q (gate pending within threshold)", qh.Health, Working)
	}
}

func TestClassifyZombie(t *testing.T) {
	worktree := t.TempDir()
	now := time.Now()

	writeQuestState(t, worktree, "Implement", false, nil, "quest-dead")

	// Last file change was 30 minutes ago
	touchFile(t, filepath.Join(worktree, "src", "old.go"), now.Add(-30*time.Minute))
	// Set the quest-state.json mod time to be old too
	os.Chtimes(filepath.Join(worktree, "tmp", "quest-state.json"), now.Add(-30*time.Minute), now.Add(-30*time.Minute))

	opts := Options{
		GateThreshold: 10 * time.Minute,
		ZombieTimeout: 15 * time.Minute,
		Now:           now,
	}

	qh, err := classifyQuest(worktree, opts)
	if err != nil {
		t.Fatalf("classifyQuest: %v", err)
	}

	if qh.Health != Zombie {
		t.Errorf("Health = %q, want %q", qh.Health, Zombie)
	}
	if qh.Action != "nudge" {
		t.Errorf("Action = %q, want %q (no checkpoint)", qh.Action, "nudge")
	}
}

func TestClassifyZombieWithCheckpoint(t *testing.T) {
	worktree := t.TempDir()
	now := time.Now()

	writeQuestState(t, worktree, "Implement", false, nil, "quest-resumable")

	// Last file change was 30 minutes ago
	touchFile(t, filepath.Join(worktree, "src", "old.go"), now.Add(-30*time.Minute))
	os.Chtimes(filepath.Join(worktree, "tmp", "quest-state.json"), now.Add(-30*time.Minute), now.Add(-30*time.Minute))

	// Create checkpoint
	touchFile(t, filepath.Join(worktree, "tmp", "checkpoint.md"), now.Add(-30*time.Minute))

	opts := Options{
		GateThreshold: 10 * time.Minute,
		ZombieTimeout: 15 * time.Minute,
		Now:           now,
	}

	qh, err := classifyQuest(worktree, opts)
	if err != nil {
		t.Fatalf("classifyQuest: %v", err)
	}

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
	worktree := t.TempDir()
	now := time.Now()

	writeQuestState(t, worktree, "Complete", false, nil, "quest-done")
	touchFile(t, filepath.Join(worktree, "src", "done.go"), now.Add(-60*time.Minute))
	os.Chtimes(filepath.Join(worktree, "tmp", "quest-state.json"), now.Add(-60*time.Minute), now.Add(-60*time.Minute))

	opts := Options{
		GateThreshold: 10 * time.Minute,
		ZombieTimeout: 15 * time.Minute,
		Now:           now,
	}

	qh, err := classifyQuest(worktree, opts)
	if err != nil {
		t.Fatalf("classifyQuest: %v", err)
	}

	if qh.Health != Complete {
		t.Errorf("Health = %q, want %q", qh.Health, Complete)
	}
	if qh.Action != "none" {
		t.Errorf("Action = %q, want %q", qh.Action, "none")
	}
}

func TestClassifyIdle(t *testing.T) {
	worktree := t.TempDir()
	now := time.Now()

	writeQuestState(t, worktree, "Onboard", false, nil, "")
	touchFile(t, filepath.Join(worktree, "src", "empty.go"), now.Add(-1*time.Minute))

	opts := Options{
		GateThreshold: 10 * time.Minute,
		ZombieTimeout: 15 * time.Minute,
		Now:           now,
	}

	qh, err := classifyQuest(worktree, opts)
	if err != nil {
		t.Fatalf("classifyQuest: %v", err)
	}

	if qh.Health != Idle {
		t.Errorf("Health = %q, want %q", qh.Health, Idle)
	}
	if qh.Action != "none" {
		t.Errorf("Action = %q, want %q", qh.Action, "none")
	}
}

func TestClassifyStalledNoGateID(t *testing.T) {
	worktree := t.TempDir()
	now := time.Now()

	writeQuestState(t, worktree, "Review", true, nil, "quest-stuck")
	touchFile(t, filepath.Join(worktree, "src", "main.go"), now.Add(-1*time.Minute))

	opts := Options{
		GateThreshold: 10 * time.Minute,
		ZombieTimeout: 15 * time.Minute,
		Now:           now,
	}

	qh, err := classifyQuest(worktree, opts)
	if err != nil {
		t.Fatalf("classifyQuest: %v", err)
	}

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

	path := filepath.Join(root, "tmp", "eagles-report.json")
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

	// Check it contains key elements
	for _, want := range []string{"Fellowship Eagles Report", "quest-api", "Implement", "working", "none", "Problems: 0"} {
		if !contains(output, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestProblemCount(t *testing.T) {
	// Manually build a report to verify problem counting
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
