---
name: quest
description: Use for any non-trivial task. Orchestrates the Research-Plan-Implement cycle with compaction between phases, integrating council, lembas, gather-lore, and warden. Enforces discipline and phase gates.
---

# Quest — Research, Plan, Implement

## Overview

Orchestrates the full Research → Plan → Implement cycle with intentional compaction between phases. This is the hub skill that enforces context engineering discipline by invoking satellite skills at the right moments and maintaining hard gates between phases.

## When to Use

- Any task that involves more than a quick fix
- When you want structured, disciplined progress through a complex change
- When context management matters (large codebase, multi-file changes)

## Phase Flow

```
Phase 0: Onboard ──→ EnterWorktree → /council
     │
     ▼
Phase 1: Research ──→ Explore agents / /gather-lore
     │                Goal: understand the system, identify files
     ▼
  /lembas
     │
     ▼
Phase 2: Plan ──────→ Plan mode / writing-plans
     │                Goal: explicit steps with file:line refs
     ▼
  /lembas
     │
     ▼
Phase 3: Implement ──→ TDD (test-driven-development) + execute plan
     │                 Goal: small verifiable changes via red-green-refactor
     ▼
  /lembas
     │
     ▼
Phase 4: Review ─────→ /warden → /pr-review-toolkit:review-pr
     │                 → verification-before-completion
     │                 Goal: conventions + code quality + verified passing
     ▼
  /lembas
     │
     ▼
Phase 5: Complete ───→ finishing-a-development-branch
                       Goal: squash/merge, PR creation, cleanup
```

## Process

### Fellowship Integration

When running as a fellowship teammate (indicated by the spawn prompt), the gate prerequisite order at the end of each phase is:
1. Run `/lembas` to compress context (hooks verify this)
2. Update task metadata: `TaskUpdate(taskId: "<your_task_id>", metadata: {"phase": "<current_phase>"})` (hooks verify this)
3. Send `[GATE]` message to the lead via SendMessage

Both steps 1 and 2 must complete before step 3 — the hooks will block gate submission otherwise. Valid phase names: Onboard, Research, Plan, Implement, Review, Complete.

### Gate State Machine

When running as a fellowship teammate, a state file at `tmp/quest-state.json` enforces gate discipline via plugin hooks. The hooks structurally prevent you from working after submitting a gate, skipping lembas, or skipping metadata updates. You do not need to manage this file — the hooks handle it automatically.

**What the hooks enforce:**
- **Phase-aware file guard:** During Onboard, Research, and Plan phases, Edit/Write to files outside `tmp/` are blocked. You cannot modify production code until you reach the Implement phase by submitting gates. Bash, Agent, Skill, and reads are allowed in all phases.
- Gate messages must start with `[GATE]` to be detected (e.g., `[GATE] Research complete\n- [x] ...`)
- After you send a gate message, your Edit/Write/Bash/Agent/Skill tools are blocked until the lead approves
- Before you can send a gate message, you must have run `/lembas` and updated task metadata with your current phase
- You cannot send a second gate while one is pending
- You cannot mark your task as completed unless your phase is `Complete`

**State file initialization** happens at Phase 0 (see below). If you are resuming a failed quest and `tmp/quest-state.json` already exists, the file is preserved with `gate_pending` reset to `false`.

### Phase 0: Onboard

