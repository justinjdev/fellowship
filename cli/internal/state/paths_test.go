package state_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/justinjdev/fellowship/cli/internal/db"
	"github.com/justinjdev/fellowship/cli/internal/state"
	"zombiezen.com/go/sqlite/sqlitex"
)

// addQuestRow writes a fellowship_quests row with the worktree column exactly
// as given, bypassing the write-side canonicalization so the read side can be
// tested against rows an older version (or a hand edit) might have left.
func addQuestRow(t *testing.T, d *db.DB, name, worktree string) {
	t.Helper()
	err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		return sqlitex.Execute(conn,
			`INSERT INTO fellowship_quests (name, task_description, worktree, branch, task_id, status)
			 VALUES (:name, 'task', :wt, '', '', 'active')`,
			&sqlitex.ExecOptions{Named: map[string]any{":name": name, ":wt": worktree}})
	})
	if err != nil {
		t.Fatal(err)
	}
}

func findQuest(t *testing.T, d *db.DB, root string) string {
	t.Helper()
	var name string
	err := d.WithConn(context.Background(), func(conn *db.Conn) error {
		var err error
		name, err = state.FindQuest(conn, root)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return name
}

// A quest registered with a relative --worktree path must still be found by a
// hook, which always looks up the absolute git top-level.
func TestFindQuest_RelativeStoredPath(t *testing.T) {
	base := t.TempDir()
	wt := filepath.Join(base, "wt-1")
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(base)

	d := db.OpenTest(t)
	addQuestRow(t, d, "quest-rel", "wt-1")

	if got := findQuest(t, d, state.CanonicalWorktree(wt)); got != "quest-rel" {
		t.Errorf("relative stored path did not match absolute lookup: got %q", got)
	}
}

// The same, the other way around: the lookup path is relative while the stored
// path is absolute.
func TestFindQuest_RelativeLookupPath(t *testing.T) {
	base := t.TempDir()
	wt := filepath.Join(base, "wt-2")
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(base)

	d := db.OpenTest(t)
	addQuestRow(t, d, "quest-abs", state.CanonicalWorktree(wt))

	if got := findQuest(t, d, "wt-2"); got != "quest-abs" {
		t.Errorf("relative lookup did not match absolute stored path: got %q", got)
	}
}

// A worktree reached through a symlink (the shape macOS /tmp -> /private/tmp
// produces) must resolve to the same quest as the real path.
func TestFindQuest_SymlinkedPath(t *testing.T) {
	base := t.TempDir()
	real := filepath.Join(base, "real-wt")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "link-wt")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	d := db.OpenTest(t)
	addQuestRow(t, d, "quest-link", link)

	if got := findQuest(t, d, state.CanonicalWorktree(real)); got != "quest-link" {
		t.Errorf("symlinked stored path did not match resolved lookup: got %q", got)
	}
	if got := findQuest(t, d, link); got != "quest-link" {
		t.Errorf("symlinked lookup did not match: got %q", got)
	}
}

func TestFindQuest_NoMatch(t *testing.T) {
	base := t.TempDir()
	d := db.OpenTest(t)
	addQuestRow(t, d, "quest-other", filepath.Join(base, "somewhere-else"))

	if got := findQuest(t, d, filepath.Join(base, "not-a-quest")); got != "" {
		t.Errorf("unrelated path should not match a quest, got %q", got)
	}
	if got := findQuest(t, d, ""); got != "" {
		t.Errorf("empty path should not match a quest, got %q", got)
	}
}

func TestCanonicalWorktree_Empty(t *testing.T) {
	if got := state.CanonicalWorktree(""); got != "" {
		t.Errorf("empty path should stay empty, got %q", got)
	}
}

// A path that does not exist yet still canonicalizes to an absolute path, so a
// quest can be registered before its worktree is created.
func TestCanonicalWorktree_NonexistentPath(t *testing.T) {
	base := t.TempDir()
	t.Chdir(base)
	got := state.CanonicalWorktree("not-created-yet/deeper")
	want := filepath.Join(state.CanonicalWorktree(base), "not-created-yet", "deeper")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
