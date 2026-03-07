package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/justinjdev/fellowship/cli/internal/tome"
	"github.com/justinjdev/fellowship/cli/internal/state"
)

func TestFileTrack_EditToolInput(t *testing.T) {
	dir := t.TempDir()
	tomePath := filepath.Join(dir, "quest-tome.json")
	s := &state.State{Phase: "Implement"}
	input := &HookInput{
		ToolInput: ToolInput{FilePath: "/home/user/project/main.go"},
	}

	modified := FileTrack(s, input, tomePath)
	if !modified {
		t.Error("FileTrack should return true on first file write")
	}

	data, err := os.ReadFile(tomePath)
	if err != nil {
		t.Fatalf("reading tome: %v", err)
	}
	var c tome.QuestTome
	if err := json.Unmarshal(data, &c); err != nil {
		t.Fatalf("parsing tome: %v", err)
	}
	if len(c.FilesTouched) != 1 {
		t.Fatalf("FilesTouched len = %d, want 1", len(c.FilesTouched))
	}
	if c.FilesTouched[0] != "/home/user/project/main.go" {
		t.Errorf("FilesTouched[0] = %q, want /home/user/project/main.go", c.FilesTouched[0])
	}
}

func TestFileTrack_NotebookPath(t *testing.T) {
	dir := t.TempDir()
	tomePath := filepath.Join(dir, "quest-tome.json")
	s := &state.State{Phase: "Implement"}
	input := &HookInput{
		ToolInput: ToolInput{NotebookPath: "/home/user/project/analysis.ipynb"},
	}

	modified := FileTrack(s, input, tomePath)
	if !modified {
		t.Error("FileTrack should return true for notebook path")
	}

	c, _ := tome.Load(tomePath)
	if len(c.FilesTouched) != 1 || c.FilesTouched[0] != "/home/user/project/analysis.ipynb" {
		t.Errorf("expected notebook path in FilesTouched, got %v", c.FilesTouched)
	}
}

func TestFileTrack_TmpPathExclusion(t *testing.T) {
	dir := t.TempDir()
	tomePath := filepath.Join(dir, "quest-tome.json")
	s := &state.State{Phase: "Implement"}

	tests := []struct {
		name string
		path string
	}{
		{"absolute tmp", "/home/user/project/tmp/checkpoint.md"},
		{"relative tmp", "tmp/quest-state.json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := &HookInput{
				ToolInput: ToolInput{FilePath: tt.path},
			}
			modified := FileTrack(s, input, tomePath)
			if modified {
				t.Errorf("FileTrack should return false for tmp path %q", tt.path)
			}
		})
	}
}

func TestFileTrack_EmptyFilePath(t *testing.T) {
	dir := t.TempDir()
	tomePath := filepath.Join(dir, "quest-tome.json")
	s := &state.State{Phase: "Implement"}
	input := &HookInput{
		ToolInput: ToolInput{},
	}

	modified := FileTrack(s, input, tomePath)
	if modified {
		t.Error("FileTrack should return false when no file path present")
	}
}

func TestFileTrack_Deduplication(t *testing.T) {
	dir := t.TempDir()
	tomePath := filepath.Join(dir, "quest-tome.json")
	s := &state.State{Phase: "Implement"}
	input := &HookInput{
		ToolInput: ToolInput{FilePath: "/home/user/project/main.go"},
	}

	FileTrack(s, input, tomePath)
	modified := FileTrack(s, input, tomePath)
	if modified {
		t.Error("FileTrack should return false on duplicate file")
	}

	c, _ := tome.Load(tomePath)
	if len(c.FilesTouched) != 1 {
		t.Errorf("FilesTouched len = %d, want 1", len(c.FilesTouched))
	}
}

func TestFileTrack_TomeCreationOnFirstWrite(t *testing.T) {
	dir := t.TempDir()
	tomePath := filepath.Join(dir, "quest-tome.json")
	s := &state.State{Phase: "Implement"}
	input := &HookInput{
		ToolInput: ToolInput{FilePath: "/home/user/project/new.go"},
	}

	// Tome file should not exist yet
	if _, err := os.Stat(tomePath); !os.IsNotExist(err) {
		t.Fatal("Tome file should not exist before first FileTrack call")
	}

	modified := FileTrack(s, input, tomePath)
	if !modified {
		t.Error("FileTrack should return true and create tome on first file write")
	}

	// Tome file should now exist
	c, err := tome.Load(tomePath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Version != 1 {
		t.Errorf("Version = %d, want 1", c.Version)
	}
	if c.Status != "active" {
		t.Errorf("Status = %q, want active", c.Status)
	}
}
