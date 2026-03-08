---
description: Recover a fellowship after a session crash. Scans worktrees and state files, presents a recovery dashboard, and re-spawns Gandalf with recovered quest context. Use when returning to a crashed or expired fellowship session.
---

# Rekindle — Fellowship Crash Recovery

## Overview

Reconstructs fellowship state from on-disk artifacts after a session crash and transitions into Gandalf coordinator mode with recovered context. The flame that was quenched can be rekindled.

## When to Use

- Session crashed or context window filled up during a fellowship
- User returns to find scattered worktrees from a previous fellowship
- User invokes `/rekindle` directly

## Process

### Step 1: Scan

Run the CLI to discover fellowship artifacts:

```bash
fellowship status --json
```

This scans all git worktrees for `.fellowship/quest-state.json` files, checks for checkpoints (`.fellowship/checkpoint.md`), detects merged branches, and reads `.fellowship/fellowship-state.json` from the main repo.

If no quests are found, report: "There is nothing to rekindle. The ashes have gone cold." and stop.

### Step 2: Classify

Each quest gets one classification:

| Classification | Condition | Action |
|---|---|---|
| **Complete** | Branch merged into main | Skip — already shipped |
| **Resumable** | Has `.fellowship/checkpoint.md` | Continue from current phase with checkpoint context |
| **Stale** | No checkpoint | Restart current phase from scratch |

### Step 3: Present Recovery Dashboard

Show the user what was found:

```
The flame that was quenched can be rekindled.
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

quest-api-auth    │ Implement (checkpoint ✓) │ Resumable
quest-db-schema   │ Plan      (checkpoint ✓) │ Resumable
quest-ui-login    │ Research  (no checkpoint) │ Restart phase

Merged (skipping):
  fellowship/config-fix (branch merged into main)

Proceed with recovery? (y/n)
```

If the user declines, stop. Do not proceed without confirmation.

### Step 4: Re-spawn Fellowship

On user confirmation, transition into Gandalf coordinator mode:

1. **Load config:** Read `~/.claude/fellowship.json` if it exists (same as `/fellowship`)
2. **Create team:** `TeamCreate` with name `fellowship-{timestamp}`
3. **Write fellowship state:** Write `.fellowship/fellowship-state.json` with recovered quest list (same as `/fellowship` startup)
4. **Install hooks:** Run `fellowship install` to set up gate enforcement
5. **For each non-complete quest:**
   a. `TaskCreate` with the original task description (from `fellowship-state.json` or inferred from quest name)
   b. Spawn a quest runner teammate with the **resume spawn prompt** (see below)
6. **Enter Gandalf coordinator loop** — same behavior as `/fellowship` (gate handling, status reports, user commands)

**Resume spawn prompt:**

```
You are a quest runner resuming after a session crash.
Gandalf the White has returned: "I come back to you now, at the turn of the tide."

YOUR TASK: {task_description}

RESUME CONTEXT:
- Your worktree already exists at {worktree_path}
- Your current phase: {phase}
- Classification: {classification}
- Checkpoint: {if resumable: "Load .fellowship/checkpoint.md for recovered context" | if stale: "No checkpoint — restart current phase from scratch"}

INSTRUCTIONS:
1. Run /quest to resume this task
2. In Phase 0 (Onboard), detect the RESUME CONTEXT block above and:
   - Skip worktree creation — you are already in your worktree
   - Run `fellowship install` to restore hooks
   - Run `fellowship init` to reset gate state (clears gate_pending, preserves phase)
   - Store your worktree path in task metadata: TaskUpdate(taskId: "{task_id}", metadata: {"worktree_path": "{worktree_path}"})
   - If checkpoint exists, load .fellowship/checkpoint.md as your initial context
   - Skip /council — checkpoint replaces orientation
   - Proceed to your current phase: {phase}
3. Gate handling — gates are enforced by plugin hooks via a state file
   (.fellowship/quest-state.json). The hooks structurally block your tools
   after gate submission. Here is how it works:

   Before EACH gate, you MUST:
   a. Run /lembas to compress context (hooks verify this)
   b. Run TaskUpdate(taskId: "{task_id}", metadata: {"phase": "<phase>"})
      to record your current phase (hooks verify this)
   c. Send ONE gate checklist via SendMessage to the lead.
      The message content MUST start with [GATE] — e.g.:
      "[GATE] Research complete\n- [x] Key files identified..."
      Messages without the [GATE] prefix are not detected as gates.

   After sending a gate message, your Edit/Write/Bash/Agent/Skill tools
   are blocked by hooks until the lead approves. You cannot bypass this.
   The lead approves by updating your state file — only the lead can
   unblock you.

   {gate_config_override}

   NEVER send two gates in one message.
   NEVER approve your own gates — only the lead can approve.
   NEVER write "approved" or "proceeding" — that is the lead's language.
4. When /quest reaches Phase 5 (Complete), create a PR and message
   the lead with the PR URL
5. If you get stuck or need a decision, message the lead
6. If you receive a shutdown request, respond immediately using
   SendMessage with type "shutdown_response", approve: true, and
   the request_id from the message. Do not just acknowledge in text.

CONVENTIONS:
- Use conventional commits for all git commits (e.g., feat:, fix:, docs:, refactor:)

BOUNDARIES:
- Stay in YOUR worktree. Do NOT read, write, or navigate into other
  teammates' worktrees. Your working directory is your worktree root.
- Do NOT use MCP tools or external service integrations (Notion, Slack,
  Jira, etc.) without first messaging the lead and getting explicit
  approval. Your scope is local: code, tests, git, and the filesystem.
- Do NOT push branches, create PRs, or take any action visible to
  others without lead approval (except at Phase 5 as instructed above).

CONTEXT:
- Fellowship team: {team_name}
- Your quest: {quest_name}
- Your task ID: {task_id}
- Other active quests: {brief_list}
- PR config: {pr_config_line}
```

**Substitution rules:** Same as `/fellowship` spawn prompt, plus:

| Placeholder | Source |
|---|---|
| `{worktree_path}` | From `fellowship status --json` output |
| `{phase}` | From quest state file |
| `{classification}` | "resumable" or "stale" |

### Gandalf's Voice (Recovery)

| Moment | Line |
|--------|------|
| Starting recovery | "The flame that was quenched can be rekindled." |
| Re-spawning quests | "I come back to you now, at the turn of the tide." |
| All quests resumed | "The board is set once more. The pieces are moving." |
| Quest already complete | "That road has already been walked. We need not tread it again." |
| No artifacts found | "There is nothing to rekindle. The ashes have gone cold." |

## Key Principles

1. **User confirms before recovery.** Never auto-resume without showing what was found.
2. **Checkpoint is king.** `.fellowship/checkpoint.md` is the primary per-quest recovery artifact.
3. **New team, new tasks.** Old task IDs are stale. Recovery creates fresh coordination state.
4. **Same Gandalf behavior.** After recovery, the coordinator loop is identical to `/fellowship`.
5. **Graceful degradation.** No fellowship-state.json? Fall back to worktree scanning. No checkpoint? Restart the phase.