1. **Config:** Read `~/.claude/fellowship.json` (the user's personal Claude directory) if it exists. Merge with defaults (see fellowship skill for the full schema). If the file does not exist, all defaults apply.
2. **Isolate:** Detect whether you're resuming an existing worktree: check if task metadata contains `worktree_path` (via `TaskGet`) and the path exists on disk. If so, you're already isolated — skip worktree creation. Otherwise, if `config.worktree.enabled` is true (default), create an isolated worktree:
   - **Resolve branch name:** Determine the branch name using config:
     1. If `branch.pattern` is set: substitute `{slug}`, `{ticket}`, `{author}` placeholders (see below).
     2. Else: use `fellowship/{slug}`.
   - **Placeholder resolution:**
     - `{slug}`: slugify the task description (lowercase, hyphens for spaces, strip non-alphanumeric). If a ticket was extracted, derive slug from the remaining text after extraction.
     - `{ticket}`: match `branch.ticketPattern` (default: `[A-Z]+-\d+`) against the task description. If matched, use the match. If not matched and the pattern contains `{ticket}`, ask the user to provide a ticket ID.
     - `{author}`: use `branch.author` from config. If not set and the pattern contains `{author}`, ask the user to provide their name.
   - **Create worktree (3-step sequence — all steps are REQUIRED):**
     1. Run `git rev-parse HEAD` and save the full SHA in your response text (not a shell variable — shell state does not persist between tool calls). This is the base commit.
     2. Call `EnterWorktree` with the resolved branch name. If `config.worktree.directory` is set, create the worktree there instead of the default location.
     3. **Immediately** after entering the worktree — before ANY other action — run `git reset --hard <sha>` using the exact SHA from step 1. `EnterWorktree` bases off the default branch, not the current branch. This reset is what makes the worktree start from the correct point. Skip this and the worktree will be wrong.
   - **Install hooks in worktree (fellowship only):** After entering the worktree, project-level hooks must be re-created so gate enforcement continues. Run: `fellowship install`. This must happen before the state file creation below.
3. **State file (fellowship only):** This MUST happen before any other tool calls (Skill, Bash, etc.) so that hooks can enforce gates from the start. If running as a fellowship teammate:
   - If `tmp/quest-state.json` already exists (respawn), reset `gate_pending` to `false` and preserve the existing `phase`.
   - Otherwise, create `tmp/quest-state.json`:
     ```json
     {
       "version": 1,
       "quest_name": "<quest_name>",
       "task_id": "<task_id>",
       "team_name": "<team_name>",
       "phase": "Onboard",
       "gate_pending": false,
       "gate_id": null,
       "lembas_completed": false,
       "metadata_updated": false,
       "auto_approve_gates": []
     }
     ```
     Populate `auto_approve_gates` from `config.gates.autoApprove` if set.
   - Store the worktree path in task metadata: `TaskUpdate(taskId: "<task_id>", metadata: {"worktree_path": "<cwd>"})`
4. **Orient:** Invoke `/council` to load task-relevant context.

If the user has already described their task, pass the description directly. Otherwise, council will ask.

**Gate:** Isolation set up (worktree created or skipped per config) AND Session Context block must exist before proceeding.

### Phase 1: Research

Goal: Understand the system well enough to plan changes. Stay objective — gather information, don't propose solutions yet.

**Actions:**
1. If entering an unfamiliar area, invoke `/gather-lore` to extract conventions from reference files
2. Use Explore agents (Task tool, subagent_type=Explore) to scan relevant code paths
3. Read key files identified in the Session Context
4. Document findings: how the current system works, constraints, edge cases

**Hard gate — Research must produce:**
- [ ] Key files identified with specific line ranges
- [ ] Constraints and dependencies documented
- [ ] Current behavior understood (not just file locations)

If these aren't met, continue researching. Don't proceed to planning with incomplete understanding.

**Transition:** Invoke `/lembas` with phase "Research" before moving to Plan.

### Phase 2: Plan

Goal: Outline explicit steps with file:line references and a test strategy.

**Actions:**
1. Enter plan mode (EnterPlanMode) or use the writing-plans skill for formal plans
2. Write steps that reference specific files and line ranges from research
3. Define test strategy: what to test, how to verify
4. Assess whether the plan has 2+ independent workstreams

**Hard gate — Plan must include:**
- [ ] Explicit file paths and line ranges for every change
- [ ] Test strategy (what tests to write or run)
- [ ] User approval of the plan

**Transition:** Invoke `/lembas` with phase "Plan" before moving to Implement.

### Phase 3: Implement

Goal: Execute the plan with small, verifiable changes and tight feedback loops. Default to TDD.

**Execution mode — choose based on plan structure:**

**Single-stream (default):** Tasks are sequential or tightly coupled.
1. Invoke `superpowers:test-driven-development` — red-green-refactor for each unit of work
2. Execute the plan step by step
3. Verify after each change (run tests, check build)
4. Commit each logical unit

**Parallel subagents:** Plan has 3+ independent tasks touching different files.
1. Dispatch multiple implementation subagents simultaneously (multiple Task tool calls in one message)
2. Each subagent gets the full task text, relevant context, and TDD instructions
3. No two subagents modify the same file — this is a planning constraint, not a runtime guard. If the plan has file conflicts between subtasks, fix the plan.
4. Collect results, review each, then commit

**Guidelines:**
- **TDD by default.** Write the failing test first, then the minimal implementation, then refactor.
- Follow the plan. If the plan is wrong, trigger recovery (see below) — don't silently deviate.
- Small changes. One function, one test, one commit. Not a big-bang change.
- Use conventional commits for all git commits (e.g., `feat:`, `fix:`, `docs:`, `refactor:`).
- Verify as you go. Don't batch all testing to the end.

**Recovery — when implementation hits a wall:**

Trigger recovery when any of these occur:
- A plan step is impossible or wrong (missing API, incorrect assumption, dependency doesn't work as expected)
- TDD cycles aren't converging — 3+ failed attempts at making a test pass suggest a design problem, not a code problem
- Implementation reveals the plan was incomplete (unaccounted-for edge case, missing step)

Recovery procedure:
1. **Stop implementing.** Commit what works so far — don't discard partial progress.
2. **Document what went wrong.** Be specific: which step failed, what was discovered, why the plan doesn't hold.
3. **Return to Phase 2 (Plan).** Invoke `/lembas` with phase "Implement (partial)" to compact, then re-enter plan mode with the new information. Revise only the affected steps — don't replan from scratch.
4. **Get user approval** on the revised plan before resuming implementation.
5. If running as a fellowship teammate, message the lead with the blocker before replanning.

**Transition:** Invoke `/lembas` with phase "Implement" before moving to Review.

### Phase 4: Review

Goal: Convention compliance, code quality, and verified passing state before completion.

**Actions — three sequential steps:**

**Step 1: Convention review**
1. Invoke `/warden` to compare changes against reference files and conventions
2. Fix all BLOCKING issues identified
3. For ADVISORY issues, present to the user for decision

**Step 2: Code quality review**
1. Invoke `/pr-review-toolkit:review-pr` for comprehensive code quality analysis (silent failure hunting, type design, test coverage)
2. Address any critical or important issues identified
3. Re-run affected tests after fixes

**Step 3: Verification gate**
1. Invoke `superpowers:verification-before-completion` — run tests for affected package(s) only, confirm build passes, verify output matches expectations
2. Use the scope from Session Context to determine which test suites to run — in a monorepo, run only the affected package(s), not the entire suite
3. Do NOT claim work is complete until verification passes
4. If verification fails, fix and re-verify

**Output:** Summary of what was built, what was reviewed, verification results, and readiness for completion.

**Transition:** Invoke `/lembas` with phase "Review" before moving to Complete.

### Phase 5: Complete

Goal: Integrate the work — squash/merge, PR creation, worktree cleanup.

**Actions:**
1. Invoke `superpowers:finishing-a-development-branch` to present integration options
2. This skill handles: squash vs merge decision, PR creation, branch cleanup
3. If working in a worktree (from Phase 0), clean up the worktree after merge
4. **PR config:** If `config.pr.draft` is true, create the PR as a draft. If `config.pr.template` is set (a string), use it as the PR body template — the template can contain `{task}`, `{summary}`, and `{changes}` placeholders that get filled in with the actual values.

**Gate:** Phase 4 verification must have passed. Do not complete without a green verification step.

## Escape Hatch

Use the shortened cycle when ALL of these are true:
- Single file changed (or 2 files where one is a test)
- < 50 lines of new/modified code
- No new patterns introduced — you're following an existing pattern exactly
- Familiar area — you've seen the conventions (or CLAUDE.md documents them clearly)

If any condition is uncertain, run the full cycle.

Shortened cycle:
1. Quick research (read the relevant file)
2. Implement the change
3. `/warden`

## Key Principles

1. **Context is the bottleneck.** Compact between every phase. Don't let research noise pollute planning, or planning noise pollute implementation.
2. **Hard gates prevent drift.** Don't plan without understanding. Don't implement without a plan. Don't PR without review.
3. **Compose, don't rebuild.** This skill orchestrates existing skills (council, gather-lore, lembas, warden, review-pr, writing-plans, TDD, verification-before-completion, finishing-a-development-branch). It doesn't replace them.
4. **Human in the loop.** Plan approval is non-negotiable. The user guides direction; the agent handles execution.
5. **Frequent compaction.** When in doubt, compact. The cost of re-reading a file is low; the cost of degraded reasoning is high.
