package state_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/justinjdev/fellowship/cli/internal/state"
)

func TestLeadMarker_RoundTrip(t *testing.T) {
	root := t.TempDir()
	if err := state.WriteLeadMarker(root, ".fellowship", "session-123"); err != nil {
		t.Fatalf("WriteLeadMarker: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, ".fellowship", "lead")); err != nil {
		t.Fatalf("marker file: %v", err)
	}

	m, err := state.ReadLeadMarker(root, ".fellowship")
	if err != nil {
		t.Fatalf("ReadLeadMarker: %v", err)
	}
	if m.SessionID != "session-123" {
		t.Errorf("SessionID = %q, want session-123", m.SessionID)
	}
	if m.Root != root {
		t.Errorf("Root = %q, want %q", m.Root, root)
	}
	if m.CreatedAt == "" {
		t.Error("CreatedAt should be recorded")
	}
	if got := state.LeadSessionID(root, ".fellowship"); got != "session-123" {
		t.Errorf("LeadSessionID = %q, want session-123", got)
	}
}

func TestLeadMarker_CustomDataDir(t *testing.T) {
	root := t.TempDir()
	if err := state.WriteLeadMarker(root, "queststate", "session-abc"); err != nil {
		t.Fatalf("WriteLeadMarker: %v", err)
	}
	if got := state.LeadMarkerPath(root, "queststate"); got != filepath.Join(root, "queststate", "lead") {
		t.Errorf("LeadMarkerPath = %q", got)
	}
	if got := state.LeadSessionID(root, "queststate"); got != "session-abc" {
		t.Errorf("LeadSessionID = %q, want session-abc", got)
	}
	// The marker is per data directory: looking in the default one finds nothing.
	if got := state.LeadSessionID(root, ".fellowship"); got != "" {
		t.Errorf("LeadSessionID in the default dir = %q, want empty", got)
	}
}

// A fellowship initialized before the marker existed has none, and a corrupt
// marker is no better than a missing one: both read as "lead unknown", never as
// an error the guard would act on.
func TestLeadMarker_MissingAndCorrupt(t *testing.T) {
	root := t.TempDir()
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
	if got := state.LeadSessionID(root, ".fellowship"); got != "" {
		t.Errorf("LeadSessionID on a corrupt marker = %q, want empty", got)
	}
}

func TestWriteLeadMarker_EmptySessionID(t *testing.T) {
	root := t.TempDir()
	// No session id available (not run by Claude Code): the marker still
	// records that a lead initialized here, and the guard reads "unknown".
	if err := state.WriteLeadMarker(root, ".fellowship", ""); err != nil {
		t.Fatalf("WriteLeadMarker: %v", err)
	}
	if got := state.LeadSessionID(root, ".fellowship"); got != "" {
		t.Errorf("LeadSessionID = %q, want empty", got)
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
