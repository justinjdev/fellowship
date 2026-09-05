---
name: retro
description: Post-fellowship retrospective analysis. Collects gate history, palantir alerts, and quest metrics to surface patterns and interactively recommend configuration improvements.
---

# Retro — Post-Fellowship Retrospective

## Overview

Analyzes a completed fellowship's history to surface patterns and interactively offer configuration improvements. Reads gate events, palantir alerts, and git metrics across all quest worktrees, then presents findings and walks through actionable recommendations one by one.

## When to Use

- **Manual:** invoke `/retro` after a fellowship completes or is disbanded
- **Suggested:** Gandalf suggests running `/retro` during the disband flow (not enforced)

## Process

### Step 1: Locate Fellowship Data

1. Find the git root directory and run all commands below from it
2. Enumerate the fellowship's quests and their metadata:
   ```bash
   ~/.claude/fellowship/bin/fellowship state show --json
   ```
   The output is JSON with `name`, `quests[]` (each with `name`, `worktree`, `branch`, `task_description`, `status`), `scouts[]`, and `groups[]`. `status` is `completed` or `cancelled` for a finished quest — this is what Step 2.2 below checks instead of asking each worktree individually.
3. If the command reports that no fellowship is initialized, report "No fellowship state found — nothing to analyze" and stop

### Step 2: Collect Data

Everything below reads the fellowship database via the CLI — there are no state files to open.

1. **Gate events:** Run `~/.claude/fellowship/bin/fellowship events --limit 0 --json` for the whole fellowship (or `--quest <name>` for one quest). Each entry has `timestamp`, `quest`, `type`, `phase`, and `detail`. Collect all entries of type `gate_approved`, `gate_rejected`, `gate_submitted`, and `phase_transition`.

2. **Quest state:** Run `~/.claude/fellowship/bin/fellowship health --json` once for every quest's `phase` and `health` (`stalled`/`zombie`/`idle` quests and a `struggling` flag are useful retrospective signals in their own right — a quest that struggled but still finished is worth calling out) — no need to shell into each worktree individually with `gate status --dir <worktree>`. Cross-reference with the `status` field from Step 1's `state show --json` to record whether the quest reached `completed`.

3. **Quest history:** Run `~/.claude/fellowship/bin/fellowship history show --quest <quest_name> --json`. Record gate history (approved/rejected counts per phase), phases completed with durations, and files touched.

4. **Git metrics:** Run these Bash commands for each worktree:
   - `git -C {worktree} log --oneline | wc -l` — commit count
   - `git -C {worktree} diff --stat "$(git -C {worktree} rev-list --max-parents=0 HEAD | tail -n1)"..HEAD 2>/dev/null || echo "0 files changed"` — change summary

5. **Palantir alerts:** The palantir records its alerts as events, so they come from the same event output as step 1 — collect the entries whose `type` starts with `palantir_` (`palantir_stuck`, `palantir_drift`, `palantir_conflict`, `palantir_health`, `palantir_notes`). The `quest` field names the quest the alert is about and `detail` carries the alert text.

6. **Failure records:** Run `~/.claude/fellowship/bin/fellowship failures scan --all`. Each record has `quest`, `phase`, `trigger`, `files`, `modules`, `what_failed`, and `resolution` fields. Collect all entries.

### Step 3: Analyze

Compute the following from collected data:

**Summary metrics:**
- Total quests completed vs failed (phase != "Complete")
- Total gate events: approved, rejected, submitted
- Rejection rate by phase (e.g., "Plan: 2/3 rejected, Research: 0/3 rejected")

**Phase patterns:**
- Which phases have the highest rejection rates
- Which quests spent the longest time in each phase (from history phase durations)
- Phases where all gates were approved (candidates for auto-approve)
- Quests health flagged `struggling` at any point (from Step 2.2) — even one that finished is worth naming, since repeated rejections in one phase are a pattern worth a recommendation

**Warden violations:**
- Check history gate entries for any rejection reasons mentioning convention or warden issues

**Palantir alert summary:**
- Count by type (`palantir_stuck`, `palantir_drift`, `palantir_conflict`, `palantir_health`, `palantir_notes`)
- Which quests were flagged most frequently

**Failure patterns:**
- Count failure records by trigger (recovery, rejection, abandonment)
- Modules/files with multiple failure records (hot spots for failure)
- Common tags across failure records (recurring themes)

### Step 4: Present Results

Output the analysis in this format:

```
Fellowship Retrospective: {fellowship_name}
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Quests: {completed} completed, {failed} failed
Gates: {total} total, {rejected} rejected ({rejection_details})
Palantir alerts: {alert_summary}
Failure records: {failure_count} ({trigger_breakdown})

Observations:
- {observation_1}
- {observation_2}
- {observation_3}

Recommendations:
1. {recommendation_1}
2. {recommendation_2}
3. {recommendation_3}
```

**Observation examples:**
- "Plan gates rejected 2/3 times — plans needed more research context"
- "quest-ui-login spent longest in Research — task may have been under-specified"
- "No warden violations — conventions well-established"
- "2 file conflict alerts — consider splitting shared files across quests"
- "auth module has 4 autopsies — consider documenting its quirks in CLAUDE.md"
- "3 recovery autopsies in Implement — plans may need more detail"

**Recommendation examples:**
- Auto-approve phases with 0% rejection rate
- Keep manual gates that have high rejection rates (proving their value)
- Suggest templates for recurring quest patterns
- Adjust palantir settings if alerts were excessive or insufficient

### Step 5: Interactive Recommendations

Walk through each recommendation one at a time. For each:

1. Present the recommendation with current and proposed values:

```
Recommendation N: {description}
Currently: {current_value}
Proposed: {proposed_value}

Apply? (y/n)
```

2. Use `AskUserQuestion` with y/n choices

3. **On accept:** Read `~/.claude/fellowship.json` (create if it doesn't exist). Apply the change to the appropriate config key. Write the file back. Report what was changed.

4. **On reject:** Move to the next recommendation without changes.

5. After all recommendations are presented, summarize what was applied:

```
Applied {n} of {total} recommendations:
- {applied_1}
- {applied_2}

Config updated at ~/.claude/fellowship.json
```

If no recommendations were accepted, say "No changes applied."

## Key Principles

- **Read-only analysis.** The retro skill only reads worktree data — it never modifies quest state, worktrees, or git history.
- **Config changes only on explicit accept.** Each recommendation requires individual user confirmation before applying.
- **Graceful degradation.** If some data sources are missing (no palantir alerts, no history, worktree deleted), analyze what's available and note what was missing.
- **Terminal output only.** No saved retro files — keeps it lightweight and ephemeral.
