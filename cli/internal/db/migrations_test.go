package db

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

// TestMigrationsAreOrderedAndSequential guards the ladder's own invariant:
// entries in migrations must appear in strictly increasing version order.
// applyMigrations relies on that order to know it has covered every step
// between a store's current version and latest.
func TestMigrationsAreOrderedAndSequential(t *testing.T) {
	prev := baseSchemaVersion
	for i, m := range migrations {
		if m.version <= prev {
			t.Fatalf("migrations[%d] has version %d, which is not greater than the previous version %d", i, m.version, prev)
		}
		prev = m.version
	}
}

// TestOpenMemory_FreshInstallLandsOnLatest asserts a brand-new store (the
// path every OpenMemory / OpenPath-with-create call takes) ends up stamped
// at latestSchemaVersion, not the frozen base version.
func TestOpenMemory_FreshInstallLandsOnLatest(t *testing.T) {
	d := OpenTest(t)
	if err := d.WithConn(t.Context(), func(conn *Conn) error {
		v, err := userVersion(conn)
		if err != nil {
			return err
		}
		if v != latestSchemaVersion() {
			t.Fatalf("fresh store user_version = %d, want latestSchemaVersion() = %d", v, latestSchemaVersion())
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// TestEnsureSchema_NewerVersionErrors asserts a store stamped above what
// this binary knows about refuses to be touched, with a message the CLI can
// surface as-is rather than guessing at a downgrade.
func TestEnsureSchema_NewerVersionErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fellowship.db")
	d, err := OpenPath(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.WithConn(t.Context(), func(conn *Conn) error {
		return sqlitex.ExecuteTransient(conn, "PRAGMA user_version = 99", nil)
	}); err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}

	d2, err := OpenPath(path)
	if err == nil {
		d2.Close()
		t.Fatal("expected an error opening a store from a newer schema version")
	}
	if !strings.Contains(err.Error(), "newer fellowship") || !strings.Contains(err.Error(), "upgrade the binary") {
		t.Fatalf("error should explain the store is from a newer fellowship and say to upgrade the binary, got: %v", err)
	}
	if errors.Is(err, ErrNoStore) {
		t.Fatalf("a newer-version store must not read as ErrNoStore: %v", err)
	}
}

// TestEnsureSchema_UpgradesV1DedupesWorktrees seeds a store at version 1
// with two fellowship_quests rows already sharing a worktree — possible
// before the unique index existed — and verifies opening it upgrades to
// latestSchemaVersion, dedupes deterministically (keeping the newest row by
// rowid), and does not re-run the ladder on a subsequent open.
func TestEnsureSchema_UpgradesV1DedupesWorktrees(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fellowship.db")

	d, err := OpenPath(path)
	if err != nil {
		t.Fatal(err)
	}

	// Roll the store back to a pre-migration v1 shape: drop the unique
	// index migration 2 adds, insert two quests sharing a worktree (which
	// the index would otherwise reject), and stamp user_version back to 1.
	if err := d.WithConn(t.Context(), func(conn *Conn) error {
		if err := sqlitex.ExecuteTransient(conn, "DROP INDEX IF EXISTS idx_fellowship_quests_worktree", nil); err != nil {
			return err
		}
		for _, name := range []string{"quest-first", "quest-second"} {
			if err := sqlitex.Execute(conn,
				`INSERT INTO fellowship_quests (name, worktree) VALUES (:name, :wt)`,
				&sqlitex.ExecOptions{Named: map[string]any{":name": name, ":wt": "/shared/worktree"}}); err != nil {
				return err
			}
		}
		return sqlitex.ExecuteTransient(conn, "PRAGMA user_version = 1", nil)
	}); err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}

	before := migrationCalls
	d2, err := OpenPath(path)
	if err != nil {
		t.Fatalf("reopen after seeding v1 with duplicate worktrees: %v", err)
	}
	if migrationCalls != before+1 {
		t.Fatalf("expected the migration ladder to run once, ran %d time(s)", migrationCalls-before)
	}

	var keptName string
	var rowCount int
	if err := d2.WithConn(t.Context(), func(conn *Conn) error {
		v, err := userVersion(conn)
		if err != nil {
			return err
		}
		if v != latestSchemaVersion() {
			t.Fatalf("user_version after upgrade = %d, want %d", v, latestSchemaVersion())
		}
		return sqlitex.Execute(conn,
			`SELECT COUNT(*), MAX(name) FROM fellowship_quests WHERE worktree = '/shared/worktree'`,
			&sqlitex.ExecOptions{
				ResultFunc: func(stmt *sqlite.Stmt) error {
					rowCount = stmt.ColumnInt(0)
					keptName = stmt.ColumnText(1)
					return nil
				},
			})
	}); err != nil {
		t.Fatal(err)
	}
	if rowCount != 1 {
		t.Fatalf("expected exactly 1 row per worktree after dedupe, got %d", rowCount)
	}
	if keptName != "quest-second" {
		t.Fatalf("dedupe should keep the newest row by rowid (quest-second), kept %q instead", keptName)
	}

	// A second insert with the same worktree must now be rejected by the
	// unique index the migration created.
	if err := d2.WithConn(t.Context(), func(conn *Conn) error {
		return sqlitex.Execute(conn,
			`INSERT INTO fellowship_quests (name, worktree) VALUES ('quest-third', '/shared/worktree')`, nil)
	}); err == nil {
		t.Fatal("expected the unique index to reject a duplicate worktree after migration")
	}

	// Opening again must not re-run the ladder: the store is already current.
	if err := d2.Close(); err != nil {
		t.Fatal(err)
	}
	before = migrationCalls
	d3, err := OpenPath(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d3.Close()
	if migrationCalls != before {
		t.Fatalf("migration ladder re-ran on an already-current store (%d -> %d)", before, migrationCalls)
	}
}

// TestFreshSchemaMatchesMigratedSchema asserts a brand-new store and a
// store that started at version 1 and was upgraded through the ladder end
// up with identical schema objects — the guarantee that lets version 2
// fresh == version 1 upgraded, so nothing downstream needs to special-case
// which path a store took to get there.
func TestFreshSchemaMatchesMigratedSchema(t *testing.T) {
	fresh, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()

	pool, err := sqlitex.NewPool("file::memory:?mode=memory", sqlitex.PoolOptions{
		PoolSize: 1,
		Flags:    sqlite.OpenReadWrite | sqlite.OpenCreate | sqlite.OpenMemory,
	})
	if err != nil {
		t.Fatal(err)
	}
	migrated := &DB{pool: pool, path: ":memory:"}
	defer migrated.Close()

	if err := migrated.WithConn(t.Context(), func(conn *Conn) error {
		// Apply the frozen version-1 base schema directly (not `schema`,
		// which already includes migration 2's index) and stamp it at
		// version 1, then let ensureSchema do exactly what a real upgrade
		// does.
		for _, stmt := range baseSchema {
			if err := sqlitex.ExecuteTransient(conn, stmt, nil); err != nil {
				return err
			}
		}
		if err := sqlitex.ExecuteTransient(conn, "PRAGMA user_version = 1", nil); err != nil {
			return err
		}
		return ensureSchema(conn)
	}); err != nil {
		t.Fatal(err)
	}

	freshObjects, err := schemaObjects(t, fresh)
	if err != nil {
		t.Fatal(err)
	}
	migratedObjects, err := schemaObjects(t, migrated)
	if err != nil {
		t.Fatal(err)
	}

	if len(freshObjects) != len(migratedObjects) {
		t.Fatalf("fresh install has %d schema objects, migrated store has %d", len(freshObjects), len(migratedObjects))
	}
	for name, sql := range freshObjects {
		got, ok := migratedObjects[name]
		if !ok {
			t.Errorf("migrated store is missing schema object %q present in a fresh install", name)
			continue
		}
		if got != sql {
			t.Errorf("schema object %q differs between fresh install and migrated store:\nfresh:    %s\nmigrated: %s", name, sql, got)
		}
	}
	for name := range migratedObjects {
		if _, ok := freshObjects[name]; !ok {
			t.Errorf("migrated store has schema object %q not present in a fresh install", name)
		}
	}
}

// schemaObjects reads sqlite_master into a map of object name -> normalized
// SQL, skipping SQLite's own internal objects (e.g. autoindexes with no
// name of their own).
func schemaObjects(t *testing.T, d *DB) (map[string]string, error) {
	t.Helper()
	objects := map[string]string{}
	err := d.WithConn(t.Context(), func(conn *Conn) error {
		return sqlitex.Execute(conn,
			`SELECT name, sql FROM sqlite_master WHERE sql IS NOT NULL ORDER BY name`,
			&sqlitex.ExecOptions{
				ResultFunc: func(stmt *sqlite.Stmt) error {
					name := stmt.ColumnText(0)
					sql := normalizeSQL(stmt.ColumnText(1))
					objects[name] = sql
					return nil
				},
			})
	})
	return objects, err
}

// normalizeSQL collapses whitespace so that DDL differing only in
// indentation or line breaks (as CREATE TABLE and CREATE TABLE IF NOT
// EXISTS forms can, once SQLite echoes them back) still compares equal.
func normalizeSQL(sql string) string {
	return strings.Join(strings.Fields(sql), " ")
}
