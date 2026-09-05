---
name: fellowship
description: Multi-task orchestrator. Coordinates agent teammates (led by Gandalf) running /quest (code) or /scout (research) workflows. Use when you have multiple independent tasks to run in parallel.
---

# Fellowship — Multi-Quest Orchestrator

## Overview

Coordinates parallel teammates — quest runners and scouts — over the agent teams API (`TeamCreate`, `SendMessage`, `TaskCreate`, `TaskUpdate`, `TeamDelete`). The lead plays Gandalf, the coordinator who never writes code: it spawns teammates, routes gate approvals, delivers research findings, and reports progress. Quest teammates run the `/quest` lifecycle — Research → Plan → Implement → Review — in an isolated worktree and produce PRs. Scout teammates run `/scout` for research: no code, no PRs, no worktree.

## When to Use

- 2+ independent tasks (quests, scouts, or a mix) that share no in-progress state
- You want parallel execution with isolation and coordination, or research running alongside active code quests

## Lifecycle

### Ensure CLI

Before anything else, install the CLI binary (idempotent — a no-op when the right version is already present). Resolving the most-recently-installed version directory avoids ambiguity when several are cached:

```bash
latest_plugin_dir="$(ls -dt ~/.claude/plugins/cache/justinjdev/fellowship/* 2>/dev/null | head -n1)"
"$latest_plugin_dir/plugin/hooks/scripts/ensure-binary.sh"
```

The binary is then at `~/.claude/fellowship/bin/fellowship`. Use that full path for every CLI call in this session; do not rely on PATH. If the script fails, stop and tell the user: "Failed to install the `fellowship` CLI binary. Check your internet connection or reinstall the plugin." Do not proceed without it.

### Start

`/fellowship` creates the fellowship team via `TeamCreate` with name `fellowship-{timestamp}`. The lead enters coordinator mode, waiting for quests. The fellowship starts empty (or with initial tasks if the user provides them upfront).

### Add Quests and Scouts

The user adds tasks at any time:

```
User: "quest: fix auth bug #42"
User: "implement issues #42, #51, #67 with fellowship"
User: "scout: how does the auth middleware chain work?"
User: "scout: list all API endpoints and their rate limit configs → send to quest-rate-limit"
User: "company: API work — quest: add endpoint, quest: add tests, scout: review API docs"
```

**Companies** group related quests and scouts for batch operations and progress tracking. A company is a reporting layer only — it does not change how quests execute.

### Pre-flight: Verify CWD

Run `pwd`. If the path contains `.claude/worktrees` you are inside a quest worktree and Gandalf must not start here — stop and tell the user: "Error: Gandalf cannot start from inside a quest worktree (`<CWD>`). Please exit this session and restart Claude Code from the main repository root."

### Load Config

Read both config layers (neither need exist): the project config at `.fellowship/config.json` (repo root) and the user config at `~/.claude/fellowship.json`. Merge **defaults → project → user** (user always wins); `/settings` holds the full schema. Fellowship reads `branch.*`, `worktree.*`, `gates.autoApprove`, `pr.*`, `palantir.*`, and `models.*`.

**Model routing:** for each spawn below, check `config.models.<role>` (`quest`, `scout`, `palantir`). A model alias (`haiku`, `sonnet`, `opus`) is passed as the Agent tool's `model` parameter. Unset, `null`, or `"inherit"` means *omit the parameter* — it accepts neither `"inherit"` nor full model IDs, and omission is how inheritance is spelled. Omitted, the agent definition's own default applies (scout: sonnet, palantir: haiku) and quest teammates inherit the session model. A per-invocation `model` overrides the definition's frontmatter, so config always wins when set.

**IMPORTANT — gate defaults:** with no config file, or `gates.autoApprove` absent or empty, ALL gates surface to the user. Gandalf must NEVER tell a teammate a gate is auto-approved unless `gates.autoApprove` explicitly lists it.

### Detect Base Branch

Run `git branch --show-current`.

- **Detached HEAD** (empty output): probe with `git symbolic-ref refs/remotes/origin/HEAD --short 2>/dev/null | sed 's|origin/||'`; failing that, `main` if `git rev-parse --verify main` succeeds, else `master`.
- **On `main` or `master`:** use it, no confirmation needed.
- **Any other branch:** confirm with `AskUserQuestion` — "Quest worktrees will be based off '<branch>'. Is that correct?", options `["Yes, use <branch>", "No, use main instead", "Use a different branch"]`, prompting for a name on the third.

Then run `git status --porcelain`. On uncommitted changes, warn: "Warning: you have uncommitted changes in your working tree. Quest worktrees will be created from the branch tip and will not include these changes. Continue anyway?" with `["Continue", "Abort"]`; stop if they abort.

Store the result as `base_branch` — every quest spawn prompt carries it so worktrees start from the right commit.

### Write Fellowship State

