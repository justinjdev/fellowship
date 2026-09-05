package main

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/justinjdev/fellowship/cli/internal/db"
	"github.com/justinjdev/fellowship/cli/internal/state"
)

// The verifier's repro, end to end: one Bash tool call from a quest worktree,
// `cd <main-repo-root> && fellowship state init --claim-lead`, made the teammate
// the recorded lead. gate-guard only looked for `init --phase`, and
// worktree-guard never sees Bash at all.
func TestRunHookWith_BlocksStateCommandsFromAQuestWorktree(t *testing.T) {
	root := newMainRepo(t)
	worktree := addWorktree(t, root, "quest-claim-alpha")
	d := fellowshipWith(t, root, map[string]string{"quest-claim-alpha": worktree})
	recordLead(t, d, root, "lead-1")
	setQuestState(t, d, &state.State{QuestName: "quest-claim-alpha", Phase: "Implement", SessionID: "team-1"})

	blocked := []string{
		"cd " + root + " && fellowship state init --claim-lead",
		"cd " + root + " && fellowship state init --name evil --skip-hook-install",
		`sh -c "cd ` + root + ` && fellowship state init --claim-lead"`,
		"~/.claude/fellowship/bin/fellowship state init --claim-lead",
		"fellowship state update-quest --name quest-claim-alpha --status completed",
		`sh -c "fellowship init --phase Review"`,
		"~/.claude/fellowship/bin/fellowship init --phase Review",
	}
	for _, cmd := range blocked {
		if got := runHookWith("gate-guard", teamBashInput("team-1", cmd), worktree, d); got != 2 {
			t.Errorf("gate-guard on %q: exit %d, want 2 (block)", cmd, got)
		}
	}

	// The teammate's own commands are untouched.
	allowed := []string{
		"fellowship gate status",
		"fellowship todo list",
		"fellowship init --dir " + worktree,
		"go test ./...",
	}
	for _, cmd := range allowed {
		if got := runHookWith("gate-guard", teamBashInput("team-1", cmd), worktree, d); got != 0 {
			t.Errorf("gate-guard on %q: exit %d, want 0 (allow)", cmd, got)
		}
	}
}

// The CLI-side backstop for the same attack: a session recorded against a quest
// cannot name itself the lead, even standing in the main working tree.
func TestStateInit_ClaimLeadRefusesATeammateSession(t *testing.T) {
	root := newMainRepo(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(root)
	d := db.OpenTest(t)

	t.Setenv("CLAUDE_CODE_SESSION_ID", "lead-1")
	if got := runStateInit(d, []string{"--name", "demo", "--skip-hook-install"}); got != 0 {
		t.Fatalf("state init = %d, want 0", got)
	}
	// A teammate ran `fellowship init` for its quest, recording its session.
	setQuestState(t, d, &state.State{QuestName: "quest-1", Phase: "Implement", SessionID: "team-1"})

	t.Setenv("CLAUDE_CODE_SESSION_ID", "team-1")
	if got := runStateInit(d, []string{"--claim-lead"}); got != 1 {
		t.Errorf("teammate --claim-lead = %d, want 1 (refused)", got)
	}
	if got := recordedLead(t, d, root); got != "lead-1" {
		t.Errorf("recorded lead = %q, want lead-1 (unchanged)", got)
	}

	// The lead's own new session may still claim it.
	t.Setenv("CLAUDE_CODE_SESSION_ID", "lead-2")
	if got := runStateInit(d, []string{"--claim-lead"}); got != 0 {
		t.Errorf("lead --claim-lead = %d, want 0", got)
	}
	if got := recordedLead(t, d, root); got != "lead-2" {
		t.Errorf("recorded lead = %q, want lead-2", got)
	}
}

// A plain `state init` re-recorded the lead as whoever ran it. It is not a
// re-record any more: that is --claim-lead's job.
func TestStateInit_DoesNotSilentlyReRecordTheLead(t *testing.T) {
	root := newMainRepo(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(root)
	d := db.OpenTest(t)

	t.Setenv("CLAUDE_CODE_SESSION_ID", "lead-1")
	if got := runStateInit(d, []string{"--name", "demo", "--skip-hook-install"}); got != 0 {
		t.Fatalf("state init = %d, want 0", got)
	}

	t.Setenv("CLAUDE_CODE_SESSION_ID", "team-1")
	if got := runStateInit(d, []string{"--name", "evil", "--skip-hook-install"}); got != 0 {
		t.Fatalf("second state init = %d, want 0", got)
	}
	if got := recordedLead(t, d, root); got != "lead-1" {
		t.Errorf("recorded lead = %q, want lead-1 (a second state init must not re-record it)", got)
	}
}

// `fellowship init` records which session is working the quest — the data the
// --claim-lead check reads.
func TestRunInit_RecordsTheTeammateSession(t *testing.T) {
	root := newMainRepo(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(root)
	d := db.OpenTest(t)

	t.Setenv("CLAUDE_CODE_SESSION_ID", "team-1")
	if got := runInit(d, []string{"--quest", "alpha"}); got != 0 {
		t.Fatalf("init = %d, want 0", got)
	}
	if got := questSession(t, d, "alpha"); got != "team-1" {
		t.Errorf("recorded quest session = %q, want team-1", got)
	}

	// Re-running init keeps it current rather than clearing it.
	if got := runInit(d, []string{"--quest", "alpha"}); got != 0 {
		t.Fatalf("second init = %d, want 0", got)
	}
	if got := questSession(t, d, "alpha"); got != "team-1" {
		t.Errorf("recorded quest session after re-init = %q, want team-1", got)
	}

	if err := d.WithConn(context.Background(), func(conn *db.Conn) error {
		is, err := state.SessionIsTeammate(conn, "team-1")
		if err != nil {
			return err
		}
		if !is {
			t.Error("SessionIsTeammate should recognize the recorded session")
		}
		is, err = state.SessionIsTeammate(conn, "lead-1")
		if err != nil {
			return err
		}
		if is {
			t.Error("SessionIsTeammate should not claim an unrelated session")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func questSession(t *testing.T, d *db.DB, quest string) string {
	t.Helper()
	sid := ""
	if err := d.WithConn(context.Background(), func(conn *db.Conn) error {
		s, err := state.Load(conn, quest)
		if err != nil {
			return err
		}
		sid = s.SessionID
		return nil
	}); err != nil {
		t.Fatalf("loading %s: %v", quest, err)
	}
	return sid
}

// teamBashInput builds a Bash PreToolUse payload for a given session.
func teamBashInput(sessionID, command string) io.Reader {
	return strings.NewReader(`{"session_id":` + quote(sessionID) +
		`,"tool_name":"Bash","tool_input":{"command":` + quote(command) + `}}`)
}
