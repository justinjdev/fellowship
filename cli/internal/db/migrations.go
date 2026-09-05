package db

import (
	"fmt"

	"zombiezen.com/go/sqlite/sqlitex"
)

// migration is one versioned schema change applied on top of the frozen
// version-1 baseSchema (schema.go). To ship a schema change:
//
//  1. Append a new migration{version, up} below, with version one higher
//     than the current last entry.
//  2. Make the fresh-install `schema` slice in schema.go end up with the
//     same objects your up func leaves behind, so a brand-new store and a
//     store upgraded through the ladder always match — see
//     TestFreshSchemaMatchesMigratedSchema.
//  3. Never edit an existing entry's up func or version once it has
//     shipped, even for a purely additive change (a new column, a new
//     index): a store that already recorded that version in PRAGMA
//     user_version will never run the step again, so an edit only affects
//     stores that haven't upgraded yet and silently diverges from ones
//     that already did. Ship a corrective follow-up migration instead.
//
// up runs inside the same transaction as every other pending step for that
// open (see applyMigrations), so a mid-ladder failure rolls the whole batch
// back rather than stranding a store between versions.
type migration struct {
	version int
	up      func(conn *Conn) error
}

// migrations is the ordered ladder ensureSchema applies on top of
// baseSchemaVersion. Versions must be strictly increasing — see
// TestMigrationsAreOrderedAndSequential.
var migrations = []migration{
	{
		version: 2,
		up: func(conn *Conn) error {
			// Two fellowship_quests rows could already share a worktree —
			// nothing stopped it before this index existed, and FindQuest
			// silently returned whichever row its scan happened to see
			// last. Dedupe deterministically (keep the newest row by
			// rowid) before creating the unique index, so upgrading an old
			// store can't fail on data that predates the constraint.
			if err := sqlitex.ExecuteTransient(conn, `
				DELETE FROM fellowship_quests
				WHERE worktree IS NOT NULL AND worktree != ''
				AND rowid NOT IN (
					SELECT MAX(rowid) FROM fellowship_quests
					WHERE worktree IS NOT NULL AND worktree != ''
					GROUP BY worktree
				)`, nil); err != nil {
				return fmt.Errorf("dedupe fellowship_quests.worktree: %w", err)
			}
			return sqlitex.ExecuteTransient(conn, questWorktreeUniqueIndexSQL, nil)
		},
	},
}

// migrationCalls counts how many times applyMigrations has actually run the
// ladder. Tests use it to assert that an already-current store takes the
// read-only fast path in ensureSchema, the same way applySchemaCalls does
// for a fresh install; nothing in production reads it.
var migrationCalls int

// latestSchemaVersion is the version ensureSchema and applySchema bring a
// store to: the frozen version-1 base plus every migration step's version.
func latestSchemaVersion() int {
	v := baseSchemaVersion
	for _, m := range migrations {
		if m.version > v {
			v = m.version
		}
	}
	return v
}

// applyMigrations runs every step whose version is above from, in order,
// inside one transaction, then bumps user_version to latest. Callers must
// already know from < latest.
func applyMigrations(conn *Conn, from, latest int) error {
	migrationCalls++
	endFn, err := sqlitex.ImmediateTransaction(conn)
	if err != nil {
		return fmt.Errorf("db: begin migration tx: %w", err)
	}
	fnErr := func() error {
		for _, m := range migrations {
			if m.version <= from {
				continue
			}
			if err := m.up(conn); err != nil {
				return fmt.Errorf("db: migrate to version %d: %w", m.version, err)
			}
		}
		return sqlitex.ExecuteTransient(conn, fmt.Sprintf("PRAGMA user_version = %d", latest), nil)
	}()
	endFn(&fnErr)
	return fnErr
}
