package db

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// newRepo creates an empty git repository and returns its root.
func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.Command("git", "init", "-q")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("git init unavailable: %v: %s", err, out)
	}
	return dir
}

// Every hook invocation opens the store. Re-running the schema DDL on each open
// would take a write lock and append to the WAL for what is a read-only
// decision, so an already-current store must skip applySchema entirely.
func TestOpenPath_SkipsSchemaOnAlreadyCurrentStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fellowship.db")

	d, err := OpenPath(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}

	before := applySchemaCalls
	d2, err := OpenPath(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d2.Close()
	if applySchemaCalls != before {
		t.Fatalf("applySchema re-ran on an already-current store (%d -> %d)", before, applySchemaCalls)
	}
}

// The same invariant observed from the outside: three consecutive opens of an
// initialized store leave the database file and its WAL byte-for-byte alone.
func TestOpenExisting_DoesNotGrowWAL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fellowship.db")
	d, err := OpenPath(path)
	if err != nil {
		t.Fatal(err)
	}
	d.Close()

	sizes := func() (int64, int64) {
		t.Helper()
		var dbSize, walSize int64
		if fi, err := os.Stat(path); err == nil {
			dbSize = fi.Size()
		}
		if fi, err := os.Stat(path + "-wal"); err == nil {
			walSize = fi.Size()
		}
		return dbSize, walSize
	}

	wantDB, wantWAL := sizes()
	for i := 0; i < 3; i++ {
		d, err := openExistingPath(path)
		if err != nil {
			t.Fatalf("open %d: %v", i, err)
		}
		d.Close()
		gotDB, gotWAL := sizes()
		if gotDB != wantDB || gotWAL != wantWAL {
			t.Fatalf("open %d changed the store: db %d->%d, wal %d->%d", i, wantDB, gotDB, wantWAL, gotWAL)
		}
	}
}

// OpenExisting must never bring a store into existence: an ordinary repo that
// has never run `fellowship init` stays untouched, and the caller can tell that
// case apart from a broken store.
func TestOpenExisting_NoStore(t *testing.T) {
	dir := newRepo(t)
	d, err := OpenExisting(dir)
	if err == nil {
		d.Close()
		t.Fatal("expected an error when no store exists")
	}
	if !errors.Is(err, ErrNoStore) {
		t.Fatalf("expected ErrNoStore, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, ".fellowship")); statErr == nil {
		t.Fatal("OpenExisting created a .fellowship directory")
	}
}

// A store that exists but is not a database must surface as a real error (which
// the CLI turns into a fail-closed block for gate hooks), not as ErrNoStore.
func TestOpenExisting_CorruptStore(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".fellowship"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, ".fellowship", "fellowship.db")
	if err := os.WriteFile(path, []byte("this is not a sqlite database"), 0o644); err != nil {
		t.Fatal(err)
	}

	d, err := openExistingPath(path)
	if err == nil {
		d.Close()
		t.Fatal("expected an error for a corrupt store")
	}
	if errors.Is(err, ErrNoStore) {
		t.Fatalf("a corrupt store must not read as a missing store: %v", err)
	}
}
