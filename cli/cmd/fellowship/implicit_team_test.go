package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/justinjdev/fellowship/cli/internal/db"
	"github.com/justinjdev/fellowship/cli/internal/fellowship"
	"github.com/justinjdev/fellowship/cli/internal/history"
	"github.com/justinjdev/fellowship/cli/internal/state"
)

// --- fellowship phase confirm ----------------------------------------------

func loadQuest(t *testing.T, d *db.DB, quest string) *state.State {
	t.Helper()
	var s *state.State
	if err := d.WithConn(context.Background(), func(conn *db.Conn) error {
		var err error
		s, err = state.Load(conn, quest)
		return err
	}); err != nil {
		t.Fatalf("loading %s: %v", quest, err)
	}
	return s
}

// `phase confirm` records the phase prerequisite only for the quest's own
// current phase, and never moves the phase.
func TestRunPhaseConfirm(t *testing.T) {
	root := newMainRepo(t)
	t.Setenv("HOME", t.TempDir())
	worktree := addWorktree(t, root, "quest-confirm-alpha")
	t.Chdir(root)
	d := fellowshipWith(t, root, map[string]string{"quest-confirm-alpha": worktree})
	setQuestState(t, d, &state.State{QuestName: "quest-confirm-alpha", Phase: "Plan"})

	// Naming the wrong phase, or a non-phase, is refused and records nothing.
	for _, phase := range []string{"Research", "Implement", "Review", "Onboard", "plan"} {
		if got := runPhase(d, []string{"confirm", "--dir", worktree, "--phase", phase}); got != 1 {
			t.Errorf("phase confirm --phase %s = %d, want 1 (refused)", phase, got)
		}
		s := loadQuest(t, d, "quest-confirm-alpha")
		if s.MetadataUpdated {
			t.Errorf("--phase %s recorded the prerequisite", phase)
		}
		if s.Phase != "Plan" {
			t.Fatalf("--phase %s moved the phase to %s", phase, s.Phase)
		}
	}

	// The quest's own phase records it.
	if got := runPhase(d, []string{"confirm", "--dir", worktree, "--phase", "Plan"}); got != 0 {
		t.Fatalf("phase confirm --phase Plan = %d, want 0", got)
	}
	s := loadQuest(t, d, "quest-confirm-alpha")
	if !s.MetadataUpdated {
		t.Error("prerequisite not recorded")
	}
	if s.Phase != "Plan" {
		t.Errorf("phase = %s, want Plan (confirm never moves it)", s.Phase)
	}

	// No --phase, unknown subcommand, unregistered directory.
	if got := runPhase(d, []string{"confirm", "--dir", worktree}); got != 1 {
		t.Errorf("phase confirm without --phase = %d, want 1", got)
	}
	if got := runPhase(d, []string{"advance", "--dir", worktree, "--phase", "Implement"}); got != 1 {
		t.Errorf("phase advance = %d, want 1 (no such command)", got)
	}
	if got := runPhase(d, []string{"confirm", "--dir", t.TempDir(), "--phase", "Plan"}); got != 1 {
		t.Errorf("phase confirm in an unregistered directory = %d, want 1", got)
	}
}

// --- fellowship complete -----------------------------------------------------