> **Note:** `.fellowship/` is the default data directory. Users can override it via `dataDir` in `~/.claude/fellowship.json`. All `fellowship` CLI commands resolve the correct directory automatically.

`state init` takes no `--dir` — it operates on the current directory, so run it from the repo root. Register each quest and scout after spawning it, and fill in the worktree path once task metadata carries one:

```bash
~/.claude/fellowship/bin/fellowship state init --name <fellowship_name> [--base-branch <base_branch>]
~/.claude/fellowship/bin/fellowship state add-quest --dir <repo_root> --name <quest_name> --task "<task text>" [--branch <branch>] [--task-id <id>]
~/.claude/fellowship/bin/fellowship state add-scout --dir <repo_root> --name <scout_name> --question "<question>" [--task-id <id>]
~/.claude/fellowship/bin/fellowship state add-company --dir <repo_root> --name <company_name> --quests q1,q2 --scouts s1
~/.claude/fellowship/bin/fellowship state update-quest --dir <repo_root> --name <quest_name> [--worktree <path>] [--branch <branch>] [--task-id <id>]
```

### Discover Templates

At startup, or when spawning a quest, discover templates from `.claude/fellowship-templates/` in the repo root and `~/.claude/fellowship-templates/`, project winning on a name collision. Parse each one's YAML frontmatter for `name`, `description`, and `keywords`. Selection: explicit (`template: <name>`) > keyword auto-suggest > none. Fellowship ships one example template; `/scribe` writes the rest.

### Isolation Pre-flight (REQUIRED before spawning)

Provisioning a quest's worktree is the **lead's** job. The Agent tool's
`isolation: "worktree"` parameter has been observed to silently no-op, and a
teammate left to provision its own can fail just as quietly — either way the
quest lands in the shared main tree. So before spawning any quest, Gandalf:

1. Confirms `state init` registered the `worktree-guard` hook in the project's
   git-ignored `.claude/settings.local.json` (its output says so), that
   `~/.claude/fellowship/bin/fellowship version` succeeds, and that a
   fellowship has been initialized. The guard is inert outside a fellowship,
   so installing it is always safe.
2. Runs `git worktree add -b <branch> <path> <base>` itself with `<path>`
   outside the main tree, provisions dependencies so the teammate's tests run,
   and copies `.claude/settings.local.json` into the new worktree so the guard
   is armed there too.
3. Verifies with `git worktree list` that the worktree exists and is not the
   main root — never tell a teammate it is "already isolated" otherwise.
4. Publishes the verified path with
   `TaskUpdate(taskId: "<task_id>", metadata: {"worktree_path": "<path>"})`
   *before* spawning, so the quest finds it and does not create a second one.

Two fail-closed backstops catch a mis-placed teammate regardless: its own
isolation self-check before its first write, and the `worktree-guard` hook.

See [resources/isolation.md](resources/isolation.md) for the full protocol.

### Spawn a Quest

For each quest, Gandalf:

1. `TaskCreate` in the shared task list with the quest description

**Issue detection:** check the task description for GitHub issue references (`#\d+`) first. If any are found, invoke `/missive` with them, use its output for `{issue_context}` and its suggested branch name in place of the default slug, and spawn one quest per issue (each with its own missive output). With no references, `{issue_context}` is the empty string.

2. Spawn a teammate via the `Agent` tool with:
   - `team_name`: the fellowship team name
   - `subagent_type: "general-purpose"`
   - `name`: `"quest-{n}"` or a descriptive name like `"quest-auth-bug"`
   - `model`: `config.models.quest` if set; otherwise omit — quest teammates write production code and inherit the session model by default
   - **Isolation:** the worktree the lead provisioned and verified in the
     pre-flight above, published to task metadata before this spawn. See
     [resources/isolation.md](resources/isolation.md).

**Errand persistence:** After spawning, write initial errands via `~/.claude/fellowship/bin/fellowship errand init --dir <path> --quest <name> --task "description"`. Add errands to running quests: `~/.claude/fellowship/bin/fellowship errand add --dir <worktree> 'description'`.

**Spawn prompt:** See [resources/spawn-prompts.md](resources/spawn-prompts.md) for the full quest spawn prompt template and substitution rules.

### Spawn a Plan-Driven Quest

When the user's prompt references a plan file (e.g., "implement docs/plans/my-plan.md with fellowship"):

**Solo mode (one quest for the whole plan):** read the plan file to confirm it exists, `TaskCreate` with a description naming it, spawn a teammate with the **Plan-Driven variant** from spawn-prompts.md, and register the quest in fellowship state as usual.

**Fan-out mode (multiple quests from one plan):** read the plan, propose groupings to the user ("I'd split this into 3 quests: ..."), wait for approval, then spawn each with the plan-driven prompt — every quest gets the full plan path plus instructions scoped to its own subset of tasks.

**Solo or fan-out?** Default to solo. Fan out when the plan has explicitly independent sections, when the user asks for parallel execution, or when it has 3+ tasks touching different file sets. When uncertain, ask.

