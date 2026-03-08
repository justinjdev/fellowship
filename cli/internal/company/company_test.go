package company

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/justinjdev/fellowship/cli/internal/dashboard"
	"github.com/justinjdev/fellowship/cli/internal/state"
)

func TestCalculateProgress_MixedPhases(t *testing.T) {
	company := dashboard.CompanyEntry{
		Name:   "API Work",
		Quests: []string{"quest-endpoint", "quest-tests", "quest-docs"},
		Scouts: []string{"scout-review"},
	}

	quests := []dashboard.QuestStatus{
		{Name: "quest-endpoint", Phase: "Implement", GatePending: false},
		{Name: "quest-tests", Phase: "Complete", GatePending: false},
		{Name: "quest-docs", Phase: "Research", GatePending: true},
	}

	progress := CalculateProgress(company, quests)

	if progress.Name != "API Work" {
		t.Errorf("expected name 'API Work', got %q", progress.Name)
	}
	// Total includes quests + scouts
	if progress.Total != 4 {
		t.Errorf("expected total 4, got %d", progress.Total)
	}
	if progress.Completed != 1 {
		t.Errorf("expected 1 completed, got %d", progress.Completed)
	}
	// Implement+ includes Implement, Review, Complete
	if progress.InProgress != 2 {
		t.Errorf("expected 2 in_progress (Implement+), got %d", progress.InProgress)
	}
	if progress.Pending != 1 {
		t.Errorf("expected 1 pending gate, got %d", progress.Pending)
	}
}

func TestCalculateProgress_AllComplete(t *testing.T) {
	company := dashboard.CompanyEntry{
		Name:   "done-company",
		Quests: []string{"q1", "q2"},
	}
	quests := []dashboard.QuestStatus{
		{Name: "q1", Phase: "Complete"},
		{Name: "q2", Phase: "Complete"},
	}

	progress := CalculateProgress(company, quests)

	if progress.Completed != 2 {
		t.Errorf("expected 2 completed, got %d", progress.Completed)
	}
	if progress.Pending != 0 {
		t.Errorf("expected 0 pending, got %d", progress.Pending)
	}
}

func TestCalculateProgress_MissingQuests(t *testing.T) {
	company := dashboard.CompanyEntry{
		Name:   "sparse",
		Quests: []string{"exists", "missing"},
	}
	quests := []dashboard.QuestStatus{
		{Name: "exists", Phase: "Plan"},
	}

	progress := CalculateProgress(company, quests)

	// Missing quest should be gracefully skipped
	if progress.Completed != 0 {
		t.Errorf("expected 0 completed, got %d", progress.Completed)
	}
	if progress.InProgress != 0 {
		t.Errorf("expected 0 in_progress, got %d", progress.InProgress)
	}
}

func TestBatchApprove_MultipleWorktrees(t *testing.T) {
	tmpDir := t.TempDir()

	// Create two worktrees with pending gates
	wt1 := filepath.Join(tmpDir, "wt1")
	wt2 := filepath.Join(tmpDir, "wt2")
	os.MkdirAll(filepath.Join(wt1, ".fellowship"), 0755)
	os.MkdirAll(filepath.Join(wt2, ".fellowship"), 0755)

	writeState(t, filepath.Join(wt1, ".fellowship", "quest-state.json"), &state.State{
		Version:     1,
		Phase:       "Research",
		GatePending: true,
	})
	writeState(t, filepath.Join(wt2, ".fellowship", "quest-state.json"), &state.State{
		Version:     1,
		Phase:       "Plan",
		GatePending: true,
	})

	company := dashboard.CompanyEntry{
		Name:   "batch-test",
		Quests: []string{"q1", "q2"},
	}
	fs := &dashboard.FellowshipState{
		Quests: []dashboard.QuestEntry{
			{Name: "q1", Worktree: wt1},
			{Name: "q2", Worktree: wt2},
		},
	}

	approved, errs := BatchApprove(company, fs)

	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
	if len(approved) != 2 {
		t.Fatalf("expected 2 approved, got %d", len(approved))
	}

	// Verify phases were advanced
	s1, _ := state.Load(filepath.Join(wt1, ".fellowship", "quest-state.json"))
	if s1.Phase != "Plan" {
		t.Errorf("expected q1 phase 'Plan', got %q", s1.Phase)
	}
	if s1.GatePending {
		t.Error("expected q1 gate_pending to be false")
	}

	s2, _ := state.Load(filepath.Join(wt2, ".fellowship", "quest-state.json"))
	if s2.Phase != "Implement" {
		t.Errorf("expected q2 phase 'Implement', got %q", s2.Phase)
	}
}

func TestBatchApprove_NoPendingGates(t *testing.T) {
	tmpDir := t.TempDir()
	wt := filepath.Join(tmpDir, "wt")
	os.MkdirAll(filepath.Join(wt, ".fellowship"), 0755)

	writeState(t, filepath.Join(wt, ".fellowship", "quest-state.json"), &state.State{
		Version:     1,
		Phase:       "Implement",
		GatePending: false,
	})

	company := dashboard.CompanyEntry{
		Name:   "no-gates",
		Quests: []string{"q1"},
	}
	fs := &dashboard.FellowshipState{
		Quests: []dashboard.QuestEntry{
			{Name: "q1", Worktree: wt},
		},
	}

	approved, errs := BatchApprove(company, fs)

	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
	if len(approved) != 0 {
		t.Errorf("expected 0 approved (no-op), got %d", len(approved))
	}
}

func TestBatchApprove_MissingWorktree(t *testing.T) {
	company := dashboard.CompanyEntry{
		Name:   "missing-wt",
		Quests: []string{"q1", "q2"},
	}
	fs := &dashboard.FellowshipState{
		Quests: []dashboard.QuestEntry{
			{Name: "q1", Worktree: "/nonexistent/path"},
			// q2 has no worktree mapping at all
		},
	}

	approved, errs := BatchApprove(company, fs)

	// q1 should produce an error (can't load state), q2 is skipped (no mapping)
	if len(approved) != 0 {
		t.Errorf("expected 0 approved, got %d", len(approved))
	}
	if len(errs) != 1 {
		t.Errorf("expected 1 error (for q1 missing state), got %d", len(errs))
	}
}

func TestFindCompanyForQuest(t *testing.T) {
	companies := []dashboard.CompanyEntry{
		{Name: "API", Quests: []string{"q-api", "q-tests"}},
		{Name: "Docs", Quests: []string{"q-docs"}},
	}

	if got := FindCompanyForQuest(companies, "q-api"); got != "API" {
		t.Errorf("expected 'API', got %q", got)
	}
	if got := FindCompanyForQuest(companies, "q-docs"); got != "Docs" {
		t.Errorf("expected 'Docs', got %q", got)
	}
	if got := FindCompanyForQuest(companies, "q-other"); got != "" {
		t.Errorf("expected empty string for ungrouped quest, got %q", got)
	}
}

func TestProgressSummary(t *testing.T) {
	p := CompanyProgress{
		Name:       "API Work",
		Total:      3,
		InProgress: 2,
	}
	got := ProgressSummary(p)
	expected := "2/3 quests in Implement+"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func writeState(t *testing.T, path string, s *state.State) {
	t.Helper()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}
