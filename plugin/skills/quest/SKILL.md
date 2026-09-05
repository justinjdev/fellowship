---
name: quest
description: Use for multi-file or multi-step changes that need research and a plan — not single-file fixes under ~50 lines that follow an existing pattern (see Escape Hatch below). Runs the Research → Plan → Implement → Review lifecycle with a hard gate leaving each of the first three phases and context compaction between them.
---

# Quest — Research, Plan, Implement, Review

Four phases, three gates. A gate leaves Research, Plan, and Implement. Nothing
leaves Review: the quest ends when the PR is open and the task is marked
complete.

```
Research  ── orient, scan prior art, study the code ──[GATE]─→
Plan      ── explicit steps with file:line refs ─────[GATE]─→
Implement ── TDD, small commits, todo tracking ────[GATE]─→
Review    ── balrog → warden → review-pr → verify → PR. No gate.
```

`/lembas` runs between every phase and is a hook-enforced prerequisite for
each gate. `.fellowship/` below means the configured data directory —
`dataDir` in `~/.claude/fellowship.json` overrides it and every `fellowship`
command resolves it for you. Some steps invoke `superpowers` and
`pr-review-toolkit` skills; if one is not installed, do the step's goal by
hand rather than skipping it, and say so in your gate message.

## Gates (fellowship teammates)

Gate state lives in the fellowship database, enforced by hooks; you never
write it. Read it with `~/.claude/fellowship/bin/fellowship gate status`.
At the end of Research, Plan, and Implement, in this order:

1. Invoke `/lembas` (hooks verify this).
2. `TaskUpdate(taskId: "<your_task_id>", metadata: {"phase": "<current_phase>"})`
   (hooks verify this). Valid phase names: Research, Plan, Implement, Review.
3. SendMessage the lead a `[GATE]` message:

```
[GATE] <phase> complete

## Summary
<2-3 sentences: what was done this phase, key decisions made>

## Artifacts
- <file:lines> — <what/why>

## Risks
<any concerns for the next phase, or "None">

## Next Phase Needs
<what the next phase should focus on>
```

Steps 1 and 2 must both be done before step 3 or the hooks block the gate.
Then end your turn and wait for the lead.

The hooks also block Edit/Write outside `.fellowship/` during Research and
Plan (Bash, Agent, Skill, and reads are always allowed); require `[GATE]` at
the start of a line to detect a gate; block your tools between a submission
and the lead's decision; refuse a second gate while one is pending; and refuse
to mark the task completed unless the phase is Review with no gate pending.

## Phase 1: Research

Understand the system well enough to plan. Gather information; don't propose
solutions yet.

### Step 0 — resume check

This is the only place a quest looks for a checkpoint. If the spawn prompt has
a `RESUME CONTEXT:` block, or `.fellowship/checkpoint.md` exists:

1. You are already in your worktree — skip step 1 below.
2. `~/.claude/fellowship/bin/fellowship init --dir $(pwd)` clears
   `gate_pending` and keeps the phase. If a gate was pending when the previous
   session died the hooks block this — only the lead can clear it, so ask.
3. `TaskUpdate(taskId: "<task_id>", metadata: {"worktree_path": "<cwd>"})`.
4. Read `.fellowship/checkpoint.md` as your starting context in place of step
   2, and `fellowship history show --dir $(pwd)` for your phases, gates, and
   files touched.
5. Resume at the phase `fellowship gate status` reports, going straight there
   if it is past Research.

With no checkpoint, restart the current phase from scratch.

### Step 1 — worktree and state

Skip this whole step on resume, and see Plan-driven mode below if the spawn
prompt carries a `PRE-EXISTING PLAN:`.

1. **Config.** Read `.fellowship/config.json` (repo root) and
   `~/.claude/fellowship.json` if present and merge defaults → project → user
   (user wins; `/settings` documents the schema).
2. **Isolate.** If task metadata already has a `worktree_path` that exists on
   disk you are isolated — skip ahead. Otherwise, when `worktree.enabled` is
   true (the default):
   - **Branch name:** the name `/missive` suggested, if the spawn prompt has
     one. Else `branch.pattern` with `{slug}` (slugified task description),
     `{ticket}` (matched by `branch.ticketPattern`, default `[A-Z]+-\d+`) and
     `{author}` (`branch.author`) substituted — ask the user for any
     placeholder you cannot fill. Else `fellowship/{slug}`.
   - **All four steps are required:** (a) resolve the base SHA (`git rev-parse
     <base_branch>` if the spawn prompt names one, else `git rev-parse HEAD`)
     into your response text, not a shell variable; (b) `EnterWorktree` with
     that branch name, honoring `worktree.directory` if set; (c)
     **immediately** `git reset --hard <sha>` — `EnterWorktree` bases off the
     default branch, so without this the worktree starts from the wrong point;
     (d) `pwd` to confirm you are in your own worktree, since parallel spawns
     can race your CWD into a sibling's; `cd` to the path `EnterWorktree`
     printed if not.