### Spawn a Scout

For each scout, Gandalf:

1. `TaskCreate` with the question and type "scout"
2. Spawn via `Agent` tool with `subagent_type: "fellowship:scout"`, no worktree isolation. Pass `model: config.models.scout` if set; otherwise omit (the scout agent definition defaults to sonnet).

**Spawn prompt:** See [resources/spawn-prompts.md](resources/spawn-prompts.md) for the scout spawn prompt template.

### Spawn Palantir

When `config.palantir.minQuests` or more quests are active (default 2) and `config.palantir.enabled` is true (default), spawn one palantir monitoring agent — never more than one per fellowship — and shut it down when quests drop below the threshold. Pass `model: config.models.palantir` if set, else omit (the agent defaults to haiku; monitoring is read-only).

**Spawn prompt:** See [resources/spawn-prompts.md](resources/spawn-prompts.md) for the palantir spawn prompt template.

### Disband

On "wrap up" or "disband": send `shutdown_request` to every active teammate including palantir; summarize quests completed, PR URLs, and open items; clear the ephemeral discoveries with `~/.claude/fellowship/bin/fellowship bulletin clear`; offer `/retro` ("it identifies patterns across quests and can recommend configuration improvements") as a suggestion the user may skip; then `TeamDelete`.

## Gate Handling

Each quest runs the full `/quest` lifecycle: **Research → Plan → Implement → Review**, with a gate leaving each of the first three. No gate leaves Review — a quest ends inside it, when the PR is opened and the task is marked complete. Gates are enforced by a state machine: project-level hooks block teammate tools based on phase and gate state, and only Gandalf can unblock a pending gate.

**DEFAULT: ALL gates surface to the user.** No gates are ever auto-approved unless `config.gates.autoApprove` explicitly lists them. Gandalf must NEVER auto-approve a gate that is not listed in `config.gates.autoApprove`.

**With `config.gates.autoApprove` (opt-in only):** Gates listed in the array are auto-approved by hooks. Valid gate names are the three phases a gate leaves: `"Research"`, `"Plan"`, `"Implement"`. `"Review"` is rejected — no gate leaves it.

### Gate Approval Procedure

1. Read the worktree path: `TaskGet(taskId)` → `metadata.worktree_path`
2. `~/.claude/fellowship/bin/fellowship gate approve --dir <worktree_path>`
3. SendMessage the teammate that it is approved

### Gate Rejection Procedure

1. `~/.claude/fellowship/bin/fellowship gate reject --dir <worktree_path>`
2. SendMessage the feedback; the teammate addresses it, re-runs the prerequisites, and resubmits

## Conflict Resolution

When Palantir raises a file conflict alert, Gandalf follows the conflict resolution protocol: Pause (`~/.claude/fellowship/bin/fellowship hold --dir <worktree> [--reason "..."]`) → Assess (real vs incidental) → Resolve (sequence/partition/merge) → Resume (`~/.claude/fellowship/bin/fellowship unhold --dir <worktree>`).

See [resources/conflict-resolution.md](resources/conflict-resolution.md) for the full protocol.

## Lead Behavior

Gandalf's decision tree and event handling rules — reactive (teammate events), proactive (user commands), gate tracking, gate discipline, and Gandalf's voice.

See [resources/lead-behavior.md](resources/lead-behavior.md) for the full behavior specification.

## Progress Tracking

Status report format, phase-to-progress mappings, and company grouping.

See [resources/progress-tracking.md](resources/progress-tracking.md) for details.

## Edge Cases

- **Quest fails:** report to the user with context (which phase, what went wrong). Before respawning, preserve the failure for future quests with `~/.claude/fellowship/bin/fellowship autopsy infer --dir <worktree_path>`, which reconstructs a best-effort record from the quest's tome, herald, and eagles data. The worktree is preserved.
  - **Respawn:** spawn a new teammate with the same task description using the **Resume** variant of the quest spawn prompt (see [spawn-prompts.md](resources/spawn-prompts.md)) — it tells the teammate its worktree already exists and where its checkpoint is — with its working directory set to that worktree.
- **Direct teammate access:** through Gandalf ("tell quest-2 to skip the logger refactor"), or Shift+Down to message the teammate directly.
- **Session death:** worktrees survive, coordination does not. `/rekindle` is the recovery path: it scans, classifies, and respawns each quest from its `.fellowship/checkpoint.md`. To unstick a quest by hand: `~/.claude/fellowship/bin/fellowship gate reject --dir <worktree>`.

## Key Principles

1. **Coordinate, don't execute.** Gandalf never writes code — it spawns, routes, and reports.
2. **Compose over existing primitives.** Agent teams + quest + worktrees.
3. **Dynamic over static.** Accept quests anytime, not just at startup.
4. **Isolation by default.** Every quest gets its own worktree; no shared in-progress state.
5. **Human in the loop.** All gates surface to the user unless config opts specific ones out. Gandalf never merges PRs.
