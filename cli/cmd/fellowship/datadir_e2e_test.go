package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/justinjdev/fellowship/cli/internal/state"
)

// A project that configures dataDir keeps that config in the MAIN worktree.
// Hooks used to resolve the name from their own session's top-level, so a
// teammate inside a linked worktree read no project config at all: it exempted
// ".fellowship" while every coordination write went to the configured
// directory — blocking the writes the phase rule is supposed to allow.
func TestRunHookWith_DataDirResolvedFromTheMainRepo(t *testing.T) {
	root := newMainRepo(t)
	t.Setenv("HOME", t.TempDir()) // no user config
	worktree := addWorktree(t, root, "quest-datadir-alpha")

	// The project config always lives in .fellowship/config.json, whatever it
	// renames the working directory to.
	if err := os.MkdirAll(filepath.Join(root, ".fellowship"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".fellowship", "config.json"),
		[]byte(`{"dataDir":".queststate"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	d := fellowshipWith(t, root, map[string]string{"quest-datadir-alpha": worktree})
	setQuestState(t, d, &state.State{QuestName: "quest-datadir-alpha", Phase: "Research"})

	configured := filepath.Join(worktree, ".queststate", "checkpoint.md")
	if got := runHookWith("gate-guard", writeInput("teammate", configured), worktree, d); got != 0 {
		t.Errorf("write into the configured data directory: exit %d, want 0 (allow)", got)
	}
	source := filepath.Join(worktree, "src", "main.go")
	if got := runHookWith("gate-guard", writeInput("teammate", source), worktree, d); got != 2 {
		t.Errorf("source write during Research: exit %d, want 2 (block)", got)
	}
}
