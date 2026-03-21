package dashboard

import (
	"fmt"
	"time"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"

	"github.com/justinjdev/fellowship/cli/internal/db"
)

// DashboardError represents a server-level error logged to the database.
type DashboardError struct {
	ID        int    `json:"id"`
	Timestamp string `json:"timestamp"`
	Source    string `json:"source"`
	Handler   string `json:"handler"`
	Message   string `json:"message"`
	Detail    string `json:"detail,omitempty"`
}

// maxErrors is the retention limit for dashboard errors. Older entries are
// pruned each time a new error is logged to prevent unbounded table growth.
const maxErrors = 500

// LogError inserts a dashboard error into the database and prunes old entries
// beyond the retention limit.
func LogError(conn *db.Conn, source, handler, message, detail string) error {
	ts := time.Now().UTC().Format(time.RFC3339)
	if err := sqlitex.Execute(conn,
		`INSERT INTO dashboard_errors (timestamp, source, handler, message, detail)
		 VALUES (?, ?, ?, ?, ?)`,
		&sqlitex.ExecOptions{
			Args: []any{ts, source, handler, message, detail},
		},
	); err != nil {
		return err
	}
	// Best-effort prune: keep only the most recent maxErrors rows.
	sqlitex.Execute(conn,
		`DELETE FROM dashboard_errors WHERE id NOT IN (SELECT id FROM dashboard_errors ORDER BY id DESC LIMIT ?)`,
		&sqlitex.ExecOptions{Args: []any{maxErrors}},
	)
	return nil
}

// ReadErrors returns the last n errors in descending order (newest first).
// If n <= 0, returns all errors.
func ReadErrors(conn *db.Conn, n int) ([]DashboardError, error) {
	var errors []DashboardError

	var query string
	var args []any

	if n > 0 {
		query = `SELECT id, timestamp, source, handler, message, detail
			FROM dashboard_errors ORDER BY id DESC LIMIT ?`
		args = []any{n}
	} else {
		query = `SELECT id, timestamp, source, handler, message, detail
			FROM dashboard_errors ORDER BY id DESC`
	}

	err := sqlitex.Execute(conn, query, &sqlitex.ExecOptions{
		Args: args,
		ResultFunc: func(stmt *sqlite.Stmt) error {
			errors = append(errors, DashboardError{
				ID:        stmt.ColumnInt(0),
				Timestamp: stmt.ColumnText(1),
				Source:    stmt.ColumnText(2),
				Handler:   stmt.ColumnText(3),
				Message:   stmt.ColumnText(4),
				Detail:    stmt.ColumnText(5),
			})
			return nil
		},
	})
	if err != nil {
		return nil, fmt.Errorf("dashboard: read errors: %w", err)
	}

	if errors == nil {
		errors = []DashboardError{}
	}
	return errors, nil
}

// ClearErrors deletes all dashboard errors from the database.
func ClearErrors(conn *db.Conn) error {
	return sqlitex.Execute(conn, `DELETE FROM dashboard_errors`, nil)
}
