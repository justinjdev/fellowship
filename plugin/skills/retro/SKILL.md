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

1. Find the git root directory
2. Read `~/.claude/fellowship.json` to check for a custom `dataDir` setting. If absent, use the default data directory (`.fellowship/`).
3. Read `fellowship-state.json` from the resolved data directory to enumerate all quest worktrees and their metadata
4. If no fellowship state file exists, report "No fellowship state found — nothing to analyze" and stop

### Step 2: Collect Data

For each quest worktree listed in `fellowship-state.json`:

1. **Gate events:** Read `quest-herald.jsonl` from the worktree's data directory via the Read tool. Each line is a JSON object with `timestamp`, `quest`, `type`, `phase`, and `detail` fields. Collect all entries of type `gate_approved`, `gate_rejected`, `gate_submitted`, and `phase_transition`.

2. **Quest state:** Read `quest-state.json` from the worktree's data directory. Record the final `phase`, `quest_name`, and whether the quest completed.

3. **Quest tome:** Read `quest-tome.json` from the worktree's data directory if it exists. Record gate history (approved/rejected counts per phase), phases completed with durations, and files touched.

4. **Git metrics:** Run these Bash commands for each worktree:
   - `git -C {worktree} log --oneline | wc -l` — commit count
   - `git -C {worktree} diff --stat "$(git -C {worktree} rev-list --max-parents=0 HEAD | tail -n1)"..HEAD 2>/dev/null || echo "0 files changed"` — change summary

5. **Palantir alerts:** Read `palantir-alerts.jsonl` from the resolved data directory at the git root if it exists. Each line is a JSON object with `timestamp`, `type` (stuck/drift/conflict/health/bulletin), and `detail`. Standard alerts include `quests`; bulletin alerts include `source_quest`, `target_quest`, `topic`, and `discovery` instead.

6. **Autopsies:** Scan the `autopsies/` subdirectory within the resolved data directory. Each `.json` file is a structured failure record with `quest`, `phase`, `trigger`, `files`, `modules`, `what_failed`, and `resolution` fields. Collect all entries.

### Step 3: Analyze

Compute the following from collected data:

**Summary metrics:**
- Total quests completed vs failed (phase != "Complete")
- Total gate events: approved, rejected, submitted
- Rejection rate by phase (e.g., "Plan: 2/3 rejected, Research: 0/3 rejected")

**Phase patterns:**
- Which phases have the highest rejection rates
- Which quests spent the longest time in each phase (from tome phase durations)
- Phases where all gates were approved (candidates for auto-approve)

**Warden violations:**
- Check tome gate history for any rejection reasons mentioning convention or warden issues

**Palantir alert summary:**
- Count by type (stuck, drift, conflict, health, bulletin)
- Which quests were flagged most frequently

**Autopsy patterns:**
- Count autopsies by trigger (recovery, rejection, abandonment)
- Modules/files with multiple autopsies (hot spots for failure)
- Common tags across autopsies (recurring themes)

### Step 4: Present Results

Output the analysis in this format:

```
Fellowship Retrospective: {fellowship_name}
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Quests: {completed} completed, {failed} failed
Gates: {total} total, {rejected} rejected ({rejection_details})
Palantir alerts: {alert_summary}
Autopsies: {autopsy_count} ({trigger_breakdown})

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
- **Graceful degradation.** If some data sources are missing (no palantir alerts, no tome, worktree deleted), analyze what's available and note what was missing.
- **Terminal output only.** No saved retro files — keeps it lightweight and ephemeral.
