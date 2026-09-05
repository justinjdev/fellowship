package db

import (
	"path/filepath"
	"testing"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

// A store written before the lead moved into SQLite has no lead table. Opening
// it runs migration 4, which creates the table without inventing a lead — the
// legacy marker file stays authoritative for that fellowship until its next
// `state init`.
func TestEnsureSchema_UpgradesV3AddsLeadTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fellowship.db")

	d, err := OpenPath(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.WithConn(t.Context(), func(conn *Conn) error {
		if err := sqlitex.ExecuteTransient(conn, "DROP TABLE IF EXISTS lead", nil); err != nil {
			return err
		}
		return sqlitex.ExecuteTransient(conn, "PRAGMA user_version = 3", nil)
	}); err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}

	d2, err := OpenPath(path)
	if err != nil {
		t.Fatalf("reopen after seeding v3: %v", err)
	}
	defer d2.Close()

	if err := d2.WithConn(t.Context(), func(conn *Conn) error {
		rows := 0
		if err := sqlitex.ExecuteTransient(conn, `SELECT count(*) FROM lead`, &sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				rows = stmt.ColumnInt(0)
				return nil
			},
		}); err != nil {
			return err
		}
		if rows != 0 {
			t.Errorf("lead table has %d rows after the upgrade, want 0", rows)
		}
		v, err := userVersion(conn)
		if err != nil {
			return err
		}
		if v != latestSchemaVersion() {
			t.Errorf("user_version = %d, want %d", v, latestSchemaVersion())
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