3. **Quest state (fellowship only).** Before any other tool call, so hooks can
   enforce from the start:
   ```bash
   ~/.claude/fellowship/bin/fellowship init --dir <worktree_path>
   ```
   using the verified path from step 2, not your CWD. It creates state at
   Research (or, on a respawn, resets `gate_pending` and keeps the phase),
   resolves your quest name from the worktree the lead registered (override
   with `--quest <name>`), and loads `gates.autoApprove` from the merged
   config. Confirm with `fellowship gate status --dir <worktree_path>`, then
   `TaskUpdate(taskId: "<task_id>", metadata: {"worktree_path": "<cwd>"})`.

### Step 2 — orient

Read the root CLAUDE.md for this area's reference files, review conventions,
and architecture constraints; if there is none, note that `/chronicle` would
set one up. In a monorepo (`packages/`, `apps/`, or a workspace config),
identify the affected package(s) and merge any package-level CLAUDE.md over
the root one — a single-package repo is its own scope. Then use Explore agents
(Agent tool, `subagent_type=Explore`, `model: "haiku"`, or `models.explore` if
config sets it) to find 5–10 key files within that scope. Produce:

```
## Session Context

**Task:** [one line]

**Package(s):** [name(s) and path(s)]

**Key Files:**
- [path:lines] — [why it's relevant]

**Relevant Conventions:**
- [from root and package CLAUDE.md]

**Architecture Notes:**
- [constraints, patterns, dependencies; note non-default fellowship config]

**Out of Scope:**
- [what not to touch]
```

Every later phase and `/lembas` carry this block forward. As a teammate you
have no direct user: route blocking questions to the lead via SendMessage and
otherwise proceed on documented assumptions.

### Step 3 — prior art

Past failures in this area, then what sibling quests have found. Either
`notes scan` flag alone is fine; use both once you know both.

```bash
~/.claude/fellowship/bin/fellowship failures scan --dir <main_repo> --files "<files>" --modules "<modules>"
~/.claude/fellowship/bin/fellowship notes scan --topics "<topics>" --files "<files>"
```

### Step 4 — study the code

Read the key files. For each area whose conventions you do not already know —
CLAUDE.md's Reference Files section names the ones that matter — study a
reference file and extract, exhaustively: **structure** (ordering, imports,
exports), **dependencies** (how internal and external ones are reached, any DI
pattern), **error handling** (types, propagation, how errors surface), **data
flow** (validation, transformation, return shape), **naming**, and **what it
conspicuously does not do**. Absences matter as much as presences; carry both
into the plan. With no reference files documented, ask the user for one or two
examples a reviewer would approve, or find recently added approved work with
`git log --oneline --diff-filter=A -- <dir> | head -10`. Offer to add any
convention the study turns up that CLAUDE.md does not record.

Document how the system works today, its constraints, and its edge cases.

### Promoted quests

With a `PROMOTED FROM:` block in the spawn prompt, write the
`SCOUT FINDINGS CONTENT:` to `.fellowship/scout-findings-{scout_name}.md` so
it survives compaction, then judge whether the findings are about this task's
system at all. If not, research normally. If so, spot-check the key claims
against the referenced files, flag anything stale, and fill the gaps a scout
does not cover: write targets, test locations and patterns, build/lint/test
commands. The gate below is unchanged.

**Gate — Research must produce:** key files with line ranges; documented
constraints and dependencies; the current behavior understood, not just
located. If any is missing, keep researching.

## Phase 2: Plan

Enter plan mode (EnterPlanMode), or `superpowers:writing-plans` for a formal
plan. Write steps that name specific files and line ranges from Research,
define the test strategy, and note whether the plan has 2+ independent
workstreams.

**Gate — Plan must include:** explicit file paths and line ranges for every
change; a test strategy; the user's approval of the plan.

## Phase 3: Implement

Execute the plan in small verifiable steps. TDD by default.

**Todos** are the source of truth for remaining work, not the original
prompt. `fellowship todo list --dir .` shows the checklist;
`fellowship todo update --dir . <id> <status>` moves one along, through
`pending`, `in_progress`, `done`, `blocked`, `skipped`.

**Single-stream (default):** invoke `superpowers:test-driven-development` and
work the plan step by step — failing test, minimal implementation, refactor —
verifying and committing each logical unit.

