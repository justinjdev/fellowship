package state

import (
	"os"
	"path/filepath"
	"testing"
)

func tmpState(t *testing.T, content string) string {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	stateDir := filepath.Join(dir, ".fellowship")
	os.MkdirAll(stateDir, 0755)
	path := filepath.Join(stateDir, "quest-state.json")
	os.WriteFile(path, []byte(content), 0644)
	return path
}

const validState = `{
  "version": 1,
  "quest_name": "test",
  "task_id": "1",
  "team_name": "test-team",
  "phase": "Research",
  "gate_pending": false,
  "gate_id": null,
  "lembas_completed": false,
  "metadata_updated": false,
  "auto_approve_gates": []
}`

func TestLoadState(t *testing.T) {
	path := tmpState(t, validState)
	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if s.Phase != "Research" {
		t.Errorf("Phase = %q, want Research", s.Phase)
	}
	if s.GatePending {
		t.Error("GatePending should be false")
	}
	if s.Version != 1 {
		t.Errorf("Version = %d, want 1", s.Version)
	}
}

func TestLoadState_MissingFile(t *testing.T) {
	_, err := Load("/nonexistent/path")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadState_InvalidJSON(t *testing.T) {
	path := tmpState(t, "not json")
	_, err := Load(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoadState_EmptyFile(t *testing.T) {
	path := tmpState(t, "")
	_, err := Load(path)
	if err == nil {
		t.Error("expected error for empty file")
	}
}

func TestSaveState(t *testing.T) {
	path := tmpState(t, validState)
	s, _ := Load(path)
	s.Phase = "Plan"
	s.LembasCompleted = true
	if err := Save(path, s); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	s2, _ := Load(path)
	if s2.Phase != "Plan" {
		t.Errorf("Phase = %q after save, want Plan", s2.Phase)
	}
	if !s2.LembasCompleted {
		t.Error("LembasCompleted should be true after save")
	}
}

func TestNextPhase(t *testing.T) {
	tests := []struct {
		current string
		want    string
		wantErr bool
	}{
		{"Onboard", "Research", false},
		{"Research", "Plan", false},
		{"Plan", "Implement", false},
		{"Implement", "Adversarial", false},
		{"Adversarial", "Review", false},
		{"Review", "Complete", false},
		{"Complete", "", true},
		{"InvalidPhase", "", true},
	}
	for _, tt := range tests {
		got, err := NextPhase(tt.current)
		if (err != nil) != tt.wantErr {
			t.Errorf("NextPhase(%q) error = %v, wantErr %v", tt.current, err, tt.wantErr)
		}
		if got != tt.want {
			t.Errorf("NextPhase(%q) = %q, want %q", tt.current, got, tt.want)
		}
	}
}

func TestIsEarlyPhase(t *testing.T) {
	early := []string{"Onboard", "Research", "Plan"}
	late := []string{"Implement", "Adversarial", "Review", "Complete"}
	for _, p := range early {
		if !IsEarlyPhase(p) {
			t.Errorf("IsEarlyPhase(%q) should be true", p)
		}
	}
	for _, p := range late {
		if IsEarlyPhase(p) {
			t.Errorf("IsEarlyPhase(%q) should be false", p)
		}
	}
}

func TestFindStateFile_NoFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	path, err := FindStateFile(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != "" {
		t.Errorf("expected empty path, got %q", path)
	}
}

func TestFindStateFile_SkipsWhenFellowshipStateExists(t *testing.T) {
	// Simulate the main repo root where both quest-state.json and
	// fellowship-state.json exist (lead's CWD). The hook should NOT
	// find the quest-state file so the lead isn't blocked.
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	dd := filepath.Join(dir, ".fellowship")
	os.MkdirAll(dd, 0755)
	os.WriteFile(filepath.Join(dd, "quest-state.json"), []byte(validState), 0644)
	os.WriteFile(filepath.Join(dd, "fellowship-state.json"), []byte(`{"version":1}`), 0644)

	// FindStateFile uses gitRoot which won't work in a temp dir, so it
	// falls back to fromDir. With both files present it should return "".
	path, err := FindStateFile(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != "" {
		t.Errorf("expected empty path when fellowship-state.json exists, got %q", path)
	}
}

func TestFindStateFile_ReturnsPathWhenOnlyQuestState(t *testing.T) {
	// Simulate a quest worktree where only quest-state.json exists.
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	dd := filepath.Join(dir, ".fellowship")
	os.MkdirAll(dd, 0755)
	os.WriteFile(filepath.Join(dd, "quest-state.json"), []byte(validState), 0644)

	path, err := FindStateFile(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := filepath.Join(dd, "quest-state.json")
	if path != expected {
		t.Errorf("got %q, want %q", path, expected)
	}
}
