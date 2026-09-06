package db

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"zombiezen.com/go/sqlite/sqlitex"
)

// A hook is a decision, not an upgrade path: it must never run the schema
// ladder, and the two hooks that only read must not even hold a writable
// connection.
func TestOpenForHook_RefusesToMigrate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fellowship.db")
	d, err := OpenPath(path)
	if err != nil {
		t.Fatal(err)
	}
	// Wind the store back to a version this binary would want to upgrade.
	if err := d.WithConn(t.Context(), func(conn *Conn) error {
		return sqlitex.ExecuteTransient(conn, "PRAGMA user_version = 1", nil)
	}); err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}

	for _, readOnly := range []bool{false, true} {
		hooked, err := openForHookPath(path, readOnly)
		if err == nil {
			hooked.Close()
			t.Fatalf("readOnly=%v: expected an out-of-date error", readOnly)
		}
		if !errors.Is(err, ErrSchemaOutOfDate) {
			t.Errorf("readOnly=%v: error = %v, want ErrSchemaOutOfDate", readOnly, err)
		}
	}

	// The store was left exactly as it was — no migration, no WAL churn.
	reopened, err := OpenPath(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if err := reopened.WithConn(t.Context(), func(conn *Conn) error {
		v, err := userVersion(conn)
		if err != nil {
			return err
		}
		if v != latestSchemaVersion() {
			t.Errorf("user_version = %d, want %d after a real open upgraded it", v, latestSchemaVersion())
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// The read-only connection really is read-only.
func TestOpenForHook_ReadOnlyConnectionCannotWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fellowship.db")
	d, err := OpenPath(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}

	hooked, err := openForHookPath(path, true)
	if err != nil {
		t.Fatal(err)
	}
	defer hooked.Close()

	writeErr := hooked.WithConn(context.Background(), func(conn *Conn) error {
		return sqlitex.ExecuteTransient(conn,
			`INSERT INTO quest_state (quest_name, phase, created_at, updated_at) VALUES ('q','Research','a','b')`, nil)
	})
	if writeErr == nil {
		t.Error("a read-only hook connection wrote to the store")
	}

	// Reading is exactly what it is for.
	if err := hooked.WithConn(context.Background(), func(conn *Conn) error {
		return sqlitex.ExecuteTransient(conn, "SELECT count(*) FROM quest_state", nil)
	}); err != nil {
		t.Errorf("read through the read-only connection: %v", err)
	}
}

// A zero-byte store is never opened, let alone rebuilt, by a hook.
func TestOpenForHook_EmptyStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fellowship.db")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := openForHookPath(path, true)
	if err == nil {
		d.Close()
		t.Fatal("expected an error for a zero-byte store")
	}
	if !errors.Is(err, ErrEmptyStore) {
		t.Errorf("error = %v, want ErrEmptyStore", err)
	}
	if info, err := os.Stat(path); err != nil || info.Size() != 0 {
		t.Error("the hook open rebuilt a zero-byte store")
	}
}
