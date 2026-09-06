---
name: fellowship
description: Multi-task orchestrator. Coordinates agent teammates (led by Gandalf) running /quest (code) or /scout (research) workflows. Use when you have multiple independent tasks to run in parallel.
---

# Fellowship — Multi-Quest Orchestrator

## Overview

Coordinates parallel teammates — quest runners and scouts — as named background agents (`Agent`, `SendMessage`, `ListAgents`, `TaskStop`). The session has one implicit team; the fellowship name lives in the store. The lead plays Gandalf, the coordinator who never writes code: it spawns teammates, routes gate approvals, delivers research findings, and reports progress. Quest teammates run the `/quest` lifecycle — Research → Plan → Implement → Review — in an isolated worktree and produce PRs. Scout teammates run `/scout` for research: no code, no PRs, no worktree.

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

`/fellowship` picks a name `fellowship-{timestamp}` and records it with `state init --name`. The lead enters coordinator mode, waiting for quests. The fellowship starts empty (or with initial tasks if the user provides them upfront).

### Add Quests and Scouts

The user adds tasks at any time:

```
User: "quest: fix auth bug #42"
User: "implement issues #42, #51, #67 with fellowship"
User: "scout: how does the auth middleware chain work?"
User: "scout: list all API endpoints and their rate limit configs → send to quest-rate-limit"
User: "group: API work — quest: add endpoint, quest: add tests, scout: review API docs"
```

**Groups** collect related quests and scouts for batch operations and progress tracking. A group is a reporting layer only — it does not change how quests execute.

### Pre-flight: Verify CWD