**Parallel subagents:** only when the plan has 3+ independent tasks touching
disjoint files. Dispatch them in one message, each with the full task text,
context, and TDD instructions, then review and commit the results. No two may
touch the same file — a planning constraint, not a runtime guard; if the plan
has such a conflict, fix the plan.

Use conventional commits and verify as you go rather than batching it. Post to
the notes board whenever you find something a sibling quest would want —
refactors, API changes, infrastructure shifts, gotchas, deprecations, findings
about shared code — but not phase progress or edit intentions:

```bash
~/.claude/fellowship/bin/fellowship notes post --quest "<quest>" --topic "<topic>" --files "<files>" --discovery "<text>"
```

**Recovery.** Trigger it when a plan step is impossible or wrong, when 3+ TDD
cycles fail to converge (a design problem, not a code problem), or when
implementation reveals the plan was incomplete. Stop and commit what works,
document which step failed and why the plan does not hold, record it:

```bash
echo '{"quest":"<quest>","task":"<task>","phase":"Implement","trigger":"recovery","files":["<files>"],"modules":["<modules>"],"what_failed":"<specific>","resolution":"<what changed>","tags":["<tags>"]}' | ~/.claude/fellowship/bin/fellowship failures create --dir <main_repo>
```

Then `/lembas` with phase "Implement (partial)", message the lead with the
blocker, re-enter plan mode, revise only the affected steps, and get approval
before resuming.

**Gate — Implement must produce:** the plan executed or a documented recovery;
tests passing for what you changed; each logical unit committed.

## Phase 4: Review

Attack it, check it against the conventions, verify it, ship it. No gate
leaves Review — the quest ends at step 5.

**1. Adversarial (balrog).** Spawn the balrog agent via the Agent tool with
the worktree path, the task description, and your teammate name (balrog
reports back via SendMessage, addressed by name; standalone, tell it to
present findings directly). Pass `model` only if `models.balrog` is set —
adversarial review is not the place to economize unless the user opted in. If
balrog never reports back, or the Agent tool errors, continue and say so in
your completion report. Fix every **Critical/High** finding and confirm each
against balrog's reproduction steps; put **Medium** findings to the user and
record the decision; log **Low** ones without blocking.

**2. Conventions.** Invoke `/warden`. Fix every BLOCKING issue; put ADVISORY
ones to the user.

**3. Code quality.** Invoke `/pr-review-toolkit:review-pr`. Address the
critical and important findings, then re-run the affected tests.

**4. Verification.** Invoke `superpowers:verification-before-completion`. Run
the tests for the affected package(s) only — the Session Context says which —
confirm the build, and check the output matches expectations. Don't claim the
work is done until this passes; if it fails, fix and re-verify.

**5. Ship.** Invoke `superpowers:finishing-a-development-branch` for the
squash/merge decision, PR creation, and branch cleanup. Draft the PR if
`pr.draft` is set; use `pr.template` as the body if set (it takes `{task}`,
`{summary}`, `{changes}`); include any `/missive` issue keywords (e.g.
`Closes #42`). Clean up the worktree after the merge, then report what was
built, what review found, the verification results, and the PR link. Marking
the task complete ends the quest — the hooks allow it only from Review with no
gate pending, and the history is marked completed for you.

## Plan-driven mode

With `PRE-EXISTING PLAN:` and a path in the spawn prompt: provision the
worktree exactly as in Research step 1.2 (`git reset --hard` and CWD check
included), copy the plan file to `.fellowship/plan.md` in the worktree, then

```bash
~/.claude/fellowship/bin/fellowship init --dir <worktree_path> --phase Implement --plan-skip
~/.claude/fellowship/bin/fellowship todo init --dir <worktree_path>
~/.claude/fellowship/bin/fellowship todo add --dir <worktree_path> "<task>"   # one per plan task
```

`--plan-skip` starts the quest at Implement and records Research and Plan as
skipped in the history. Skip both phases entirely and work `.fellowship/plan.md`
as your blueprint. One gate remains: Implement.

## Escape hatch

Skip the lifecycle only when **all** of these hold: one file changes (or two,
one a test); under ~50 lines; no new pattern — you are following an existing
one exactly; and a familiar area, or one CLAUDE.md documents clearly. If any
is uncertain, run the full lifecycle. The short version: read the relevant
file, make the change, run `/warden`.

## Key principles

- **Context is the bottleneck.** Compact between every phase; when in doubt,
  compact. Re-reading a file is cheap, degraded reasoning is not.
- **Hard gates prevent drift.** No planning without understanding, no
  implementing without a plan, no PR without review.
- **Compose, don't rebuild.** This skill orchestrates existing skills.
- **Human in the loop.** Plan approval is non-negotiable.
