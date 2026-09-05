package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// LeadMarkerName is the file, inside the fellowship data directory, that
// records which Claude Code session started the fellowship.
const LeadMarkerName = "lead"

// LeadMarker is the content of that file.
//
// The worktree-guard needs to tell the lead's own session (which legitimately
// writes in the main worktree) from a quest teammate that was mis-placed into
// it. Nothing in the git topology distinguishes them — both resolve to the main
// root — so the lead is identified by session: `fellowship state init` records
// the id of the session it ran in, and hook payloads carry the id of the
// session the hook is firing for.
type LeadMarker struct {
	// SessionID is the Claude Code session that ran `fellowship state init`.
	// Empty when the environment exposed no session id, in which case the
	// guard falls back to its narrower rule.
	SessionID string `json:"session_id,omitempty"`
	// Root is the main worktree root the fellowship was initialized in.
	Root string `json:"root,omitempty"`
	// CreatedAt is when the marker was written (RFC3339, UTC).
	CreatedAt string `json:"created_at,omitempty"`
}

// LeadMarkerPath returns the marker path for a data directory in root.
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

// WriteLeadMarker records sessionID as the fellowship's lead session. An empty
// sessionID is still written (with the root and timestamp) so the marker
// documents that a lead was recorded but no session id was available.
func WriteLeadMarker(root, dataDirName, sessionID string) error {
	path := LeadMarkerPath(root, dataDirName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("state: create data directory: %w", err)
	}
	data, err := json.Marshal(LeadMarker{
		SessionID: sessionID,
		Root:      root,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return fmt.Errorf("state: encode lead marker: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("state: write lead marker: %w", err)
	}
	return nil
}

// ReadLeadMarker reads the marker. A missing marker is not an error: fellowships
// initialized before the marker existed simply have none, and the guard treats
// that as "lead unknown".
func ReadLeadMarker(root, dataDirName string) (LeadMarker, error) {
	data, err := os.ReadFile(LeadMarkerPath(root, dataDirName))
	if err != nil {
		if os.IsNotExist(err) {
			return LeadMarker{}, nil
		}
		return LeadMarker{}, fmt.Errorf("state: read lead marker: %w", err)
	}
	var m LeadMarker
	if err := json.Unmarshal(data, &m); err != nil {
		return LeadMarker{}, fmt.Errorf("state: parse lead marker: %w", err)
	}
	return m, nil
}

// LeadSessionID returns the recorded lead session id, or "" for any reason it
// cannot be read. Callers use it to identify the lead, never to block, so a
// failure to read it must read as "unknown" rather than propagate.
func LeadSessionID(root, dataDirName string) string {
	m, err := ReadLeadMarker(root, dataDirName)
	if err != nil {
		return ""
	}
	return m.SessionID
}
