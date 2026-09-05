package state_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/justinjdev/fellowship/cli/internal/db"
	"github.com/justinjdev/fellowship/cli/internal/state"
)

// writeLegacyMarker writes the pre-store <data-dir>/lead file by hand. Nothing
// in the CLI writes it any more; these tests keep the one-release fallback
// read honest.
func writeLegacyMarker(t *testing.T, root, dataDirName, sessionID string) {
	t.Helper()
	path := state.LeadMarkerPath(root, dataDirName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(state.Lead{SessionID: sessionID, Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLead_StoreRoundTrip(t *testing.T) {
	root := t.TempDir()
	d := db.OpenTest(t)

	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		return state.RecordLead(conn, root, "session-123")
	}); err != nil {
		t.Fatalf("RecordLead: %v", err)
	}

	if err := d.WithConn(context.Background(), func(conn *db.Conn) error {
		lead, found, err := state.ReadLead(conn)
		if err != nil {
			return err
		}
		if !found {
			t.Fatal("ReadLead found no lead after RecordLead")
		}
		if lead.SessionID != "session-123" {
			t.Errorf("SessionID = %q, want session-123", lead.SessionID)
		}
		if lead.Root != root {
			t.Errorf("Root = %q, want %q", lead.Root, root)
		}
		if lead.CreatedAt == "" {
			t.Error("CreatedAt should be recorded")
		}
		if got := state.LeadSessionID(conn, root, ".fellowship"); got != "session-123" {
			t.Errorf("LeadSessionID = %q, want session-123", got)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// Re-recording replaces the lead rather than accumulating rows: a resumed
// fellowship gets a new session id, and only one session can be the lead.
func TestLead_RecordReplaces(t *testing.T) {
	root := t.TempDir()
	d := db.OpenTest(t)
	for _, sid := range []string{"first", "second"} {
		if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
			return state.RecordLead(conn, root, sid)
		}); err != nil {
			t.Fatalf("RecordLead(%s): %v", sid, err)
		}
	}
	if err := d.WithConn(context.Background(), func(conn *db.Conn) error {
		if got := state.LeadSessionID(conn, root, ".fellowship"); got != "second" {
			t.Errorf("LeadSessionID = %q, want second", got)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// The whole point of moving the lead into the store: a marker file a teammate
// wrote must not overrule the lead the store names.
func TestLead_StoreBeatsLegacyMarker(t *testing.T) {
	root := t.TempDir()
	d := db.OpenTest(t)
	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		return state.RecordLead(conn, root, "real-lead")
	}); err != nil {
		t.Fatal(err)
	}
	writeLegacyMarker(t, root, ".fellowship", "spoofed-teammate")

	if err := d.WithConn(context.Background(), func(conn *db.Conn) error {
		if got := state.LeadSessionID(conn, root, ".fellowship"); got != "real-lead" {
			t.Errorf("LeadSessionID = %q, want real-lead (the store wins)", got)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// A fellowship initialized by the previous release recorded its lead in the
// data directory. That file is still honored — but only while the store names
// no lead at all.
func TestLead_LegacyMarkerFallback(t *testing.T) {
	root := t.TempDir()
	d := db.OpenTest(t)
	writeLegacyMarker(t, root, "queststate", "session-abc")

	if got := state.LeadMarkerPath(root, "queststate"); got != filepath.Join(root, "queststate", "lead") {
		t.Errorf("LeadMarkerPath = %q", got)
	}
	if err := d.WithConn(context.Background(), func(conn *db.Conn) error {
		if got := state.LeadSessionID(conn, root, "queststate"); got != "session-abc" {
			t.Errorf("LeadSessionID = %q, want session-abc", got)
		}
		// The marker is per data directory: looking in the default one finds nothing.
		if got := state.LeadSessionID(conn, root, ".fellowship"); got != "" {
			t.Errorf("LeadSessionID in the default dir = %q, want empty", got)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// A missing marker and a corrupt one both read as "lead unknown", never as an
// error the guard would act on.
func TestLead_MissingAndCorruptMarker(t *testing.T) {
	root := t.TempDir()
	d := db.OpenTest(t)

	m, err := state.ReadLeadMarker(root, ".fellowship")
	if err != nil {
		t.Fatalf("missing marker should not error: %v", err)
	}
	if m.SessionID != "" {
		t.Errorf("SessionID = %q, want empty", m.SessionID)
	}

	if err := os.MkdirAll(filepath.Join(root, ".fellowship"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".fellowship", "lead"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := state.ReadLeadMarker(root, ".fellowship"); err == nil {
		t.Error("a corrupt marker should report an error to callers that want one")
	}
	if err := d.WithConn(context.Background(), func(conn *db.Conn) error {
		if got := state.LeadSessionID(conn, root, ".fellowship"); got != "" {
			t.Errorf("LeadSessionID on a corrupt marker = %q, want empty", got)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// No session id available (not run by Claude Code): the row still records that
// a lead initialized here, and the guard reads "unknown".
func TestLead_EmptySessionID(t *testing.T) {
	root := t.TempDir()
	d := db.OpenTest(t)
	if err := d.WithTx(context.Background(), func(conn *db.Conn) error {
		return state.RecordLead(conn, root, "")
	}); err != nil {
		t.Fatal(err)
	}
	if err := d.WithConn(context.Background(), func(conn *db.Conn) error {
		_, found, err := state.ReadLead(conn)
		if err != nil {
			return err
		}
		if !found {
			t.Error("a lead with no session id should still be recorded")
		}
		if got := state.LeadSessionID(conn, root, ".fellowship"); got != "" {
			t.Errorf("LeadSessionID = %q, want empty", got)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestCurrentSessionID(t *testing.T) {
	t.Setenv("CLAUDE_SESSION_ID", "")
	t.Setenv("CLAUDE_CODE_SESSION_ID", "sid-1")
	if got := state.CurrentSessionID(); got != "sid-1" {
		t.Errorf("CurrentSessionID() = %q, want sid-1", got)
	}
	t.Setenv("CLAUDE_CODE_SESSION_ID", "")
	t.Setenv("CLAUDE_SESSION_ID", "sid-2")
	if got := state.CurrentSessionID(); got != "sid-2" {
		t.Errorf("CurrentSessionID() = %q, want sid-2", got)
	}
	t.Setenv("CLAUDE_SESSION_ID", "")
	if got := state.CurrentSessionID(); got != "" {
		t.Errorf("CurrentSessionID() = %q, want empty", got)
	}
}
