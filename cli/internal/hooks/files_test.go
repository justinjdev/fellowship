package hooks

import (
	"context"
	"testing"

	"github.com/justinjdev/fellowship/cli/internal/db"
	"github.com/justinjdev/fellowship/cli/internal/state"
	"github.com/justinjdev/fellowship/cli/internal/tome"
)

func seedQuest(t *testing.T, d *db.DB, name string) {
	t.Helper()
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		return state.Upsert(conn, &state.State{QuestName: name, Phase: "Implement"})
	})
}

func TestFileTrack_EditToolInput(t *testing.T) {
	d := db.OpenTest(t)
	seedQuest(t, d, "q1")

	s := &state.State{Phase: "Implement"}
	input := &HookInput{
		ToolInput: ToolInput{FilePath: "/home/user/project/main.go"},
	}

	var modified bool
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		modified = FileTrack(conn, s, input, "q1")
		return nil
	})
	if !modified {
		t.Error("FileTrack should return true on first file write")
	}

	d.WithConn(context.Background(), func(conn *db.Conn) error {
		files, err := tome.LoadFiles(conn, "q1")
		if err != nil {
			t.Fatalf("loading files: %v", err)
		}
		if len(files) != 1 {
			t.Fatalf("files len = %d, want 1", len(files))
		}
		if files[0] != "/home/user/project/main.go" {
			t.Errorf("files[0] = %q, want /home/user/project/main.go", files[0])
		}
		return nil
	})
}

func TestFileTrack_NotebookPath(t *testing.T) {
	d := db.OpenTest(t)
	seedQuest(t, d, "q1")

	s := &state.State{Phase: "Implement"}
	input := &HookInput{
		ToolInput: ToolInput{NotebookPath: "/home/user/project/analysis.ipynb"},
	}

	var modified bool
	d.WithTx(context.Background(), func(conn *db.Conn) error {
		modified = FileTrack(conn, s, input, "q1")
		return nil
	})
	if !modified {
		t.Error("FileTrack should return true for notebook path")
	}

	d.WithConn(context.Background(), func(conn *db.Conn) error {
		files, _ := tome.LoadFiles(conn, "q1")
		if len(files) != 1 || files[0] != "/home/user/project/analysis.ipynb" {
			t.Errorf("expected notebook path in files, got %v", files)
		}
		return nil
	})
}

func TestFileTrack_DataDirPathExclusion(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // ensure default datadir (.fellowship)
	d := db.OpenTest(t)
	seedQuest(t, d, "q1")

	s := &state.State{Phase: "Implement"}

	tests := []struct {
		name string
		path string
	}{
		{"absolute data dir", "/home/user/project/.fellowship/checkpoint.md"},
		{"relative data dir", ".fellowship/quest-state.json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := &HookInput{
				ToolInput: ToolInput{FilePath: tt.path},
			}
			d.WithTx(context.Background(), func(conn *db.Conn) error {
				modified := FileTrack(conn, s, input, "q1")
				if modified {
					t.Errorf("FileTrack should return false for data dir path %q", tt.path)
				}
				return nil
			})
		})
	}
}

func TestFileTrack_EmptyFilePath(t *testing.T) {
	d := db.OpenTest(t)
	seedQuest(t, d, "q1")

	s := &state.State{Phase: "Implement"}
	input := &HookInput{
		ToolInput: ToolInput{},
	}

	d.WithTx(context.Background(), func(conn *db.Conn) error {
		modified := FileTrack(conn, s, input, "q1")
		if modified {
			t.Error("FileTrack should return false when no file path present")
		}
		return nil
	})
}

func TestFileTrack_Deduplication(t *testing.T) {
	d := db.OpenTest(t)
	seedQuest(t, d, "q1")

	s := &state.State{Phase: "Implement"}
	input := &HookInput{
		ToolInput: ToolInput{FilePath: "/home/user/project/main.go"},
	}

	d.WithTx(context.Background(), func(conn *db.Conn) error {
		FileTrack(conn, s, input, "q1")
		modified := FileTrack(conn, s, input, "q1")
		if modified {
			t.Error("FileTrack should return false on duplicate file")
		}

		files, _ := tome.LoadFiles(conn, "q1")
		if len(files) != 1 {
			t.Errorf("files len = %d, want 1", len(files))
		}
		return nil
	})
}