Run `git rev-parse --path-format=absolute --show-toplevel` and `git rev-parse --path-format=absolute --git-common-dir` (`--path-format=absolute` matters — without it, `--git-common-dir` prints a relative path like `.git` when you're already at the main root, which will never string-match an absolute top-level path). The main repo root is the PARENT of the common git dir. If your top-level does not equal the main repo root, you are inside a distinct worktree (a quest worktree, or any other) and Gandalf must not start here — stop and tell the user: "Error: Gandalf cannot start from inside a worktree (`<CWD>`). Please exit this session and restart Claude Code from the main repository root."

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

`state init` takes no `--dir` — it operates on the current directory, so run it from the repo root. Register each quest and scout — and, for a quest, its worktree — *before* spawning it: `fellowship init --dir <worktree>` resolves the quest from that registration, and gate hooks block an unregistered worktree.

`--claim-lead` records *this* session as the fellowship's lead. `state init` records a lead only when none is recorded yet, so on a repo that has run a fellowship before — a new session picking up an existing one — pass `--claim-lead` as well, or `worktree-guard` keeps treating the previous session as the lead and refuses this one's `Edit`/`Write` in the main tree. It only runs from the main working tree, and refuses a session that is already recorded against a quest.

```bash
~/.claude/fellowship/bin/fellowship state init --name <fellowship_name> [--base-branch <base_branch>] [--claim-lead]
~/.claude/fellowship/bin/fellowship state add-quest --dir <repo_root> --name <quest_name> --task "<task text>" [--branch <branch>] [--worktree <path>]
~/.claude/fellowship/bin/fellowship state add-scout --dir <repo_root> --name <scout_name> --question "<question>"
~/.claude/fellowship/bin/fellowship state add-group --dir <repo_root> --name <group_name> --quests q1,q2 --scouts s1
~/.claude/fellowship/bin/fellowship state update-quest --dir <repo_root> --name <quest_name> [--worktree <path>] [--branch <branch>] [--status active|completed|cancelled]
```

### Discover Templates

At startup, or when spawning a quest, discover templates from three directories, highest priority first: `.claude/fellowship-templates/` in the repo root, `~/.claude/fellowship-templates/`, and the plugin's own `plugin/skills/lorebook/templates/` (under `$latest_plugin_dir` from Ensure CLI). Parse each one's YAML frontmatter for `name`, `description`, and `keywords`. Selection: explicit (`template: <name>`) > keyword auto-suggest > none.

Fellowship ships one built-in, `example`. It carries no keywords, so it never auto-suggests — it is there to be read and copied. `/scribe` writes real ones.

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
4. Registers the verified path in the store *before* spawning —
   `fellowship state add-quest --worktree <path>` (or `state update-quest
   --worktree <path>`) — so the quest finds it and does not create a second
   one.

Two fail-closed backstops catch a mis-placed teammate regardless: its own
isolation self-check before its first write, and the `worktree-guard` hook.

See [resources/isolation.md](resources/isolation.md) for the full protocol.

### Spawn a Quest

For each quest, Gandalf:

1. Register the quest in the store (`state add-quest --dir <repo_root> --name <quest_name> --task "<task text>" --worktree <path>`) with the quest description

**Issue detection:** check the task description for GitHub issue references (`#\d+`) first. If any are found, invoke `/missive` with them, use its output for `{issue_context}` and its suggested branch name in place of the default slug, and spawn one quest per issue (each with its own missive output). With no references, `{issue_context}` is the empty string.

2. Spawn a teammate via the `Agent` tool with:
   - `name`: `"quest-{n}"` or a descriptive name like `"quest-auth-bug"` — this is how `SendMessage(to: "<quest_name>")` addresses it
   - `subagent_type: "general-purpose"`
   - `run_in_background: true`
   - no `isolation` parameter — the lead provisioned the worktree; the flag would create a second one and demote the teammate to a plain subagent
   - `model`: `config.models.quest` if set; otherwise omit — quest teammates write production code and inherit the session model by default
   - **Isolation:** the worktree the lead provisioned and verified in the
     pre-flight above, registered in the store before this spawn. See
     [resources/isolation.md](resources/isolation.md).
   - **Worktree path:** carried in the spawn prompt as `{worktree_path}` — the
     teammate's working directory stays the main repo root for its whole
     life, so it names this path in every `fellowship --dir` call and every
     file it reads or writes.

**Todo persistence:** After spawning, write initial todos via `~/.claude/fellowship/bin/fellowship todo init --dir <path> --quest <name> --task "description"`. Add todos to running quests: `~/.claude/fellowship/bin/fellowship todo add --dir <worktree> 'description'`.

**Spawn prompt:** See [resources/spawn-prompts.md](resources/spawn-prompts.md) for the full quest spawn prompt template and substitution rules.

### Spawn a Plan-Driven Quest

When the user's prompt references a plan file (e.g., "implement docs/plans/my-plan.md with fellowship"):

**Solo mode (one quest for the whole plan):** read the plan file to confirm it exists, spawn a teammate with the **Plan-Driven variant** from spawn-prompts.md, and register the quest in fellowship state as usual.

**Fan-out mode (multiple quests from one plan):** read the plan, propose groupings to the user ("I'd split this into 3 quests: ..."), wait for approval, then spawn each with the plan-driven prompt — every quest gets the full plan path plus instructions scoped to its own subset of tasks.

**Solo or fan-out?** Default to solo. Fan out when the plan has explicitly independent sections, when the user asks for parallel execution, or when it has 3+ tasks touching different file sets. When uncertain, ask.

### Spawn a Scout

For each scout, Gandalf:

1. Register the scout in the store (`state add-scout --dir <repo_root> --name <scout_name> --question "<question>"`)
2. Spawn via `Agent` tool with `name: "<scout_name>"`, `subagent_type: "fellowship:scout"`, `run_in_background: true`, no worktree isolation. Pass `model: config.models.scout` if set; otherwise omit (the scout agent definition defaults to sonnet).

**Spawn prompt:** See [resources/spawn-prompts.md](resources/spawn-prompts.md) for the scout spawn prompt template.

### Spawn Palantir

When `config.palantir.minQuests` or more quests are active (default 2) and `config.palantir.enabled` is true (default), spawn one palantir monitoring agent — `Agent(name: "palantir", subagent_type: "fellowship:palantir", run_in_background: true, model: config.models.palantir if set)` — never more than one per fellowship — and stop it with `TaskStop(task_id: "palantir")` when quests drop below the threshold. Reach it with `SendMessage(to: "palantir", message: "check")`. Pass `model: config.models.palantir` if set, else omit (the agent defaults to haiku; monitoring is read-only).

**Spawn prompt:** See [resources/spawn-prompts.md](resources/spawn-prompts.md) for the palantir spawn prompt template.

### Disband

On "wrap up" or "disband": `TaskStop(task_id: "<name>")` every teammate still running (quests, scouts, palantir) — an idle teammate needs nothing, nothing will message it again; summarize quests completed, PR URLs, and open items; clear the ephemeral discoveries with `~/.claude/fellowship/bin/fellowship notes clear`; offer `/retro` ("it identifies patterns across quests and can recommend configuration improvements") as a suggestion the user may skip. There is no team object to delete — the fellowship is the session's implicit team, and nothing is created or destroyed around it.

## Gate Handling

Each quest runs the full `/quest` lifecycle: **Research → Plan → Implement → Review**, with a gate leaving each of the first three. No gate leaves Review — a quest ends inside it, when the PR is opened and the task is marked complete. Gates are enforced by a state machine: project-level hooks block teammate tools based on phase and gate state, and only Gandalf can unblock a pending gate.

**DEFAULT: ALL gates surface to the user.** No gates are ever auto-approved unless `config.gates.autoApprove` explicitly lists them. Gandalf must NEVER auto-approve a gate that is not listed in `config.gates.autoApprove`.

**With `config.gates.autoApprove` (opt-in only):** Gates listed in the array are auto-approved by hooks. Valid gate names are the three phases a gate leaves: `"Research"`, `"Plan"`, `"Implement"`. `"Review"` is rejected — no gate leaves it.

### Surfacing a Gate

A gate that is not auto-approved is put to the user with `AskUserQuestion`, one gate per question, never a free-text "say approve". First show the teammate's `[GATE]` message (its Summary, Artifacts, Risks and Next Phase Needs, plus the hook's auto-generated Gate Context), then ask:

