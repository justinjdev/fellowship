package tome

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "quest-tome.json")

	original := &QuestTome{
		Version:   1,
		QuestName: "test-quest",
		CreatedAt: "2025-01-01T00:00:00Z",
		Task:      "implement feature X",
		Status:    "active",
		PhasesCompleted: []PhaseRecord{
			{Phase: "Research", CompletedAt: "2025-01-01T01:00:00Z"},
		},
		GateHistory: []GateEvent{
			{Phase: "Research", Action: "submitted", Timestamp: "2025-01-01T01:00:00Z"},
		},
		FilesTouched: []string{"main.go", "lib.go"},
		Respawns:     1,
	}

	if err := Save(path, original); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.QuestName != original.QuestName {
		t.Errorf("QuestName = %q, want %q", loaded.QuestName, original.QuestName)
	}
	if loaded.Task != original.Task {
		t.Errorf("Task = %q, want %q", loaded.Task, original.Task)
	}
	if loaded.Status != original.Status {
		t.Errorf("Status = %q, want %q", loaded.Status, original.Status)
	}
	if len(loaded.PhasesCompleted) != 1 {
		t.Errorf("PhasesCompleted len = %d, want 1", len(loaded.PhasesCompleted))
	}
	if len(loaded.GateHistory) != 1 {
		t.Errorf("GateHistory len = %d, want 1", len(loaded.GateHistory))
	}
	if len(loaded.FilesTouched) != 2 {
		t.Errorf("FilesTouched len = %d, want 2", len(loaded.FilesTouched))
	}
	if loaded.Respawns != 1 {
		t.Errorf("Respawns = %d, want 1", loaded.Respawns)
	}
	if loaded.UpdatedAt == "" {
		t.Error("UpdatedAt should be set after Save")
	}
}

func TestLoadEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "quest-tome.json")
	os.WriteFile(path, []byte{}, 0644)

	_, err := Load(path)
	if err == nil {
		t.Error("Load should fail on empty file")
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load("/nonexistent/quest-tome.json")
	if err == nil {
		t.Error("Load should fail on missing file")
	}
}

func TestSaveAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "quest-tome.json")
	c := &QuestTome{Version: 1, Status: "active", PhasesCompleted: []PhaseRecord{}, GateHistory: []GateEvent{}, FilesTouched: []string{}}

	if err := Save(path, c); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Verify no .tmp file remains
	tmpPath := path + ".tmp"
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Error("tmp file should not remain after Save")
	}

	// Verify file is valid JSON
	data, _ := os.ReadFile(path)
	var check QuestTome
	if err := json.Unmarshal(data, &check); err != nil {
		t.Errorf("saved file is not valid JSON: %v", err)
	}
}

func TestRecordPhase(t *testing.T) {
	c := &QuestTome{PhasesCompleted: []PhaseRecord{}}

	RecordPhase(c, "Research")
	if len(c.PhasesCompleted) != 1 {
		t.Fatalf("PhasesCompleted len = %d, want 1", len(c.PhasesCompleted))
	}
	if c.PhasesCompleted[0].Phase != "Research" {
		t.Errorf("Phase = %q, want Research", c.PhasesCompleted[0].Phase)
	}
	if c.PhasesCompleted[0].CompletedAt == "" {
		t.Error("CompletedAt should be set")
	}

	RecordPhase(c, "Plan")
	if len(c.PhasesCompleted) != 2 {
		t.Fatalf("PhasesCompleted len = %d, want 2", len(c.PhasesCompleted))
	}
	if c.PhasesCompleted[1].Phase != "Plan" {
		t.Errorf("Phase = %q, want Plan", c.PhasesCompleted[1].Phase)
	}
}

func TestRecordGate(t *testing.T) {
	c := &QuestTome{GateHistory: []GateEvent{}}

	RecordGate(c, "Research", "submitted")
	if len(c.GateHistory) != 1 {
		t.Fatalf("GateHistory len = %d, want 1", len(c.GateHistory))
	}
	if c.GateHistory[0].Phase != "Research" {
		t.Errorf("Phase = %q, want Research", c.GateHistory[0].Phase)
	}
	if c.GateHistory[0].Action != "submitted" {
		t.Errorf("Action = %q, want submitted", c.GateHistory[0].Action)
	}
	if c.GateHistory[0].Timestamp == "" {
		t.Error("Timestamp should be set")
	}

	RecordGate(c, "Research", "approved")
	if len(c.GateHistory) != 2 {
		t.Fatalf("GateHistory len = %d, want 2", len(c.GateHistory))
	}
}

func TestRecordFiles_Deduplication(t *testing.T) {
	c := &QuestTome{FilesTouched: []string{"main.go"}}

	RecordFiles(c, []string{"main.go", "lib.go", "main.go"})
	if len(c.FilesTouched) != 2 {
		t.Fatalf("FilesTouched len = %d, want 2", len(c.FilesTouched))
	}

	expected := map[string]bool{"main.go": true, "lib.go": true}
	for _, f := range c.FilesTouched {
		if !expected[f] {
			t.Errorf("unexpected file: %q", f)
		}
	}
}

func TestRecordFiles_Empty(t *testing.T) {
	c := &QuestTome{FilesTouched: []string{"a.go"}}
	RecordFiles(c, []string{})
	if len(c.FilesTouched) != 1 {
		t.Errorf("FilesTouched len = %d, want 1", len(c.FilesTouched))
	}
}

func TestFindTome_Exists(t *testing.T) {
	dir := t.TempDir()
	tmpDir := filepath.Join(dir, "tmp")
	os.MkdirAll(tmpDir, 0755)
	tomePath := filepath.Join(tmpDir, "quest-tome.json")
	os.WriteFile(tomePath, []byte(`{}`), 0644)

	// FindTome uses git root; test with direct dir since no git repo
	found, err := FindTome(dir)
	if err != nil {
		t.Fatalf("FindTome: %v", err)
	}
	// In a non-git dir, FindTome falls back to fromDir
	if found != tomePath {
		t.Errorf("FindTome = %q, want %q", found, tomePath)
	}
}

func TestFindTome_NotExists(t *testing.T) {
	dir := t.TempDir()
	found, err := FindTome(dir)
	if err != nil {
		t.Fatalf("FindTome: %v", err)
	}
	if found != "" {
		t.Errorf("FindTome = %q, want empty string", found)
	}
}

func TestLoadOrCreate_NewTome(t *testing.T) {
	c := LoadOrCreate("/nonexistent/quest-tome.json")
	if c.Version != 1 {
		t.Errorf("Version = %d, want 1", c.Version)
	}
	if c.Status != "active" {
		t.Errorf("Status = %q, want active", c.Status)
	}
	if c.CreatedAt == "" {
		t.Error("CreatedAt should be set")
	}
}

func TestLoadOrCreate_ExistingTome(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "quest-tome.json")
	original := &QuestTome{Version: 1, QuestName: "existing", Status: "active", PhasesCompleted: []PhaseRecord{}, GateHistory: []GateEvent{}, FilesTouched: []string{}}
	Save(path, original)

	c := LoadOrCreate(path)
	if c.QuestName != "existing" {
		t.Errorf("QuestName = %q, want existing", c.QuestName)
	}
}
