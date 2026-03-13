package herald

import (
	"fmt"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"

	"github.com/justinjdev/fellowship/cli/internal/db"
)

// TidingType represents the type of a quest tiding.
type TidingType string

const (
	GateSubmitted   TidingType = "gate_submitted"
	GateApproved    TidingType = "gate_approved"
	GateRejected    TidingType = "gate_rejected"
	PhaseTransition TidingType = "phase_transition"
	LembasCompleted TidingType = "lembas_completed"
	MetadataUpdated TidingType = "metadata_updated"
	QuestHeld       TidingType = "quest_held"
	QuestUnheld     TidingType = "quest_unheld"
)

// Tiding represents a single quest event.
type Tiding struct {
	Timestamp string     `json:"timestamp"`
	Quest     string     `json:"quest"`
	Type      TidingType `json:"type"`
	Phase     string     `json:"phase,omitempty"`
	Detail    string     `json:"detail,omitempty"`
}

// Announce inserts a tiding into the herald table.
func Announce(conn *db.Conn, t Tiding) error {
	return sqlitex.Execute(conn,
		`INSERT INTO herald (timestamp, quest, type, phase, detail) VALUES (?, ?, ?, ?, ?)`,
		&sqlitex.ExecOptions{
			Args: []any{t.Timestamp, t.Quest, string(t.Type), t.Phase, t.Detail},
		},
	)
}

// Read returns tidings for a single quest in ascending order (oldest first).
// If n > 0, returns the last n tidings.
func Read(conn *db.Conn, quest string, n int) ([]Tiding, error) {
	var tidings []Tiding

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
			tidings = append(tidings, Tiding{
				Timestamp: stmt.ColumnText(0),
				Quest:     stmt.ColumnText(1),
				Type:      TidingType(stmt.ColumnText(2)),
				Phase:     stmt.ColumnText(3),
				Detail:    stmt.ColumnText(4),
			})
			return nil
		},
	})
	if err != nil {
		return nil, fmt.Errorf("herald: read quest %s: %w", quest, err)
	}

	if tidings == nil {
		tidings = []Tiding{}
	}
	return tidings, nil
}

// ReadAll returns tidings across all quests in ascending order (oldest first).
// If n > 0, returns the last n tidings.
func ReadAll(conn *db.Conn, n int) ([]Tiding, error) {
	var tidings []Tiding

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
			tidings = append(tidings, Tiding{
				Timestamp: stmt.ColumnText(0),
				Quest:     stmt.ColumnText(1),
				Type:      TidingType(stmt.ColumnText(2)),
				Phase:     stmt.ColumnText(3),
				Detail:    stmt.ColumnText(4),
			})
			return nil
		},
	})
	if err != nil {
		return nil, fmt.Errorf("herald: read all: %w", err)
	}

	if tidings == nil {
		tidings = []Tiding{}
	}
	return tidings, nil
}
