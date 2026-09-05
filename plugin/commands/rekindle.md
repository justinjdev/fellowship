---
description: Recover a fellowship after a session crash. Scans worktrees and quest state, presents a recovery dashboard, and re-spawns Gandalf with recovered quest context. Use when returning to a crashed or expired fellowship session.
---

# Rekindle — Fellowship Crash Recovery

## Overview

Reconstructs fellowship state from on-disk artifacts after a session crash and transitions into Gandalf coordinator mode with recovered context. The flame that was quenched can be rekindled.

## When to Use

- Session crashed or context window filled up during a fellowship
- User returns to find scattered worktrees from a previous fellowship
- User invokes `/rekindle` directly

## Process

> **Note:** `.fellowship/` is the default data directory. Users can override it via `dataDir` in `~/.claude/fellowship.json`. All `fellowship` CLI commands and paths below use the configured data directory automatically.

### Step 1: Scan

Run the CLI to discover fellowship artifacts:

```bash
~/.claude/fellowship/bin/fellowship status --json
```

This reads the fellowship and quest state recorded in the fellowship database, checks each worktree for a checkpoint (`.fellowship/checkpoint.md`), and detects branches already merged into the fellowship's base branch.

If no quests are found, report: "There is nothing to rekindle. The ashes have gone cold." and stop.

### Step 2: Classify

Each quest gets one classification:

| Classification | Condition | Action |
|---|---|---|
| **Shipped** | Branch merged into the base branch | Skip — already shipped |
| **Resumable** | Has `.fellowship/checkpoint.md` | Continue from current phase with checkpoint context |
| **Stale** | No checkpoint | Restart current phase from scratch |

### Step 3: Present Recovery Dashboard

Show the user what was found:

```
The flame that was quenched can be rekindled.
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

quest-api-auth    │ Implement (checkpoint ✓)  │ Resumable
quest-db-schema   │ Plan      (checkpoint ✓)  │ Resumable
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
3. **Record fellowship state:** From the repo root, run `~/.claude/fellowship/bin/fellowship state init --name fellowship-{timestamp}` (it has no `--dir` — it operates on the current directory), then re-register each recovered quest with `~/.claude/fellowship/bin/fellowship state add-quest --name <quest_name> --task "<task>" [--branch <branch>] [--worktree <path>]` (same as `/fellowship` startup)
4. **Clean up stale flags:** Run `~/.claude/fellowship/bin/fellowship state clean-worktrees` to reset any `gate_pending`/`held` flags a crashed session left set — a quest that crashed mid-gate or mid-hold should not resume already blocked.
5. **Write failure records for dead quests:** Before respawning, run `~/.claude/fellowship/bin/fellowship failures infer --dir <worktree>` for each quest classified as `stale`. This preserves failure knowledge from the crashed session for future quests to learn from.
6. **For each quest that has not shipped:**
   a. `TaskCreate` with the original task description (from `~/.claude/fellowship/bin/fellowship state show --json` or inferred from quest name)
   b. Spawn a quest runner teammate with the **resume spawn prompt** (see below)
7. **Enter Gandalf coordinator loop** — same behavior as `/fellowship` (gate handling, status reports, user commands)

**Resume spawn prompt:** use the **Resume** variant of the base quest spawn
template in `plugin/skills/fellowship/resources/spawn-prompts.md`. Rekindle
keeps no copy of it — the gate, hold, isolation, and boundary language is the
same for a resumed quest as for a fresh one, and a second copy here would
drift. Fill the variant's `{worktree_path}`, `{phase}`, `{classification}`,
and `{checkpoint_line}` from the Step 1 scan and the Step 2 classification,
and every shared placeholder exactly as `/fellowship` does.

Two things are worth saying out loud when you send it:

- The quest resumes at the phase its state records. Recovery never skips a
  phase and never waives a gate.
- A quest classified `stale` restarts its current phase from scratch; it does
  not fall back to an earlier one.

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
2. **Checkpoint is king.** `.fellowship/checkpoint.md` is the primary per-quest recovery artifact. `/rekindle` is the recovery path *outside* a running quest; inside one, quest's Research step 0 is the only place that reads a checkpoint.
3. **New team, new tasks.** Old task IDs are stale. Recovery creates fresh coordination state.
4. **Same Gandalf behavior.** After recovery, the coordinator loop is identical to `/fellowship`.
5. **One spawn template.** The resume prompt is a variant of the shared base template, not a copy. Gate, hold, isolation, and boundary language is edited in one place.
6. **Graceful degradation.** No fellowship recorded in the database? Fall back to worktree scanning. No checkpoint? Restart the phase.
