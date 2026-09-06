package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// cliRun runs the built binary with args in dir, under the given session id,
// and returns exit code, stdout and stderr.
func cliRun(t *testing.T, bin, dir, sessionID string, stdin string, args ...string) (int, string, string) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "CLAUDE_CODE_SESSION_ID="+sessionID)
	cmd.Stdin = strings.NewReader(stdin)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	_ = cmd.Run()
	if cmd.ProcessState == nil {
		t.Fatalf("%v did not complete", args)
	}
	return cmd.ProcessState.ExitCode(), out.String(), errb.String()
}

// hookPayload builds a hook payload as Claude Code sends it for a tool call
// made by an in-process teammate: the lead's session id plus an agent id.
func hookPayload(sessionID, agentID, tool string, toolInput map[string]string) string {
	p := map[string]any{"session_id": sessionID, "hook_event_name": "PreToolUse", "tool_name": tool, "tool_input": toolInput}
	if agentID != "" {
		p["agent_id"] = agentID
		p["agent_type"] = "general-purpose"
	}
	b, _ := json.Marshal(p)
	return string(b)
}

// TestImplicitTeamSmoke is the end-to-end walk of one quest under the
// implicit-team API, through the real binary with real hook payloads: the
// lead initializes the fellowship from the main tree; a teammate that shares
// the lead's session id (an in-process background agent) works its worktree;
// each gate is blocked until both prerequisites are recorded, submitted, held
// pending, and approved from the main tree; completion is refused before
// Review and allowed in it.
func TestImplicitTeamSmoke(t *testing.T) {
	bin := buildFellowshipBinary(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := newMainRepo(t)
	const lead = "lead-session"
	const agent = "agent-quest-smoke"

	step := func(name string, code, want int, stderr string) {
		t.Helper()
		if code != want {
			t.Fatalf("%s: exit %d, want %d\n%s", name, code, want, stderr)
		}
		t.Logf("%s: exit %d %s", name, code, strings.TrimSpace(stderr))
	}

	// --- Lead, in the main tree ---------------------------------------------
	code, _, stderr := cliRun(t, bin, root, lead, "", "state", "init", "--name", "fellowship-smoke", "--base-branch", "main", "--skip-hook-install")
	step("state init", code, 0, stderr)

	wt := addWorktree(t, root, "quest-smoke")
	code, _, stderr = cliRun(t, bin, root, lead, "", "state", "add-quest", "--dir", root, "--name", "quest-smoke", "--task", "smoke test", "--branch", "quest-smoke", "--worktree", wt)
	step("state add-quest", code, 0, stderr)

	// --- Teammate: same session id as the lead, working in its worktree -----
	code, _, stderr = cliRun(t, bin, wt, lead, "", "init", "--dir", wt)
	step("teammate init", code, 0, stderr)

	teammate := func(name, hook string, tool string, in map[string]string) (int, string, string) {
		t.Helper()
		return cliRun(t, bin, wt, lead, hookPayload(lead, agent, tool, in), "hook", hook)
	}
	gate := func(phase string) map[string]string {
		return map[string]string{"to": "main", "summary": "gate", "message": "[GATE] " + phase + " complete\n\n## Summary\nsmoke"}
	}

	// The teammate cannot move its own phase or claim the lead, even though
	// its CLI environment carries the lead's session id.
	code, _, stderr = teammate("init --phase", "gate-guard", "Bash", map[string]string{"command": "fellowship init --dir " + wt + " --phase Implement"})
	step("gate-guard refuses teammate init --phase", code, 2, stderr)
	code, _, stderr = cliRun(t, bin, wt, lead, "", "init", "--dir", wt, "--phase", "Implement")
	step("CLI refuses init --phase from the worktree", code, 1, stderr)

	for i, phase := range []string{"Research", "Plan", "Implement"} {
		next := []string{"Plan", "Implement", "Review"}[i]

		// Gate without prerequisites: denied (JSON deny on stdout, exit 0).
		code, out, stderr := teammate("gate-submit early", "gate-submit", "SendMessage", gate(phase))
		step(phase+": gate-submit without prerequisites", code, 0, stderr)
		if !strings.Contains(out, `"permissionDecision":"deny"`) || !strings.Contains(out, "phase not confirmed") {
			t.Fatalf("%s: expected a deny naming the phase prerequisite, got %s", phase, out)
		}
		t.Logf("%s: gate-submit deny: %s", phase, strings.TrimSpace(out))

		// Prerequisite 1: /lembas, recorded by the PostToolUse gate-prereq hook.
		code, _, stderr = teammate("lembas", "gate-prereq", "Skill", map[string]string{"skill": "fellowship:lembas"})
		step(phase+": gate-prereq (lembas)", code, 0, stderr)

		// Prerequisite 2: phase confirm — the wrong phase is refused, the
		// current one recorded; the phase never moves.
		code, _, stderr = cliRun(t, bin, wt, lead, "", "phase", "confirm", "--dir", wt, "--phase", next)
		step(phase+": phase confirm --phase "+next, code, 1, stderr)
		code, _, stderr = cliRun(t, bin, wt, lead, "", "phase", "confirm", "--dir", wt, "--phase", phase)
		step(phase+": phase confirm --phase "+phase, code, 0, stderr)

		// Completion is refused before Review, by the guard and by the CLI.
		code, _, stderr = teammate("complete", "gate-guard", "Bash", map[string]string{"command": "fellowship complete --dir " + wt})
		step(phase+": gate-guard refuses `fellowship complete`", code, 2, stderr)
		code, _, stderr = cliRun(t, bin, wt, lead, "", "complete", "--dir", wt)
		step(phase+": CLI refuses `fellowship complete`", code, 1, stderr)

		// The gate goes through, enriched, and the quest is pending.
		code, out, stderr = teammate("gate-submit", "gate-submit", "SendMessage", gate(phase))
		step(phase+": gate-submit", code, 0, stderr)
		if strings.Contains(out, `"deny"`) {
			t.Fatalf("%s: gate was denied with prerequisites met: %s", phase, out)
		}
		if out != "" && !strings.Contains(out, `"message":"[GATE] `+phase) {
			t.Fatalf("%s: enrichment must return the body under `message`, got %s", phase, out)
		}
		code, out, stderr = cliRun(t, bin, wt, lead, "", "gate", "status", "--dir", wt)
		step(phase+": gate status", code, 0, stderr)
		if !strings.Contains(out, "Pending:  true") {
			t.Fatalf("%s: gate not pending after submit:\n%s", phase, out)
		}

		// Blocked while pending — Bash, Edit, a second gate — escape allowed.
		code, _, stderr = teammate("ls", "gate-guard", "Bash", map[string]string{"command": "ls"})
		step(phase+": gate-guard blocks Bash while pending", code, 2, stderr)
		code, _, stderr = teammate("edit", "gate-guard", "Edit", map[string]string{"file_path": filepath.Join(wt, "README.md")})
		step(phase+": gate-guard blocks Edit while pending", code, 2, stderr)
		code, out, stderr = teammate("second gate", "gate-submit", "SendMessage", gate(phase))
		step(phase+": second gate-submit while pending", code, 0, stderr)
		if !strings.Contains(out, "Gate already pending") {
			t.Fatalf("%s: second gate should be denied, got %s", phase, out)
		}
		code, _, stderr = teammate("status", "gate-guard", "Bash", map[string]string{"command": "~/.claude/fellowship/bin/fellowship gate status"})
		step(phase+": gate-guard allows the read-only escape", code, 0, stderr)
		// Self-approval from the worktree is blocked by the guard.
		code, _, stderr = teammate("self-approve", "gate-guard", "Bash", map[string]string{"command": "fellowship gate approve --dir " + wt})
		step(phase+": gate-guard blocks self-approval", code, 2, stderr)

		// The lead approves from the main tree, then messages the teammate.
		code, out, stderr = cliRun(t, bin, root, lead, "", "gate", "approve", "--dir", wt)
		step(phase+": lead gate approve", code, 0, stderr)
		if !strings.Contains(out, "Phase advanced to "+next) {
			t.Fatalf("%s: unexpected approve output %q", phase, out)
		}
		code, _, stderr = teammate("ls after approve", "gate-guard", "Bash", map[string]string{"command": "ls"})
		step(next+": gate-guard allows Bash after approval", code, 0, stderr)
	}

	// --- Review: no gate; completion allowed ---------------------------------
	// Even with both prerequisites recorded, no gate leaves Review.
	code, _, stderr = teammate("lembas", "gate-prereq", "Skill", map[string]string{"skill": "fellowship:lembas"})
	step("Review: gate-prereq (lembas)", code, 0, stderr)
	code, _, stderr = cliRun(t, bin, wt, lead, "", "phase", "confirm", "--dir", wt, "--phase", "Review")
	step("Review: phase confirm --phase Review", code, 0, stderr)
	code, out, stderr := teammate("gate in Review", "gate-submit", "SendMessage", gate("Review"))
	step("Review: gate-submit", code, 0, stderr)
	if !strings.Contains(out, "final phase") {
		t.Fatalf("Review: a gate must be refused, got %s", out)
	}
	code, _, stderr = teammate("complete", "gate-guard", "Bash", map[string]string{"command": "fellowship complete --dir " + wt})
	step("Review: gate-guard allows `fellowship complete`", code, 0, stderr)
	code, _, stderr = cliRun(t, bin, wt, lead, "", "complete", "--dir", wt)
	step("Review: `fellowship complete`", code, 0, stderr)

	code, out, stderr = cliRun(t, bin, root, lead, "", "state", "show", "--json")
	step("state show", code, 0, stderr)
	if !strings.Contains(out, `"status": "completed"`) {
		t.Fatalf("state show should report the quest completed:\n%s", out)
	}
	code, out, stderr = cliRun(t, bin, root, lead, "", "history", "show", "--quest", "quest-smoke", "--json")
	step("history show", code, 0, stderr)
	if !strings.Contains(out, `"status": "completed"`) {
		t.Fatalf("history should report the quest completed:\n%s", out)
	}
	// The lead can still re-claim itself: the in-process teammate's init did
	// not record the shared session id against the quest.
	code, _, stderr = cliRun(t, bin, root, lead, "", "state", "init", "--claim-lead")
	step("state init --claim-lead after the quest", code, 0, stderr)
}
