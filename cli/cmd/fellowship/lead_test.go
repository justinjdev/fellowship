package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/justinjdev/fellowship/cli/internal/state"
)

// writeLegacyLeadMarker writes the pre-store <data-dir>/lead file. Nothing in
// the CLI writes it any more — a teammate could, which is exactly the attack.
func writeLegacyLeadMarker(t *testing.T, root, sessionID string) {
	t.Helper()
	path := state.LeadMarkerPath(root, ".fellowship")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(state.Lead{SessionID: sessionID, Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// The lead marker used to live in the data directory, which every guard exempts
// from its write rules — so the marker naming the lead was writable by the very
// sessions it was there to identify. The store now names the lead, and a file
// that disagrees is ignored.
func TestRunWorktreeGuard_SpoofedMarkerCannotBecomeTheLead(t *testing.T) {
	root := newMainRepo(t)
	worktree := addWorktree(t, root, "quest-spoof-alpha")
	d := fellowshipWith(t, root, map[string]string{"quest-spoof-alpha": worktree})
	recordLead(t, d, root, "lead-session")
	writeLegacyLeadMarker(t, root, "teammate-session")

	target := filepath.Join(root, "src", "main.go")
	if got := runWorktreeGuard(context.Background(), d, root, writeInput("teammate-session", target)); got != 2 {
		t.Errorf("spoofed marker: runWorktreeGuard = %d, want 2 (block)", got)
	}
	if got := runWorktreeGuard(context.Background(), d, root, writeInput("lead-session", target)); got != 0 {
		t.Errorf("the real lead: runWorktreeGuard = %d, want 0 (allow)", got)
	}
}

// A fellowship initialized by the previous release recorded its lead in the
// marker file and has no lead row. That marker is still honored, for one
// release, while the store names nobody.
func TestRunWorktreeGuard_LegacyMarkerFallback(t *testing.T) {
	root := newMainRepo(t)
	worktree := addWorktree(t, root, "quest-legacy-alpha")
	d := fellowshipWith(t, root, map[string]string{"quest-legacy-alpha": worktree})
	writeLegacyLeadMarker(t, root, "old-lead-session")

	target := filepath.Join(root, "src", "main.go")
	if got := runWorktreeGuard(context.Background(), d, root, writeInput("old-lead-session", target)); got != 0 {
		t.Errorf("legacy lead: runWorktreeGuard = %d, want 0 (allow)", got)
	}
	if got := runWorktreeGuard(context.Background(), d, root, writeInput("someone-else", target)); got != 2 {
		t.Errorf("legacy non-lead: runWorktreeGuard = %d, want 2 (block)", got)
	}
}

// The store is nobody's to hand-edit, including in the main tree where no quest
// row exists at all.
func TestRunHookWith_StoreWriteIsRefusedEverywhere(t *testing.T) {
	root := newMainRepo(t)
	worktree := addWorktree(t, root, "quest-store-alpha")
	store := filepath.Join(root, ".fellowship", "fellowship.db")

	d := fellowshipWith(t, root, map[string]string{"quest-store-alpha": worktree})
	setQuestState(t, d, &state.State{QuestName: "quest-store-alpha", Phase: "Implement"})

	if got := runHookWith("gate-guard", writeInput("teammate", store), worktree, d); got != 2 {
		t.Errorf("teammate writing the store: exit %d, want 2", got)
	}
	// The main root has no quest row; the refusal is not a quest rule.
	if got := runHookWith("gate-guard", writeInput("lead", store), root, d); got != 2 {
		t.Errorf("main-tree write to the store: exit %d, want 2", got)
	}
	// An ordinary coordination write in the same directory is untouched.
	notes := filepath.Join(root, ".fellowship", "notes.md")
	if got := runHookWith("gate-guard", writeInput("teammate", notes), worktree, d); got != 0 {
		t.Errorf("coordination write: exit %d, want 0", got)
	}
}
