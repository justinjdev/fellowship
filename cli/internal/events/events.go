package events

import (
	"fmt"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"

	"github.com/justinjdev/fellowship/cli/internal/db"
)

// EventType represents the type of a quest event.
type EventType string

const (
	GateSubmitted   EventType = "gate_submitted"
	GateApproved    EventType = "gate_approved"
	GateRejected    EventType = "gate_rejected"
	PhaseTransition EventType = "phase_transition"
	LembasCompleted EventType = "lembas_completed"
	MetadataUpdated EventType = "metadata_updated"
	QuestHeld       EventType = "quest_held"
	QuestUnheld     EventType = "quest_unheld"
	QuestCompleted  EventType = "quest_completed"

	// Palantir alert types. The palantir monitor records its alerts as events
	// so retrospectives can read them back with the same CLI as gate history.
	PalantirStuck    EventType = "palantir_stuck"
	PalantirDrift    EventType = "palantir_drift"
	PalantirConflict EventType = "palantir_conflict"
	PalantirHealth   EventType = "palantir_health"
	PalantirNotes    EventType = "palantir_notes"
)

// allTypes lists every event type the CLI accepts on `events post`.
var allTypes = []EventType{
	GateSubmitted, GateApproved, GateRejected, PhaseTransition,
	LembasCompleted, MetadataUpdated, QuestHeld, QuestUnheld, QuestCompleted,
	PalantirStuck, PalantirDrift, PalantirConflict, PalantirHealth, PalantirNotes,
}

// Types returns the known event type names.
func Types() []string {
	out := make([]string, len(allTypes))
	for i, t := range allTypes {
		out[i] = string(t)
	}
	return out
}

// ValidType reports whether s names a known event type.
func ValidType(s string) (EventType, bool) {
	for _, t := range allTypes {
		if string(t) == s {
			return t, true
		}
	}
	return "", false
}

// Event represents a single quest event.
type Event struct {
	Timestamp string    `json:"timestamp"`
	Quest     string    `json:"quest"`
	Type      EventType `json:"type"`
	Phase     string    `json:"phase,omitempty"`
	Detail    string    `json:"detail,omitempty"`
}

// Record inserts an event into the store.
func Record(conn *db.Conn, t Event) error {
	return sqlitex.Execute(conn,
		`INSERT INTO herald (timestamp, quest, type, phase, detail) VALUES (?, ?, ?, ?, ?)`,
		&sqlitex.ExecOptions{
			Args: []any{t.Timestamp, t.Quest, string(t.Type), t.Phase, t.Detail},
		},
	)
}

// Read returns events for a single quest in ascending order (oldest first).
// If n > 0, returns the last n events.
func Read(conn *db.Conn, quest string, n int) ([]Event, error) {
	var events []Event

	var query string
	var args []any

	if n > 0 {
		// Subquery to get last n rows, then re-sort ascending.
		query = `SELECT timestamp, quest, type, phase, detail
			FROM (SELECT * FROM herald WHERE quest = ? ORDER BY id DESC LIMIT ?)
			ORDER BY id ASC`
		args = []any{quest, n}
	} else {
		query = `SELECT timestamp, quest, type, phase, detail FROM herald WHERE quest = ? ORDER BY id ASC`
		args = []any{quest}
	}

	err := sqlitex.Execute(conn, query, &sqlitex.ExecOptions{
		Args: args,
		ResultFunc: func(stmt *sqlite.Stmt) error {
			events = append(events, Event{
				Timestamp: stmt.ColumnText(0),
				Quest:     stmt.ColumnText(1),
				Type:      EventType(stmt.ColumnText(2)),
				Phase:     stmt.ColumnText(3),
				Detail:    stmt.ColumnText(4),
			})
			return nil
		},
	})
	if err != nil {
		return nil, fmt.Errorf("events: read quest %s: %w", quest, err)
	}

	if events == nil {
		events = []Event{}
	}
	return events, nil
}

// ReadAll returns events across all quests in ascending order (oldest first).
// If n > 0, returns the last n events.
func ReadAll(conn *db.Conn, n int) ([]Event, error) {
	var events []Event

	var query string
	var args []any

	if n > 0 {
		query = `SELECT timestamp, quest, type, phase, detail
			FROM (SELECT * FROM herald ORDER BY id DESC LIMIT ?)
			ORDER BY id ASC`
		args = []any{n}
	} else {
		query = `SELECT timestamp, quest, type, phase, detail FROM herald ORDER BY id ASC`
	}

	err := sqlitex.Execute(conn, query, &sqlitex.ExecOptions{
		Args: args,
		ResultFunc: func(stmt *sqlite.Stmt) error {
			events = append(events, Event{
				Timestamp: stmt.ColumnText(0),
				Quest:     stmt.ColumnText(1),
				Type:      EventType(stmt.ColumnText(2)),
				Phase:     stmt.ColumnText(3),
				Detail:    stmt.ColumnText(4),
			})
			return nil
		},
	})
	if err != nil {
		return nil, fmt.Errorf("events: read all: %w", err)
	}

	if events == nil {
		events = []Event{}
	}
	return events, nil
}
