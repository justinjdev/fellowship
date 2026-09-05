package db

import (
	"fmt"
	"strings"

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
	{
		version: 3,
		up: func(conn *Conn) error {
			// The quest lifecycle collapsed from seven phases to four:
			// Research -> Plan -> Implement -> Review. Stores written by an
			// older binary still name the retired phases, in the live
			// quest_state.phase and in the quest_phases / quest_gates
			// history, and every phase lookup (NextPhase, IsValidPhase,
			// the group phase rank) reads an unknown name as "not a
			// phase". Rewrite them in place so an in-flight quest keeps
			// advancing and its history keeps ranking.
			//
			// This is a data migration only. The frozen v1 DDL gives
			// quest_state.phase a default naming a retired phase, but no
			// writer relies on it -- every insert supplies a phase -- so
			// rebuilding the table to change a dead default would cost
			// more than it is worth.
			for _, table := range []string{"quest_state", "quest_phases", "quest_gates"} {
				if err := sqlitex.ExecuteTransient(conn, remapPhaseSQL(table), nil); err != nil {
					return fmt.Errorf("remap retired phases in %s: %w", table, err)
				}
			}
			// gates.autoApprove is stored per quest as a JSON array naming
			// the phase each auto-approved gate leaves. Retired names are
			// remapped the same way, except those that now map onto the
			// terminal phase: no gate leaves Review, so such an entry is
			// dropped rather than turned into one that auto-approves a
			// gate that cannot exist.
			return sqlitex.ExecuteTransient(conn, remapAutoApproveSQL(), nil)
		},
	},
}

// retiredPhases maps each phase name removed by the four-phase lifecycle
// onto its replacement, in the order they appeared in the old seven-phase
// order. Onboard's work became the first step of Research; Adversarial and
// Complete became the first and last steps of Review. This is the single
// source of truth for the remap: migration 3 rewrites stored phases through
// it, and MigrateJSON runs pre-2.0 JSON stores through it on the way in, so
// a legacy import cannot smuggle a retired name past the ladder.
var retiredPhases = []struct{ from, to string }{
	{"Onboard", "Research"},
	{"Adversarial", terminalPhase},
	{"Complete", terminalPhase},
}

// terminalPhase is the last phase of the lifecycle, the one no gate leaves.
// It duplicates state.TerminalPhase rather than importing it: the state
// package's tests reach into db, and this package stays clear of that edge.
const terminalPhase = "Review"

// RemapRetiredPhase returns the current name for a phase, passing through
// any name that is already current (or that was never a phase at all).
func RemapRetiredPhase(phase string) string {
	for _, r := range retiredPhases {
		if r.from == phase {
			return r.to
		}
	}
	return phase
}

// remapPhaseSQL builds the UPDATE that rewrites retired phase names in a
// table's phase column.
func remapPhaseSQL(table string) string {
	var cases, names strings.Builder
	for i, r := range retiredPhases {
		fmt.Fprintf(&cases, " WHEN '%s' THEN '%s'", r.from, r.to)
		if i > 0 {
			names.WriteString(", ")
		}
		fmt.Fprintf(&names, "'%s'", r.from)
	}
	return fmt.Sprintf(
		`UPDATE %s SET phase = CASE phase%s ELSE phase END WHERE phase IN (%s)`,
		table, cases.String(), names.String())
}

// remapAutoApproveSQL builds the UPDATE that rewrites the per-quest
// gates.autoApprove JSON array, dropping entries that would name the
// terminal phase (nothing leaves it, so no such gate exists).
func remapAutoApproveSQL() string {
	var cases strings.Builder
	// Entries to drop: retired names that now mean the terminal phase, plus
	// the terminal phase itself — a store written before Review became
	// terminal could legitimately name it, and no gate leaves it now.
	drop := []string{terminalPhase}
	for _, r := range retiredPhases {
		if r.to == terminalPhase {
			drop = append(drop, r.from)
			continue
		}
		fmt.Fprintf(&cases, " WHEN '%s' THEN '%s'", r.from, r.to)
	}
	// Rows worth touching at all: anything naming a dropped or renamed entry.
	touched := append([]string{}, drop...)
	for _, r := range retiredPhases {
		if r.to != terminalPhase {
			touched = append(touched, r.from)
		}
	}
	var dropped, names strings.Builder
	dropped.WriteString(quoteList(drop))
	names.WriteString(quoteList(touched))
	return fmt.Sprintf(`
		UPDATE quest_state
		SET auto_approve = (
			SELECT json_group_array(g) FROM (
				SELECT DISTINCT CASE json_each.value%s ELSE json_each.value END AS g
				FROM json_each(quest_state.auto_approve)
				WHERE json_each.value NOT IN (%s)
			)
		)
		WHERE auto_approve IS NOT NULL AND auto_approve != ''
		  AND json_valid(auto_approve)
		  AND EXISTS (
			SELECT 1 FROM json_each(quest_state.auto_approve)
			WHERE json_each.value IN (%s)
		  )`, cases.String(), dropped.String(), names.String())
}

// quoteList renders names as a SQL-quoted, comma-separated list. The values
// are fixed phase identifiers from this file, never user input.
func quoteList(names []string) string {
	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = "'" + n + "'"
	}
	return strings.Join(quoted, ", ")
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
