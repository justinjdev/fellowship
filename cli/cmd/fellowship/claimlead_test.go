package main

import (
	"context"
	"testing"

	"github.com/justinjdev/fellowship/cli/internal/db"
	"github.com/justinjdev/fellowship/cli/internal/fellowship"
	"github.com/justinjdev/fellowship/cli/internal/state"
)

func recordedLead(t *testing.T, d *db.DB, root string) string {
	t.Helper()
	lead := ""
	if err := d.WithConn(context.Background(), func(conn *db.Conn) error {
		lead = state.LeadSessionID(conn, root, ".fellowship")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return lead
}

// A lead whose session id changed mid-fellowship (a new session in the main
// tree rather than a resumed one) is refused by its own guard. --claim-lead is
// the way back, and it changes nothing but the lead.
func TestStateInit_ClaimLead(t *testing.T) {
	root := newMainRepo(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(root)
	d := db.OpenTest(t)

	t.Setenv("CLAUDE_CODE_SESSION_ID", "lead-1")
	if got := runStateInit(d, []string{"--name", "demo", "--skip-hook-install"}); got != 0 {
		t.Fatalf("state init = %d, want 0", got)
	}
	if got := recordedLead(t, d, root); got != "lead-1" {
		t.Fatalf("recorded lead = %q, want lead-1", got)
	}

	// A new session takes over the main tree.
	t.Setenv("CLAUDE_CODE_SESSION_ID", "lead-2")
	if got := runStateInit(d, []string{"--claim-lead"}); got != 0 {
		t.Fatalf("state init --claim-lead = %d, want 0", got)
	}
	if got := recordedLead(t, d, root); got != "lead-2" {
		t.Errorf("recorded lead = %q, want lead-2", got)
	}
	// The fellowship itself is untouched — --claim-lead is not a re-init.
	if got := fellowshipName(t, d); got != "demo" {
		t.Errorf("fellowship name = %q, want demo", got)
	}
}

// Without a session id the lead is anonymous, and the guard falls back to its
// narrower rule. Say so rather than recording it silently.
func TestStateInit_WarnsWithoutSessionID(t *testing.T) {
	root := newMainRepo(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(root)
	d := db.OpenTest(t)

	t.Setenv("CLAUDE_CODE_SESSION_ID", "")
	t.Setenv("CLAUDE_SESSION_ID", "")
	if got := runStateInit(d, []string{"--name", "demo", "--skip-hook-install"}); got != 0 {
		t.Fatalf("state init = %d, want 0", got)
	}
	if got := recordedLead(t, d, root); got != "" {
		t.Errorf("recorded lead = %q, want empty", got)
	}
	// The row exists all the same, documenting that a lead initialized here.
	if err := d.WithConn(context.Background(), func(conn *db.Conn) error {
		_, found, err := state.ReadLead(conn)
		if err != nil {
			return err
		}
		if !found {
			t.Error("a lead row should be written even with no session id")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func fellowshipName(t *testing.T, d *db.DB) string {
	t.Helper()
	name := ""
	if err := d.WithConn(context.Background(), func(conn *db.Conn) error {
		fs, err := fellowship.LoadFellowship(conn)
		if err != nil {
			return err
		}
		name = fs.Name
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return name
}