- `question`: `"<quest_name> submitted its <phase> gate. Approving advances it to <next_phase>."`
- `header`: `"Gate: <quest_name>"` (trimmed to fit)
- `options`: `"Approve"` — run the approval procedure; `"Reject with feedback"` — ask a follow-up `AskUserQuestion` for the feedback text (or take it from the "Other" answer), then run the rejection procedure; `"Hold"` — `fellowship hold --dir <worktree>` and tell the teammate.

A user answer typed in chat ("approve", "reject: use a table") is handled the same way. Never combine two quests' gates into one question, and never ask about a gate that is already decided.

### Gate Approval Procedure

1. Read the worktree path: `fellowship state show --json` (quests[].worktree) — or the path the lead itself provisioned
2. `~/.claude/fellowship/bin/fellowship gate approve --dir <worktree_path>`
3. `SendMessage(to: "<quest_name>", summary: "gate approved", message: "Gate approved — proceed to <next phase>. ...")` — this resumes the teammate with its context intact
4. Send palantir a "check" message if active; otherwise run
   `~/.claude/fellowship/bin/fellowship health --json` yourself — see
   "Health Monitoring Without Palantir" in
   [resources/lead-behavior.md](resources/lead-behavior.md)

### Gate Rejection Procedure

1. `~/.claude/fellowship/bin/fellowship gate reject --dir <worktree_path>`
2. SendMessage the feedback; the teammate addresses it, re-runs the prerequisites, and resubmits
3. Send palantir a "check" message if active; otherwise run
   `~/.claude/fellowship/bin/fellowship health --json` yourself — see
   "Health Monitoring Without Palantir" in
   [resources/lead-behavior.md](resources/lead-behavior.md)

## Conflict Resolution

When Palantir raises a file conflict alert, Gandalf follows the conflict resolution protocol: Pause (`~/.claude/fellowship/bin/fellowship hold --dir <worktree> [--reason "..."]`) → Assess (real vs incidental) → Resolve (sequence/partition/merge) → Resume (`~/.claude/fellowship/bin/fellowship unhold --dir <worktree>`).

See [resources/conflict-resolution.md](resources/conflict-resolution.md) for the full protocol.

## Lead Behavior

Gandalf's decision tree and event handling rules — reactive (teammate events), proactive (user commands), gate tracking, gate discipline, and Gandalf's voice.

See [resources/lead-behavior.md](resources/lead-behavior.md) for the full behavior specification.

## Progress Tracking

Status report format, phase-to-progress mappings, and how groups are reported.

See [resources/progress-tracking.md](resources/progress-tracking.md) for details.

## Edge Cases

- **Quest fails:** report to the user with context (which phase, what went wrong). Before respawning, preserve the failure for future quests with `~/.claude/fellowship/bin/fellowship failures infer --dir <worktree_path>`, which reconstructs a best-effort record from the quest's history, events, and health data. The worktree is preserved.
  - **Respawn:** spawn a new teammate with the same task description using the **Resume** variant of the quest spawn prompt (see [spawn-prompts.md](resources/spawn-prompts.md)) — it tells the teammate its worktree already exists and where its checkpoint is — with its working directory set to that worktree.
- **Direct teammate access:** through Gandalf ("tell quest-2 to skip the logger refactor"), or address the teammate by name yourself.
- **Session death:** worktrees survive, coordination does not. `/rekindle` is the recovery path: it scans, classifies, and respawns each quest from its `.fellowship/checkpoint.md`. To unstick a quest by hand: `~/.claude/fellowship/bin/fellowship gate reject --dir <worktree>`.

## Key Principles

1. **Coordinate, don't execute.** Gandalf never writes code — it spawns, routes, and reports.
2. **Compose over existing primitives.** Named background agents + quest + worktrees.
3. **Dynamic over static.** Accept quests anytime, not just at startup.
4. **Isolation by default.** Every quest gets its own worktree; no shared in-progress state.
5. **Human in the loop.** All gates surface to the user unless config opts specific ones out. Gandalf never merges PRs.
