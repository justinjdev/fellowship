package db

import (
	"fmt"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

// baseSchemaVersion is the schema version that predates the migration
// ladder in migrations.go — the shape every store was created with before
// versioned migrations existed. ensureSchema treats a fresh file (PRAGMA
// user_version reading 0) as "apply schema and stamp it at
// latestSchemaVersion"; baseSchemaVersion only ever appears already stamped
// on a store created by an older binary.
const baseSchemaVersion = 1

// questWorktreeUniqueIndexSQL enforces one fellowship_quests row per
// worktree. Two quests could otherwise share a worktree (nothing stopped
// it), and FindQuest would silently return whichever row its table scan
// happened to see last. This is migration version 2 in migrations.go;
// declared once here so the fresh-install schema below and the upgrade
// migration apply byte-identical DDL — see
// TestFreshSchemaMatchesMigratedSchema in migrations_test.go.
const questWorktreeUniqueIndexSQL = `CREATE UNIQUE INDEX IF NOT EXISTS idx_fellowship_quests_worktree ON fellowship_quests(worktree) WHERE worktree IS NOT NULL AND worktree != ''`

// baseSchema contains every CREATE TABLE, INDEX, and TRIGGER statement from
// the original version-1 schema. Do not edit a statement here to ship a
// schema change — add a migration step in migrations.go instead (see
// "Schema changes" in CONTRIBUTING.md); this slice must keep producing
// exactly what it always has, since applySchema also replays it verbatim
// for every brand-new store on the way to the current version.
var baseSchema = []string{
	// Quest state (replaces quest-state.json)
	`CREATE TABLE IF NOT EXISTS quest_state (
		quest_name       TEXT PRIMARY KEY,
		task_id          TEXT,
		team_name        TEXT,
		phase            TEXT NOT NULL DEFAULT 'Onboard',
		gate_pending     INTEGER NOT NULL DEFAULT 0,
		gate_id          TEXT,
		lembas_completed INTEGER NOT NULL DEFAULT 0,
		metadata_updated INTEGER NOT NULL DEFAULT 0,
		held             INTEGER NOT NULL DEFAULT 0,
		held_reason      TEXT,
		auto_approve     TEXT,
		created_at       TEXT NOT NULL,
		updated_at       TEXT NOT NULL
	)`,

	// Phase history (replaces quest-tome.json phases)
	`CREATE TABLE IF NOT EXISTS quest_phases (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		quest_name   TEXT NOT NULL REFERENCES quest_state(quest_name),
		phase        TEXT NOT NULL,
		completed_at TEXT NOT NULL,
		duration_s   INTEGER
	)`,

	// Gate history (replaces quest-tome.json gates)
	`CREATE TABLE IF NOT EXISTS quest_gates (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		quest_name TEXT NOT NULL REFERENCES quest_state(quest_name),
		phase      TEXT NOT NULL,
		action     TEXT NOT NULL,
		timestamp  TEXT NOT NULL,
		reason     TEXT
	)`,

	// Files touched per quest (replaces quest-tome.json files)
	`CREATE TABLE IF NOT EXISTS quest_files (
		quest_name TEXT NOT NULL REFERENCES quest_state(quest_name),
		file_path  TEXT NOT NULL,
		PRIMARY KEY (quest_name, file_path)
	)`,

	// Errands (replaces quest-errands.json)
	`CREATE TABLE IF NOT EXISTS errands (
		id          TEXT NOT NULL,
		quest_name  TEXT NOT NULL REFERENCES quest_state(quest_name),
		description TEXT NOT NULL,
		status      TEXT NOT NULL DEFAULT 'pending',
		phase       TEXT,
		created_at  TEXT NOT NULL,
		updated_at  TEXT NOT NULL,
		PRIMARY KEY (quest_name, id)
	)`,

	`CREATE TABLE IF NOT EXISTS errand_deps (
		quest_name TEXT NOT NULL,
		errand_id  TEXT NOT NULL,
		depends_on TEXT NOT NULL,
		PRIMARY KEY (quest_name, errand_id, depends_on),
		FOREIGN KEY (quest_name, errand_id) REFERENCES errands(quest_name, id),
		FOREIGN KEY (quest_name, depends_on) REFERENCES errands(quest_name, id)
	)`,

	// Herald event log (replaces quest-herald.jsonl)
	// No FK to quest_state — events logged before quest exists and survive deletion.
	// table: herald -> package events (cli/internal/events); table name unchanged, no schema change.
	`CREATE TABLE IF NOT EXISTS herald (
		id        INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp TEXT NOT NULL,
		quest     TEXT NOT NULL,
		type      TEXT NOT NULL,
		phase     TEXT,
		detail    TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS idx_herald_quest ON herald(quest, type)`,
	`CREATE INDEX IF NOT EXISTS idx_herald_ts ON herald(timestamp)`,

	// Fellowship orchestration (replaces fellowship-state.json)
	`CREATE TABLE IF NOT EXISTS fellowship (
		id          INTEGER PRIMARY KEY CHECK (id = 1),
		version     TEXT NOT NULL,
		name        TEXT NOT NULL,
		main_repo   TEXT NOT NULL,
		base_branch TEXT NOT NULL DEFAULT 'main',
		created_at  TEXT NOT NULL
	)`,

	`CREATE TABLE IF NOT EXISTS fellowship_quests (
		name             TEXT PRIMARY KEY,
		task_description TEXT,
		worktree         TEXT,
		branch           TEXT,
		task_id          TEXT,
		status           TEXT DEFAULT 'active',
		respawns         INTEGER NOT NULL DEFAULT 0
	)`,

	`CREATE TABLE IF NOT EXISTS fellowship_scouts (
		name     TEXT PRIMARY KEY,
		question TEXT,
		task_id  TEXT
	)`,

	`CREATE TABLE IF NOT EXISTS companies (
		name TEXT PRIMARY KEY
	)`,

	`CREATE TABLE IF NOT EXISTS company_members (
		company_name TEXT NOT NULL REFERENCES companies(name),
		member_name  TEXT NOT NULL,
		member_type  TEXT NOT NULL,
		PRIMARY KEY (company_name, member_name)
	)`,

	// Bulletin (replaces bulletin.jsonl)
	`CREATE TABLE IF NOT EXISTS bulletin (
		id        INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp TEXT NOT NULL,
		quest     TEXT NOT NULL,
		topic     TEXT NOT NULL,
		discovery TEXT NOT NULL
	)`,

	`CREATE TABLE IF NOT EXISTS bulletin_files (
		bulletin_id INTEGER NOT NULL REFERENCES bulletin(id),
		file_path   TEXT NOT NULL,
		PRIMARY KEY (bulletin_id, file_path)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_bulletin_topic ON bulletin(topic)`,
	`CREATE INDEX IF NOT EXISTS idx_bulletin_files ON bulletin_files(file_path)`,

	// Autopsies (replaces autopsies/*.json)
	`CREATE TABLE IF NOT EXISTS autopsies (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp   TEXT NOT NULL,
		quest       TEXT NOT NULL,
		task        TEXT,
		phase       TEXT,
		trigger_type TEXT NOT NULL,
		what_failed TEXT NOT NULL,
		resolution  TEXT,
		expires_at  TEXT NOT NULL
	)`,

	`CREATE TABLE IF NOT EXISTS autopsy_files (
		autopsy_id INTEGER NOT NULL REFERENCES autopsies(id),
		file_path  TEXT NOT NULL,
		PRIMARY KEY (autopsy_id, file_path)
	)`,

	`CREATE TABLE IF NOT EXISTS autopsy_modules (
		autopsy_id INTEGER NOT NULL REFERENCES autopsies(id),
		module     TEXT NOT NULL,
		PRIMARY KEY (autopsy_id, module)
	)`,

	`CREATE TABLE IF NOT EXISTS autopsy_tags (
		autopsy_id INTEGER NOT NULL REFERENCES autopsies(id),
		tag        TEXT NOT NULL,
		PRIMARY KEY (autopsy_id, tag)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_autopsy_files ON autopsy_files(file_path)`,
	`CREATE INDEX IF NOT EXISTS idx_autopsy_expires ON autopsies(expires_at)`,

	// Provenance tracking
	`CREATE TABLE IF NOT EXISTS state_changelog (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
		table_name TEXT NOT NULL,
		operation  TEXT NOT NULL,
		quest_name TEXT,
		old_value  TEXT,
		new_value  TEXT
	)`,

	`CREATE TRIGGER IF NOT EXISTS quest_state_insert AFTER INSERT ON quest_state
	BEGIN
		INSERT INTO state_changelog(table_name, operation, quest_name, new_value)
		VALUES('quest_state', 'INSERT', NEW.quest_name,
			json_object('phase', NEW.phase, 'gate_pending', NEW.gate_pending, 'held', NEW.held,
				'held_reason', NEW.held_reason, 'gate_id', NEW.gate_id,
				'task_id', NEW.task_id, 'team_name', NEW.team_name, 'auto_approve', NEW.auto_approve));
	END`,

	`CREATE TRIGGER IF NOT EXISTS quest_state_update AFTER UPDATE ON quest_state
	BEGIN
		INSERT INTO state_changelog(table_name, operation, quest_name, old_value, new_value)
		VALUES('quest_state', 'UPDATE', NEW.quest_name,
			json_object('phase', OLD.phase, 'gate_pending', OLD.gate_pending, 'held', OLD.held,
				'held_reason', OLD.held_reason, 'gate_id', OLD.gate_id,
				'task_id', OLD.task_id, 'team_name', OLD.team_name, 'auto_approve', OLD.auto_approve,
				'lembas_completed', OLD.lembas_completed, 'metadata_updated', OLD.metadata_updated),
			json_object('phase', NEW.phase, 'gate_pending', NEW.gate_pending, 'held', NEW.held,
				'held_reason', NEW.held_reason, 'gate_id', NEW.gate_id,
				'task_id', NEW.task_id, 'team_name', NEW.team_name, 'auto_approve', NEW.auto_approve,
				'lembas_completed', NEW.lembas_completed, 'metadata_updated', NEW.metadata_updated));
	END`,
}

// schema is the full DDL for a brand-new store: the frozen version-1
// baseSchema plus the additive DDL from every migration step in
// migrations.go, so a fresh install and a store upgraded through the ladder
// always converge on identical schema objects.
var schema = append(append([]string{}, baseSchema...), questWorktreeUniqueIndexSQL)

// applySchemaCalls counts how many times applySchema has actually run DDL.
// Tests use it to assert that opening an already-current store takes the
// read-only path; nothing in production reads it.
var applySchemaCalls int

// ensureSchema brings the store up to latestSchemaVersion, doing nothing at
// all when it is already there. The check matters for enforcement: every
// hook opens the store, and re-running DDL on each open would append to the
// WAL and take a write lock on what should be a read-only decision.
//
// A fresh file (user_version 0) gets the current schema applied directly.
// A store below the latest version runs each pending migration step in
// migrations.go, in order, inside one transaction. A store above the
// latest version was created by a newer fellowship binary than this one —
// that's not safe to touch, so it errors instead of guessing.
func ensureSchema(conn *Conn) error {
	v, err := userVersion(conn)
	if err != nil {
		return err
	}
	latest := latestSchemaVersion()
	if v == latest {
		return nil
	}
	if v > latest {
		return fmt.Errorf("db: database is from a newer fellowship (schema version %d, this binary supports up to %d); upgrade the binary", v, latest)
	}
	if err := sqlitex.ExecuteTransient(conn, "PRAGMA journal_mode = WAL", nil); err != nil {
		return err
	}
	if v == 0 {
		return applySchema(conn)
	}
	return applyMigrations(conn, v, latest)
}

// userVersion reads PRAGMA user_version, the stamp applySchema leaves behind.
func userVersion(conn *Conn) (int, error) {
	v := -1
	err := sqlitex.ExecuteTransient(conn, "PRAGMA user_version", &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			v = stmt.ColumnInt(0)
			return nil
		},
	})
	if err != nil {
		return 0, fmt.Errorf("db: read user_version: %w", err)
	}
	return v, nil
}

// applySchema creates all tables, indexes, and triggers for a brand-new
// store and stamps it at latestSchemaVersion. Uses IF NOT EXISTS so it is
// idempotent.
func applySchema(conn *Conn) error {
	applySchemaCalls++
	for _, stmt := range schema {
		if err := sqlitex.ExecuteTransient(conn, stmt, nil); err != nil {
			return fmt.Errorf("db: schema: %w\nStatement: %.80s", err, stmt)
		}
	}

	// Set schema version.
	if err := sqlitex.ExecuteTransient(conn, fmt.Sprintf("PRAGMA user_version = %d", latestSchemaVersion()), nil); err != nil {
		return fmt.Errorf("db: set user_version: %w", err)
	}
	return nil
}