func questEntry(t *testing.T, d *db.DB, name string) fellowship.QuestEntry {
	t.Helper()
	var entry fellowship.QuestEntry
	if err := d.WithConn(context.Background(), func(conn *db.Conn) error {
		fs, err := fellowship.LoadFellowship(conn)
		if err != nil {
			return err
		}
		for _, q := range fs.Quests {
			if q.Name == name {
				entry = q
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return entry
}

func historyStatus(t *testing.T, d *db.DB, name string) string {
	t.Helper()
	status := ""
	if err := d.WithConn(context.Background(), func(conn *db.Conn) error {
		h, err := history.Load(conn, name)
		if err != nil {
			return err
		}
		status = h.Status
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return status
}

// A quest ends only from Review with no gate pending. Before that, `complete`
// is refused and changes nothing; in Review it marks the history and the
// fellowship entry completed.
func TestRunComplete(t *testing.T) {
	root := newMainRepo(t)
	t.Setenv("HOME", t.TempDir())
	worktree := addWorktree(t, root, "quest-complete-alpha")
	t.Chdir(root)
	d := fellowshipWith(t, root, map[string]string{"quest-complete-alpha": worktree})

	for _, phase := range state.GatePhases() {
		setQuestState(t, d, &state.State{QuestName: "quest-complete-alpha", Phase: phase})
		if got := runComplete(d, []string{"--dir", worktree}); got != 1 {
			t.Errorf("complete in %s = %d, want 1 (refused)", phase, got)
		}
		if got := fellowship.QuestEntryStatus(questEntry(t, d, "quest-complete-alpha")); got != "active" {
			t.Errorf("complete in %s changed the entry status to %q", phase, got)
		}
	}

	// Review with a gate pending (a stale store) is still refused.
	gid := "gate-Implement-1"
	setQuestState(t, d, &state.State{QuestName: "quest-complete-alpha", Phase: state.TerminalPhase, GatePending: true, GateID: &gid})
	if got := runComplete(d, []string{"--dir", worktree}); got != 1 {
		t.Errorf("complete with a pending gate = %d, want 1 (refused)", got)
	}
	// Held is refused too.
	setQuestState(t, d, &state.State{QuestName: "quest-complete-alpha", Phase: state.TerminalPhase, Held: true})
	if got := runComplete(d, []string{"--dir", worktree}); got != 1 {
		t.Errorf("complete while held = %d, want 1 (refused)", got)
	}

	// Review, nothing pending: allowed, and both records say completed.
	setQuestState(t, d, &state.State{QuestName: "quest-complete-alpha", Phase: state.TerminalPhase})
	if got := runComplete(d, []string{"--dir", worktree}); got != 0 {
		t.Fatalf("complete in Review = %d, want 0", got)
	}
	if got := questEntry(t, d, "quest-complete-alpha").Status; got != "completed" {
		t.Errorf("entry status = %q, want completed", got)
	}
	if got := historyStatus(t, d, "quest-complete-alpha"); got != "completed" {
		t.Errorf("history status = %q, want completed", got)
	}
}

// The Bash form is refused by gate-guard under the same rule, through the
// real hook entry point — so a teammate whose prompt says "run complete" is
// stopped before the command runs, in every phase but Review.
func TestRunHookWith_GateGuardRefusesCompleteBeforeReview(t *testing.T) {
	root := newMainRepo(t)
	t.Setenv("HOME", t.TempDir())
	worktree := addWorktree(t, root, "quest-complete-beta")
	d := fellowshipWith(t, root, map[string]string{"quest-complete-beta": worktree})

	cmd := "~/.claude/fellowship/bin/fellowship complete --dir " + worktree
	for _, phase := range state.GatePhases() {
		setQuestState(t, d, &state.State{QuestName: "quest-complete-beta", Phase: phase})
		if got := runHookWith("gate-guard", bashInput(cmd), worktree, d); got != 2 {
			t.Errorf("gate-guard on `complete` in %s: exit %d, want 2 (block)", phase, got)
		}
	}
	setQuestState(t, d, &state.State{QuestName: "quest-complete-beta", Phase: state.TerminalPhase})
	if got := runHookWith("gate-guard", bashInput(cmd), worktree, d); got != 0 {
		t.Errorf("gate-guard on `complete` in Review: exit %d, want 0", got)
	}
}

// --- identity under the implicit team ---------------------------------------

// A teammate spawned in-process shares the lead's session id. `fellowship
// init` must not record that id against the quest: it identifies the session,
// not the agent, and recording it would make the lead "a teammate".
func TestRunInit_DoesNotRecordTheLeadSessionAgainstAQuest(t *testing.T) {
	root := newMainRepo(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(root)
	d := db.OpenTest(t)
	recordLead(t, d, root, "lead-1")

	t.Setenv("CLAUDE_CODE_SESSION_ID", "lead-1")
	if got := runInit(d, []string{"--quest", "alpha"}); got != 0 {
		t.Fatalf("init = %d, want 0", got)
	}
	if got := questSession(t, d, "alpha"); got != "" {
		t.Errorf("recorded quest session = %q, want none (it is the lead's id)", got)
	}
	// And so the lead can still claim itself.
	if got := claimLeadSession(d, root); got != 0 {
		t.Errorf("claim-lead after an in-process teammate ran init = %d, want 0", got)
	}

	// A separate-session teammate is still recorded, as before.
	t.Setenv("CLAUDE_CODE_SESSION_ID", "team-2")
	if got := runInit(d, []string{"--quest", "beta"}); got != 0 {
		t.Fatalf("init = %d, want 0", got)
	}
	if got := questSession(t, d, "beta"); got != "team-2" {
		t.Errorf("recorded quest session = %q, want team-2", got)
	}
}

// The lead's session id is not enough to move a phase from a process standing
// in a quest worktree: that is where an in-process teammate runs its
// commands, under the lead's id.
func TestRunInit_PhaseMoveNeedsTheMainWorktreeToo(t *testing.T) {
	root := newMainRepo(t)
	t.Setenv("HOME", t.TempDir())
	worktree := addWorktree(t, root, "quest-init-gamma")
	d := fellowshipWith(t, root, map[string]string{"quest-init-gamma": worktree})
	recordLead(t, d, root, "lead-1")
	setQuestState(t, d, &state.State{QuestName: "quest-init-gamma", Phase: "Research"})

	t.Setenv("CLAUDE_CODE_SESSION_ID", "lead-1")
	t.Chdir(worktree)
	if got := runInit(d, []string{"--dir", worktree, "--phase", "Implement"}); got != 1 {
		t.Errorf("init --phase from inside the worktree = %d, want 1 (refused)", got)
	}
	if got := questPhase(t, d, "quest-init-gamma"); got != "Research" {
		t.Errorf("phase = %q, want Research", got)
	}

	t.Chdir(root)
	if got := runInit(d, []string{"--dir", worktree, "--phase", "Implement"}); got != 0 {
		t.Errorf("init --phase from the main tree = %d, want 0", got)
	}
	if got := questPhase(t, d, "quest-init-gamma"); got != "Implement" {
		t.Errorf("phase = %q, want Implement", got)
	}
}

// The hook side of the same fact: a payload carrying the lead's session id
// AND an agent id is a subagent, and gets no lead exemption anywhere.
func TestRunHookWith_SubagentPayloadIsNotTheLead(t *testing.T) {
	root := newMainRepo(t)
	t.Setenv("HOME", t.TempDir())
	worktree := addWorktree(t, root, "quest-agent-delta")
	d := fellowshipWith(t, root, map[string]string{"quest-agent-delta": worktree})
	recordLead(t, d, root, "lead-1")
	setQuestState(t, d, &state.State{QuestName: "quest-agent-delta", Phase: "Research"})

	subagent := func(tool, body string) string {
		return `{"session_id":"lead-1","agent_id":"agent-1","agent_type":"general-purpose","tool_name":"` + tool + `","tool_input":` + body + `}`
	}
	// gate-guard: a lead-only command from the subagent is refused.
	in := strings.NewReader(subagent("Bash", `{"command":"fellowship init --dir `+worktree+` --phase Implement"}`))
	if got := runHookWith("gate-guard", in, worktree, d); got != 2 {
		t.Errorf("subagent init --phase: exit %d, want 2 (block)", got)
	}
	in = strings.NewReader(subagent("Bash", `{"command":"cd `+root+` && fellowship state init --claim-lead"}`))
	if got := runHookWith("gate-guard", in, worktree, d); got != 2 {
		t.Errorf("subagent state init --claim-lead: exit %d, want 2 (block)", got)
	}
	// The lead's own conversation, same session id, no agent id: exempt.
	in = strings.NewReader(`{"session_id":"lead-1","tool_name":"Bash","tool_input":{"command":"fellowship init --dir ` + worktree + ` --phase Implement"}}`)
	if got := runHookWith("gate-guard", in, worktree, d); got != 0 {
		t.Errorf("lead init --phase: exit %d, want 0", got)
	}

	// worktree-guard: the subagent writing source into the main tree from the
	// main tree is blocked; the lead's own conversation is not.
	target := filepath.Join(root, "src", "main.go")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	in = strings.NewReader(subagent("Write", `{"file_path":"`+target+`","content":"x"}`))
	if got := runHookWith("worktree-guard", in, root, d); got != 2 {
		t.Errorf("subagent main-tree write: exit %d, want 2 (block)", got)
	}
	in = strings.NewReader(`{"session_id":"lead-1","tool_name":"Write","tool_input":{"file_path":"` + target + `","content":"x"}}`)
	if got := runHookWith("worktree-guard", in, root, d); got != 0 {
		t.Errorf("lead main-tree write: exit %d, want 0", got)
	}
}

// --- gates.autoApprove with unknown names -----------------------------------

// A config written by an older plugin can carry retired or invalid gate names.
// init keeps the valid ones and warns, instead of failing every quest.
func TestRunInit_IgnoresUnknownAutoApproveGatesWithAWarning(t *testing.T) {
	root := newMainRepo(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Chdir(root)
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".claude", "fellowship.json"),
		[]byte(`{"gates":{"autoApprove":["Onboard","Research","Plan","Implement","Adversarial","Review"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	d := db.OpenTest(t)

	if got := runInit(d, []string{"--quest", "alpha"}); got != 0 {
		t.Fatalf("init with unknown autoApprove names = %d, want 0", got)
	}
	got := loadQuest(t, d, "alpha").AutoApproveGates
	if strings.Join(got, ",") != "Research,Plan,Implement" {
		t.Errorf("auto-approve gates = %v, want the three valid ones", got)
	}
}

func TestSplitAutoApproveGates(t *testing.T) {
	valid, unknown := splitAutoApproveGates([]string{"Onboard", "Research", "Review", "Plan"})
	if strings.Join(valid, ",") != "Research,Plan" || strings.Join(unknown, ",") != "Onboard,Review" {
		t.Errorf("split = %v / %v", valid, unknown)
	}
	if v, u := splitAutoApproveGates(nil); v != nil || u != nil {
		t.Errorf("nil split = %v / %v", v, u)
	}
	if v, u := splitAutoApproveGates([]string{}); v == nil || len(v) != 0 || u != nil {
		t.Errorf("empty split = %v / %v", v, u)
	}
}
