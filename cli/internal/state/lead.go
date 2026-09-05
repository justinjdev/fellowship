package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

// LeadMarkerName is the legacy file, inside the fellowship data directory, that
// recorded which Claude Code session started the fellowship. Fellowships now
// record the lead in the store (the `lead` table); the file is read as a
// one-release fallback and never written.
const LeadMarkerName = "lead"

// Lead is the fellowship's recorded lead session.
//
// The worktree-guard needs to tell the lead's own session (which legitimately
// writes in the main worktree) from a quest teammate that was mis-placed into
// it. Nothing in the git topology distinguishes them — both resolve to the main
// root — so the lead is identified by session: `fellowship state init` records
// the id of the session it ran in, and hook payloads carry the id of the
// session the hook is firing for.
//
// It lives in SQLite because everything else about the data directory is
// writable by the sessions this identifies: the guards exempt the data
// directory so teammates can keep coordination files there, which made the old
// <data-dir>/lead file a marker its own subjects could forge. Teammates cannot
// write SQLite through Edit/Write, and the store file itself is no longer
// exempt from the write guards.
type Lead struct {
	// SessionID is the Claude Code session that ran `fellowship state init`.
	// Empty when the environment exposed no session id, in which case the
	// guard falls back to its narrower rule.
	SessionID string `json:"session_id,omitempty"`
	// Root is the main worktree root the fellowship was initialized in.
	Root string `json:"root,omitempty"`
	// CreatedAt is when the lead was recorded (RFC3339, UTC).
	CreatedAt string `json:"created_at,omitempty"`
}

// LeadMarkerPath returns the legacy marker path for a data directory in root.
func LeadMarkerPath(root, dataDirName string) string {
	if dataDirName == "" {
		dataDirName = ".fellowship"
	}
	return filepath.Join(root, dataDirName, LeadMarkerName)
}

// CurrentSessionID returns the Claude Code session id of the session running
// this process, or "" when it is not being run by Claude Code (a plain shell,
// a test) or the id is not exported. Claude Code exports the same id it puts
// in hook payloads as CLAUDE_CODE_SESSION_ID.
func CurrentSessionID() string {
	for _, key := range []string{"CLAUDE_CODE_SESSION_ID", "CLAUDE_SESSION_ID"} {
		if v := os.Getenv(key); v != "" {
			return v
		}
	}
	return ""
}

// RecordLead writes sessionID as the fellowship's lead session, replacing any
// previously recorded lead. An empty sessionID is still recorded (with the root
// and timestamp) so the row documents that a lead was recorded but no session
// id was available.
func RecordLead(conn *sqlite.Conn, root, sessionID string) error {
	err := sqlitex.Execute(conn, `INSERT INTO lead (id, session_id, root, created_at)
		VALUES (1, :session_id, :root, :now)
		ON CONFLICT(id) DO UPDATE SET
			session_id = :session_id, root = :root, created_at = :now`,
		&sqlitex.ExecOptions{
			Named: map[string]any{
				":session_id": sessionID,
				":root":       root,
				":now":        time.Now().UTC().Format(time.RFC3339),
			},
		})
	if err != nil {
		return fmt.Errorf("state: record lead: %w", err)
	}
	return nil
}

// SessionIsTeammate reports whether sessionID is recorded against any quest —
// that is, whether the session running this command is a quest teammate.
//
// It is the store-side half of "only the lead may name the lead": a teammate
// that reached the main working tree and ran `state init --claim-lead` would
// otherwise become the lead, which is the whole thing the lead row exists to
// prevent. An empty id is nobody and reports false; so does a fellowship whose
// teammates were started without a session id in their environment, which is
// why gate-guard's refusal of lead commands from a quest worktree is the
// primary defense and this is the backstop.
func SessionIsTeammate(conn *sqlite.Conn, sessionID string) (bool, error) {
	if sessionID == "" {
		return false, nil
	}
	found := false
	err := sqlitex.Execute(conn,
		`SELECT 1 FROM quest_state WHERE session_id = :sid LIMIT 1`,
		&sqlitex.ExecOptions{
			Named: map[string]any{":sid": sessionID},
			ResultFunc: func(stmt *sqlite.Stmt) error {
				found = true
				return nil
			},
		})
	if err != nil {
		return false, fmt.Errorf("state: look up quest sessions: %w", err)
	}
	return found, nil
}

// ReadLead returns the recorded lead. found is false when no lead row exists,
// which is not an error: a fellowship initialized by an older binary has none.
func ReadLead(conn *sqlite.Conn) (lead Lead, found bool, err error) {
	err = sqlitex.Execute(conn, `SELECT session_id, root, created_at FROM lead WHERE id = 1`,
		&sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				found = true
				lead.SessionID = stmt.ColumnText(0)
				lead.Root = stmt.ColumnText(1)
				lead.CreatedAt = stmt.ColumnText(2)
				return nil
			},
		})
	if err != nil {
		return Lead{}, false, fmt.Errorf("state: read lead: %w", err)
	}
	return lead, found, nil
}

// ReadLeadMarker reads the legacy marker file. A missing marker is not an
// error: fellowships that recorded their lead in the store have none, and the
// guard treats a missing one as "lead unknown".
//
// Deprecated: the lead lives in the store (see RecordLead). This remains only
// so a fellowship initialized by the previous release keeps its lead for one
// more release, and it is consulted only when the store holds no lead at all.
func ReadLeadMarker(root, dataDirName string) (Lead, error) {
	data, err := os.ReadFile(LeadMarkerPath(root, dataDirName))
	if err != nil {
		if os.IsNotExist(err) {
			return Lead{}, nil
		}
		return Lead{}, fmt.Errorf("state: read lead marker: %w", err)
	}
	var m Lead
	if err := json.Unmarshal(data, &m); err != nil {
		return Lead{}, fmt.Errorf("state: parse lead marker: %w", err)
	}
	return m, nil
}

// LeadSessionID returns the recorded lead session id, or "" for any reason it
// cannot be read. Callers use it to identify the lead, never to block, so a
// failure to read it must read as "unknown" rather than propagate.
//
// The store is the authority. The legacy marker file is consulted only when the
// store holds no lead row at all — a store that names a lead is never overruled
// by a file the sessions it identifies could have written themselves.
func LeadSessionID(conn *sqlite.Conn, root, dataDirName string) string {
	if conn != nil {
		if lead, found, err := ReadLead(conn); err == nil && found {
			return lead.SessionID
		}
	}
	m, err := ReadLeadMarker(root, dataDirName)
	if err != nil {
		return ""
	}
	return m.SessionID
}
