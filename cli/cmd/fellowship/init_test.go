package main

import (
	"context"
	"testing"

	"github.com/justinjdev/fellowship/cli/internal/db"
	"github.com/justinjdev/fellowship/cli/internal/history"
	"github.com/justinjdev/fellowship/cli/internal/state"
)

// questPhase reads the live phase of a quest row.
func questPhase(t *testing.T, d *db.DB, quest string) string {
	t.Helper()
	var phase string
	if err := d.WithConn(context.Background(), func(conn *db.Conn) error {
		s, err := state.Load(conn, quest)
		if err != nil {
			return err
		}
		phase = s.Phase
		return nil
	}); err != nil {
		t.Fatalf("loading %s: %v", quest, err)
	}
	return phase
}

func questPhaseCount(t *testing.T, d *db.DB, quest string) int {
	t.Helper()
	n := 0
	if err := d.WithConn(context.Background(), func(conn *db.Conn) error {
		phases, err := history.LoadPhases(conn, quest)
		n = len(phases)
		return err
	}); err != nil {
		t.Fatalf("loading history for %s: %v", quest, err)
	}
	return n
}

// `fellowship init --phase X` on an EXISTING quest row rewrites the phase and
// sails past gate-guard, because nothing is pending after the reset. That is a
// self-approval: only the lead may move a phase.
func TestRunInit_PhaseMoveOnExistingRowIsLeadOnly(t *testing.T) {
	root := newMainRepo(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(root)
	d := db.OpenTest(t)

	recordLead(t, d, root, "lead-1")

	// Bootstrap: the row does not exist yet, so anyone may pick the phase.
	t.Setenv("CLAUDE_CODE_SESSION_ID", "teammate-1")
	if got := runInit(d, []string{"--quest", "alpha"}); got != 0 {
		t.Fatalf("initial init = %d, want 0", got)
	}
	if got := questPhase(t, d, "alpha"); got != "Research" {
		t.Fatalf("phase after bootstrap = %q, want Research", got)
	}

	// The attack: a teammate session rewrites the phase.
	if got := runInit(d, []string{"--quest", "alpha", "--phase", "Implement"}); got != 1 {
		t.Errorf("teammate init --phase = %d, want 1 (refused)", got)
	}
	if got := questPhase(t, d, "alpha"); got != "Research" {
		t.Errorf("phase after the teammate attempt = %q, want Research", got)
	}

	// Same attack through --plan-skip, which implies --phase Implement.
	if got := runInit(d, []string{"--quest", "alpha", "--plan-skip"}); got != 1 {
		t.Errorf("teammate init --plan-skip = %d, want 1 (refused)", got)
	}
	if got := questPhase(t, d, "alpha"); got != "Research" {
		t.Errorf("phase after the --plan-skip attempt = %q, want Research", got)
	}
	if n := questPhaseCount(t, d, "alpha"); n != 0 {
		t.Errorf("refused --plan-skip recorded %d skipped phases, want 0", n)
	}

	// The honest path: the lead moves the phase.
	t.Setenv("CLAUDE_CODE_SESSION_ID", "lead-1")
	if got := runInit(d, []string{"--quest", "alpha", "--phase", "Implement"}); got != 0 {
		t.Errorf("lead init --phase = %d, want 0", got)
	}
	if got := questPhase(t, d, "alpha"); got != "Implement" {
		t.Errorf("phase after the lead moved it = %q, want Implement", got)
	}
}

// A plain `fellowship init` (no --phase) is a gate/prereq reset, which any
// teammate may run — it cannot advance the quest.
func TestRunInit_ResetWithoutPhaseIsAllowedForTeammates(t *testing.T) {
	root := newMainRepo(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(root)
	d := db.OpenTest(t)
	recordLead(t, d, root, "lead-1")

	gateID := "gate-Research-1"
	setQuestState(t, d, &state.State{
		QuestName: "alpha", Phase: "Research", GatePending: true, GateID: &gateID,
	})

	t.Setenv("CLAUDE_CODE_SESSION_ID", "teammate-1")
	if got := runInit(d, []string{"--quest", "alpha"}); got != 0 {
		t.Fatalf("teammate init = %d, want 0", got)
	}
	if got := questPhase(t, d, "alpha"); got != "Research" {
		t.Errorf("phase = %q, want Research", got)
	}
}

// With no lead recorded nobody can be called a teammate — but the phase move is
// still refused, because an unidentified caller is not the lead either.
func TestRunInit_PhaseMoveRefusedWhenNoLeadRecorded(t *testing.T) {
	root := newMainRepo(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(root)
	d := db.OpenTest(t)
	setQuestState(t, d, &state.State{QuestName: "alpha", Phase: "Research"})

	t.Setenv("CLAUDE_CODE_SESSION_ID", "someone")
	if got := runInit(d, []string{"--quest", "alpha", "--phase", "Implement"}); got != 0 {
		t.Errorf("init with no recorded lead = %d, want 0 (warn, keep the phase)", got)
	}
	if got := questPhase(t, d, "alpha"); got != "Research" {
		t.Errorf("phase = %q, want Research (the move must be ignored)", got)
	}
}
