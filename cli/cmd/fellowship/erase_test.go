package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/justinjdev/fellowship/cli/internal/db"
	"github.com/justinjdev/fellowship/cli/internal/events"
	"github.com/justinjdev/fellowship/cli/internal/state"
)

// --- erase-to-escape --------------------------------------------------------

// Deleting <data-dir>/fellowship.db was the cheapest way to switch enforcement
// off: every hook read "no store here" and allowed. A repo whose main worktree
// still has a fellowship data directory is a fellowship whose store went
// missing, and gate hooks block there.
func TestStoreOpen_MissingStoreBlocksWhenAFellowshipIsExpected(t *testing.T) {
	root := newMainRepo(t)
	t.Setenv("HOME", t.TempDir())
	if err := os.MkdirAll(filepath.Join(root, ".fellowship"), 0o755); err != nil {
		t.Fatal(err)
	}

	d, err := openStore(root, []string{"hook", "gate-guard"})
	if err == nil {
		d.Close()
		t.Fatal("expected a no-store error")
	}
	for _, name := range []string{"gate-guard", "gate-submit", "gate-prereq", "file-track"} {
		if got := storeOpenExit(root, []string{"hook", name}, err); got != 2 {
			t.Errorf("%s with the store erased: exit %d, want 2 (block)", name, got)
		}
	}
	if got := storeOpenExit(root, []string{"hook", "worktree-guard"}, err); got != 0 {
		t.Errorf("worktree-guard with the store erased: exit %d, want 0 (fail open)", got)
	}
}

// A zero-byte store is a destroyed store, never a fresh one — and it must not
// be silently rebuilt by a hook that happens to open it.
func TestStoreOpen_EmptyStoreBlocksAndIsNotRebuilt(t *testing.T) {
	root := newMainRepo(t)
	t.Setenv("HOME", t.TempDir())
	store := filepath.Join(root, ".fellowship", "fellowship.db")
	if err := os.MkdirAll(filepath.Dir(store), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	d, err := openStore(root, []string{"hook", "gate-guard"})
	if err == nil {
		d.Close()
		t.Fatal("expected an error opening a zero-byte store")
	}
	if got := storeOpenExit(root, []string{"hook", "gate-guard"}, err); got != 2 {
		t.Errorf("gate-guard with a zero-byte store: exit %d, want 2 (block)", got)
	}
	if got := storeOpenExit(root, []string{"hook", "worktree-guard"}, err); got != 0 {
		t.Errorf("worktree-guard with a zero-byte store: exit %d, want 0 (fail open)", got)
	}
	info, statErr := os.Stat(store)
	if statErr != nil {
		t.Fatal(statErr)
	}
	if info.Size() != 0 {
		t.Errorf("a hook rebuilt the schema in a zero-byte store (now %d bytes)", info.Size())
	}

	// The store-creating commands are the ones allowed to rebuild it.
	rebuilt, err := openStore(root, []string{"init"})
	if err != nil {
		t.Fatalf("init must be able to rebuild an empty store: %v", err)
	}
	rebuilt.Close()
	if info, err := os.Stat(store); err != nil || info.Size() == 0 {
		t.Error("init should have written a schema into the empty store")
	}
}

// A repo that never had a fellowship still allows: no data directory, nothing
// to enforce.
func TestStoreOpen_PlainRepoStillAllows(t *testing.T) {
	root := newMainRepo(t)
	t.Setenv("HOME", t.TempDir())

	d, err := openStore(root, []string{"hook", "gate-guard"})
	if err == nil {
		d.Close()
		t.Fatal("expected a no-store error")
	}
	if got := storeOpenExit(root, []string{"hook", "gate-guard"}, err); got != 0 {
		t.Errorf("gate-guard in a plain repo: exit %d, want 0 (allow)", got)
	}
}

// A fresh clone of a repo that COMMITS .fellowship/config.json (the documented
// team-shared project config; the rest of the directory is git-ignored) has a
// data directory and no store, and no fellowship has ever run in it. Blocking
// there would refuse every Edit/Write/Bash in the repo until somebody ran
// `fellowship state init`.
func TestStoreOpen_CommittedProjectConfigIsNotAFellowship(t *testing.T) {
	root := newMainRepo(t)
	t.Setenv("HOME", t.TempDir())
	if err := os.MkdirAll(filepath.Join(root, ".fellowship"), 0o755); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(root, ".fellowship", "config.json")
	if err := os.WriteFile(config, []byte(`{"gates":{"autoApprove":[]}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	d, err := openStore(root, []string{"hook", "gate-guard"})
	if err == nil {
		d.Close()
		t.Fatal("expected a no-store error")
	}
	for _, name := range []string{"gate-guard", "gate-submit", "file-track"} {
		if got := storeOpenExit(root, []string{"hook", name}, err); got != 0 {
			t.Errorf("%s in a clone with only the project config: exit %d, want 0 (allow)", name, got)
		}
	}

	// Anything else in the data directory is a fellowship's leftovers, and the
	// missing store is then a destroyed store.
	if err := os.WriteFile(filepath.Join(root, ".fellowship", "checkpoint.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := storeOpenExit(root, []string{"hook", "gate-guard"}, err); got != 2 {
		t.Errorf("gate-guard with a fellowship's leftovers and no store: exit %d, want 2 (block)", got)
	}
}

// --- missing quest row ------------------------------------------------------

// A missing quest_state row is the bootstrap window — until the quest has done
// something. Then it is a deleted row, and deleting the row was another way to
// shake off a pending gate.
func TestRunHookWith_MissingQuestRow(t *testing.T) {
	root := newMainRepo(t)
	worktree := addWorktree(t, root, "quest-bootstrap-alpha")

	// Bootstrap: registered worktree, no row, no history.
	d := fellowshipWith(t, root, map[string]string{"quest-bootstrap-alpha": worktree})
	if got := runHookWith("gate-guard", bashInput("ls"), worktree, d); got != 0 {
		t.Errorf("bootstrap window: exit %d, want 0 (allow)", got)
	}

	// The same shape, but the quest has already logged an event.
	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		return events.Record(conn, events.Event{
			Timestamp: "2026-01-01T00:00:00Z",
			Quest:     "quest-bootstrap-alpha",
			Type:      events.GateSubmitted,
			Phase:     "Research",
			Detail:    "work happened",
		})
	}); err != nil {
		t.Fatal(err)
	}
	if got := runHookWith("gate-guard", bashInput("ls"), worktree, d); got != 2 {
		t.Errorf("deleted row on a quest with history: exit %d, want 2 (block)", got)
	}
	// Every gate hook agrees, not just the guard.
	if got := runHookWith("file-track", bashInput("ls"), worktree, d); got != 2 {
		t.Errorf("file-track: exit %d, want 2 (block)", got)
	}

	// A row that exists is unaffected by any of this.
	setQuestState(t, d, &state.State{QuestName: "quest-bootstrap-alpha", Phase: "Implement"})
	if got := runHookWith("gate-guard", bashInput("ls"), worktree, d); got != 0 {
		t.Errorf("restored row: exit %d, want 0", got)
	}
}
