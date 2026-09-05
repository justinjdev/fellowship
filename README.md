<img width="1512" height="982" alt="Screenshot 2026-02-27 at 14 55 56" src="https://github.com/user-attachments/assets/a4fb319e-20ca-4bba-8595-134cbd06f4b6" />

# Fellowship

A Claude Code plugin that orchestrates multi-task workflows through a structured research-plan-implement-review lifecycle. Named after the obvious — a fellowship of agents, each on their own quest, coordinated by a wizard who never writes code.

## What It Does

Fellowship gives Claude Code a disciplined workflow engine. Instead of diving straight into code, a task goes through four phases with a hard gate leaving each of the first three: research the system, plan the changes, implement with TDD, then review — an adversarial pass, conventions, verification, and the PR.

For multiple independent tasks, it spins up parallel agent teammates — each in an isolated git worktree — coordinated by a lead agent (Gandalf) who routes approvals and reports progress.

## Install

From within Claude Code, run these as **two separate commands**:

```
/plugin marketplace add justinjdev/claude-plugins
```
```
/plugin install fellowship@justinjdev
```

### Dependencies

Fellowship's `/quest` skill orchestrates skills from these plugins. Install them for the full workflow:

| Plugin | Skills used | Phase |
|--------|------------|-------|
| **superpowers** | `writing-plans`, `test-driven-development`, `verification-before-completion`, `finishing-a-development-branch` | Plan, Implement, Review |
| **pr-review-toolkit** | `review-pr` | Review |

These are referenced by name in skill prompts. If a dependency isn't installed, Claude performs the step's goal manually and notes the substitution in its output — but you lose the structured discipline the dedicated skill provides.

```
/plugin marketplace add obra/superpowers-marketplace
/plugin install superpowers@superpowers-marketplace
/plugin install pr-review-toolkit@claude-plugins-official
```

#### System Dependencies

- **Go CLI binary** — gate enforcement hooks use a Go binary that is automatically downloaded from GitHub releases on first use, with its checksum verified against the release's `checksums.txt` before it's installed. No manual installation required.

### Project Setup (Optional)

Add this hook to `.claude/settings.local.json` in repos where you use fellowship. It prints a one-line hint when a `/lembas` checkpoint from a previous session is lying around, so you know recovery is available:

```json
{
  "hooks": {
    "SessionStart": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "if [ -f .fellowship/checkpoint.md ]; then echo \"fellowship: a checkpoint from a previous session is present at .fellowship/checkpoint.md — run /rekindle to recover the fellowship, or /quest to resume a single quest.\"; fi"
          }
        ]
      }
    ]
  }
}
```

It is a convenience only: it prints the hint and nothing else. Resuming is always something you ask for — see [Resuming after a crash](#resuming-after-a-crash).

Also add the fellowship data directory to your `.gitignore` — checkpoints and state are local ephemeral files. Keep `.fellowship/config.json` trackable, though: it holds committable team-shared settings (see `/settings`):

```gitignore
.fellowship/*
!.fellowship/config.json
```

If you have configured a custom `dataDir` in `~/.claude/fellowship.json`, use that directory name instead.

### Configuration (Optional)

Create `~/.claude/fellowship.json` in your personal Claude directory to customize fellowship behavior across all projects. All settings are optional — missing keys use sensible defaults that match the out-of-box behavior.

```json
{
  "dataDir": ".fellowship",
  "branch": {
    "pattern": null,
    "author": null,
    "ticketPattern": "[A-Z]+-\\d+"
  },
  "worktree": {
    "enabled": true,
    "directory": null
  },
  "gates": {
    "autoApprove": []
  },
  "pr": {
    "draft": false,
    "template": null
  },
  "palantir": {
    "enabled": true,
    "minQuests": 2
  },
  "issues": {
    "autoClose": true
  },
  "failures": {
    "expiryDays": 90
  },
  "models": {
    "quest": null,
    "scout": null,
    "palantir": null,
    "balrog": null,
    "explore": null,
    "validator": null
  }
}
```

| Setting | Default | Description |
|---------|---------|-------------|
| `dataDir` | `".fellowship"` | Directory name for fellowship working files (state, checkpoints, todos, history). Created inside each worktree and the main repo root. |
| `branch.pattern` | `null` | Branch name template with placeholders: `{slug}` (task description), `{ticket}` (extracted from description), `{author}` (from config). When `null`, defaults to `"fellowship/{slug}"`. |
| `branch.author` | `null` | Static value for the `{author}` placeholder. If not set and pattern uses `{author}`, you'll be prompted. |
| `branch.ticketPattern` | `"[A-Z]+-\\d+"` | Regex to extract ticket IDs from quest descriptions. Default matches Jira-style IDs (e.g., `PROJ-123`). |
| `worktree.enabled` | `true` | Whether quests create isolated worktrees. Set to `false` to work on the current branch. |
| `worktree.directory` | `null` | Parent directory for worktrees. `null` uses Claude Code's default (`.claude/worktrees/`). |
| `gates.autoApprove` | `[]` | Gate names to auto-approve: `"Research"`, `"Plan"`, `"Implement"` (the phase being left — `"Research"` auto-approves Research→Plan). `"Review"` is not a valid entry: it is the last phase and no gate leaves it. `fellowship init` reads the merged value when it creates a quest's state and fails with a clear error on an unknown phase name. Gates not listed still surface to you for approval. |
| `pr.draft` | `false` | Create PRs as drafts. |
| `pr.template` | `null` | PR body template string. Supports `{task}`, `{summary}`, and `{changes}` placeholders. |
| `palantir.enabled` | `true` | Whether to spawn a palantir monitoring agent during fellowships. |
| `palantir.minQuests` | `2` | Minimum active quests before palantir is spawned. |
| `issues.autoClose` | `true` | When true, `/missive` includes `Closes #N` in PR keywords so issues close on merge. |
| `failures.expiryDays` | `90` | Days before a quest failure record expires and is eligible for cleanup. |
| `models.quest` | `null` | Model for quest teammates. Valid values: `"haiku"`, `"sonnet"`, `"opus"` (aliases only — spawn parameters accept neither `"inherit"` nor full model IDs; leave `null` to inherit). `null` = built-in default: inherit the session model. |
| `models.scout` | `null` | Model for scout teammates. Same valid values. `null` = built-in default: `sonnet`. |
| `models.palantir` | `null` | Model for the palantir monitor. Same valid values. `null` = built-in default: `haiku`. |
| `models.balrog` | `null` | Model for balrog adversarial review. Same valid values. `null` = built-in default: inherit the session model. |
| `models.explore` | `null` | Model for Explore scan subagents spawned by quest, scout, council, and guide. Same valid values. `null` = built-in default: `haiku`. |
| `models.validator` | `null` | Model for scout's validation subagent. Same valid values. `null` = built-in default: `sonnet`. |

The config is read at fellowship startup and at the start of a quest's Research phase. Changes to the file take effect on the next fellowship or quest invocation.

## Skills

Skills are invoked automatically by Claude as part of a workflow (quest phases, context compression, etc.) — you can also invoke any of them directly with `/name`.

| Skill | Purpose |
|-------|---------|
| `/quest` | Full Research → Plan → Implement → Review lifecycle for non-trivial tasks. The hub that orchestrates everything else. |
| `/fellowship` | Multi-task orchestrator. Spawns parallel agent teammates running `/quest` (code) or `/scout` (research). |
| `/scout` | Research & analysis workflow. Investigates questions, optionally validates with a fresh adversarial subagent. No code, no PRs, no commits. |
| `/council` | Context-aware onboarding you invoke yourself. Loads task-relevant files, conventions, and architecture at session start. Quest inlines the same orientation, so it does not call this. |
| `/gather-lore` | Studies reference files to extract conventions before writing code, on request. Quest inlines the same extraction, so it does not call this. |
| `/lembas` | Context compression between phases. Writes a checkpoint and continues from it, keeping the context window in the reasoning sweet spot. |
| `/warden` | Pre-PR convention review. Compares changes against reference files and documented patterns. |
| `/missive` | Fetches GitHub issue context for quest spawning — title, body, labels, comments, branch suggestions, and PR close keywords. |
| `/retro` | Post-fellowship retrospective. Analyzes gate history, palantir alerts, and quest metrics, then recommends configuration improvements. |
| `/lorebook` | Loads phase-specific guidance from an assigned quest template at the start of each quest phase. Ships one built-in example template to copy. |

## Commands

Commands are user-invoked only — Claude never calls them automatically, so they carry no base context cost.

| Command | Purpose |
|---------|---------|
| `/chronicle` | One-time codebase bootstrapping. Walks through your project to extract conventions into CLAUDE.md. |
| `/dashboard` | Starts the live web dashboard for the current fellowship — quest/scout progress, gate approvals, event history — in the background, and prints the URL. |
| `/guide` | Interactive, learn-by-doing walkthrough of fellowship using a real task on your codebase. |
| `/red-book` | Post-PR convention capture. Extracts conventions from reviewer comments and adds them to CLAUDE.md. |
| `/rekindle` | Recovers a fellowship after a session crash — scans worktrees and quest state, then re-spawns Gandalf with recovered context. |
| `/scribe` | Creates a reusable quest template that encodes project-specific rules and conventions into phase guidance. |
| `/settings` | View or edit fellowship settings (`~/.claude/fellowship.json`). Interactive setup for all configuration options. |

## Agents

| Agent | Role |
|-------|------|
| **palantir** | Background monitor during fellowship execution. A thin reporter over the CLI's health sweep (`fellowship health` — the same stalled/zombie/struggling classification `events --problems` and the dashboard read) plus scope-drift and file-conflict checks read from each quest's history. Reports to Gandalf. Defaults to the `haiku` model. |
| **balrog** | Adversarial validation agent spawned by quest as the first step of Review. Analyzes the diff for failure modes, writes and runs targeted test cases, and delivers a severity-ranked findings report. |
| **scout** | Research & analysis agent spawned as a fellowship teammate for read-only investigation — no code edits, no git operations. Defaults to the `sonnet` model. |
| **validator** | Read-only adversarial validator spawned by scout to verify research findings against the actual code (CONFIRMED/CONTESTED/UNVERIFIED). Tools restricted to Read/Glob/Grep. Defaults to the `sonnet` model. |

## How It Works

**Single task** — run `/quest`:

Four phases, three gates. A gate leaves Research, Plan, and Implement; nothing leaves Review — the quest ends inside it, when the PR is open and the task is marked complete.

```
Research  → worktree + orientation, prior art, explore agents, convention study
            ──[GATE]─→
Plan      → plan mode with file:line references + user approval
            ──[GATE]─→
Implement → TDD (red-green-refactor), todo tracking
            ──[GATE]─→
Review    → balrog attacks the implementation (edge cases, error paths)
            → /warden conventions → code quality → verification
            → PR creation + worktree cleanup
```

`/lembas` compacts context between every phase.

**Research** — run `/scout`:

```
Investigate → (Validate) → Deliver
```

Autonomous research with confidence levels. For complex questions, spawns a fresh validator subagent to adversarially verify findings. Produces a structured report — no code changes, no PRs.

**Multiple tasks** — run `/fellowship`:

Gandalf (the coordinator) spawns quest and scout teammates. Quests run in isolated worktrees and produce PRs. Scouts research questions and deliver findings. Say "status" to see a progress table. By default, all quest gates surface to you for approval — auto-approve specific gates via `~/.claude/fellowship.json` (see Configuration).

**Health monitoring** — `fellowship health` is the one classifier behind stalled/zombie/struggling detection; `fellowship events --problems` and the dashboard read the same sweep. With 2+ quests active (configurable via `palantir.minQuests`), Gandalf spawns palantir to report it continuously; below that threshold, or with `palantir.enabled: false`, Gandalf runs the sweep itself after gate transitions and spawns, so health monitoring never depends on an extra agent.

### Resuming after a crash

A `/lembas` checkpoint at `.fellowship/checkpoint.md` is what a dead session leaves behind. Exactly three things read it, and each has one job:

| | Reads the checkpoint | When |
|---|---|---|
| **quest's Research step 0** | Yes | Inside a running quest. This is the *only* checkpoint check during a quest — a resumed teammate picks up at the phase its state records. |
| **`/rekindle`** | Yes | Outside a quest, after the whole fellowship died. Scans worktrees, classifies each quest, and respawns them with the resume spawn prompt. |
| **The SessionStart hook above** | No | At session start. Prints a hint that a checkpoint exists; it never resumes anything. |

`/council` does not check for checkpoints. It is orientation for a fresh task, nothing more.

**Gate enforcement** — gates are structurally enforced via plugin hooks. After a teammate submits a gate, their work tools (Edit, Write, Bash, etc.) are blocked until the lead approves by updating quest state. Prerequisites (running `/lembas` and updating task metadata) are verified before gate submission is allowed. Self-approval is structurally impossible.

## Design Principles

- **Context is the bottleneck.** Compact between every phase. Don't let research noise degrade implementation reasoning.
- **Hard gates prevent drift.** No planning without understanding. No implementing without a plan. No PR without review.
- **Compose, don't rebuild.** Skills call other skills. No new runtime code — just orchestration over Claude Code primitives.
- **Human in the loop.** By default, all gates require your approval. You can opt into auto-approval for specific gates via config. Gandalf never merges PRs.
- **Isolation by default.** Every quest gets its own worktree. No shared in-progress state.
- **Local scope only.** Teammates are restricted to code, tests, git, and the filesystem. MCP tools and external services (Notion, Slack, Jira, etc.) require explicit approval.

## Changelog

### Unreleased

- **CLI subcommand nouns renamed to plain words** — the CLI's Tolkien-flavored subcommand nouns are now plain English, matching their Go packages: `herald` → `events`, `tome` → `history`, `errand` → `todo`, `eagles` → `health`, `bulletin` → `notes`, `autopsy` → `failures`, `company` → `group` (and `state add-company` → `state add-group`). Skill and agent names are unaffected — this only touches the seven reporting/side-channel subcommand nouns above. Each old name still works for one release: running it prints one deprecation line to stderr, then runs the renamed command. The `failures.expiryDays` config key replaces `autopsy.expiryDays` (no alias — update `~/.claude/fellowship.json` and any project `.fellowship/config.json`). SQLite table and column names are unchanged (no schema migration): `herald`, `bulletin`/`bulletin_files`, `autopsies`/`autopsy_files`/`autopsy_modules`/`autopsy_tags`, `errands`/`errand_deps`, and `companies`/`company_members` keep their existing names under the renamed packages.
- **One health classifier, reachable everywhere** — `fellowship eagles` and `fellowship herald --problems` were two separate Go implementations of the same stalled/zombie classification, and palantir carried a third copy in prose driven by unbounded `git diff`/`git status` over each worktree. `herald.DetectProblems` now delegates to eagles' sweep (which gained a `struggling` classification — repeated gate rejections in a quest's current phase, independent of its `health`), and `eagles.WriteReport` and the `.fellowship/eagles-report.json` file it wrote (nothing read it) are gone. palantir is now a thin reporter over that one sweep: it runs `fellowship eagles --json` and `fellowship state show --json` instead of reconstructing stuck/stalled from task metadata, and reads scope-drift/file-conflict signals from each quest's tome (`tome show --json`'s `files_touched`) instead of diffing worktrees itself. Below `palantir.minQuests` or with `palantir.enabled: false`, Gandalf runs the same sweep itself after every gate transition and spawn, so health monitoring never depends on an extra agent. `/rekindle` and `/retro` read quest state and phase/health the same way instead of shelling `gate status --dir <worktree>` per quest. `state show` (always JSON, but had no flag to name that) and `company show <name>` (table-only before) both accept `--json` now.
- **Four phases, three gates** — The quest lifecycle is now **Research → Plan → Implement → Review**. Onboard's work (worktree provisioning, context loading, the checkpoint resume check) is the first step of Research; the adversarial balrog pass is the first step of Review and PR creation the last. A gate leaves Research, Plan, and Implement; nothing leaves Review, so the quest ends inside it when the PR is open and the task is marked complete — which `completion-guard` now allows only in Review with no gate pending. Valid `gates.autoApprove` entries are the three gate-bearing phases. A schema migration rewrites stored phase names (live state, phase and gate history, and each quest's `autoApprove` list) in existing stores, and the pre-2.0 JSON importer runs through the same table, so an in-flight quest keeps advancing across the upgrade.
- **`/quest` and `/fellowship` are half the size** — `quest/SKILL.md` went from ~25 KB to ~15 KB and `fellowship/SKILL.md` from ~21 KB to ~15.5 KB. Quest inlines the orientation `/council` did and the pattern extraction `/gather-lore` did rather than invoking them, so a quest no longer hands its phase vocabulary to two satellite skills that then have to track it; both remain as skills you invoke yourself. Fellowship's isolation pre-flight and provisioning protocol moved to `resources/isolation.md`, and Gandalf's voice to `resources/lead-behavior.md`.
- **`/lembas` stops asking for `/compact`** — It ended by telling the user to run a command Claude cannot run, so the step was either ignored or handed over as a chore. It now writes the checkpoint and continues from that summary, which is what the checkpoint was always for.
- **One checkpoint reader per context** — Four things looked for a `/lembas` checkpoint and disagreed about who resumes. Now quest's Research step 0 is the only checkpoint check inside a quest, `/rekindle` is the recovery path outside one, and the README's `SessionStart` hook only prints a hint. `/council` no longer looks for one at all. See "Resuming after a crash".
- **`/rekindle` shares the spawn template** — It carried a hand-copied quest spawn prompt with an undefined `{gate_config_override}` placeholder. `spawn-prompts.md` gained a RESUME variant and rekindle references it, so gate, hold, isolation, and boundary language has one home.
- **Quest templates ship with one** — Templates were a feature with nothing in it. `/lorebook` now resolves a built-in directory after project and user, and fellowship ships `example` — a worked template at the specificity the docs ask for, with no keywords so it never auto-suggests. `/lorebook` and `/scribe` both cover all four phases, and Review's section is the last guidance a quest loads.

- **`/dashboard` command** — Starts the fellowship web dashboard in the background and prints its URL. The dashboard's own company gate approval now shares `company.BatchApprove` with the CLI's `fellowship company approve` instead of a second, drifted copy that skipped tome recording. The core fellowship state model (`FellowshipState`, `QuestEntry`, `CompanyEntry`, and their SQLite CRUD) moved out of the `dashboard` package into a new `cli/internal/fellowship` package, removing the import cycle that forced `company` to duplicate that batch-approve logic. The dashboard's `/api/status` response now includes a `phases` field so the UI's phase list tracks the server instead of a hardcoded array (which was previously missing the Adversarial phase).
- **The lead is no longer locked out of the main tree** — `worktree-guard` blocked every `Edit`/`Write` in the main working tree while a fellowship was active, including the lead's own. `fellowship state init` now records the lead's Claude Code session in a `lead` marker inside the data directory, and the guard allows that session, blocks a quest worktree that resolves to the main root, blocks a session that is known not to be the lead, and allows anything it cannot identify.
- **`dataDir` moves the store too** — the fellowship database was always created in `.fellowship/` even when `dataDir` named a different directory, so the store and everything that reads it lived in different places. The store now follows the configured data directory.
- **`hold`/`unhold` report an unregistered `--dir`** — instead of guessing the quest from the directory's name, which could hold a different quest that happened to share it.
- **One gate state machine** — approve, reject, submit and reset are single functions in the state package, used by `gate approve|reject`, company batch approval, the auto-approve path and the resets. Auto-approved gates now clear the gate id and record the approval and phase transition in the tome and herald, exactly as a lead approval does; a held quest can no longer submit a gate; and `fellowship init` and `state clean-worktrees` reset the lembas/metadata flags along with the gate flags.
- **Fail-closed hook dispatch** — Gate hooks (`gate-guard`, `gate-submit`, `gate-prereq`, `completion-guard`, `metadata-track`, `file-track`) now run through `plugin/hooks/scripts/fellowship.sh` instead of exec'ing the binary directly; if the binary is missing and can't be installed, they block (exit 2) instead of silently allowing the tool call through a shell "command not found". `worktree-guard` keeps its fail-open backstop posture. The `file-track` hook is now wired into `hooks.json` (it existed in the CLI but wasn't invoked), and `SessionStart` now installs the binary on `clear` and `compact` in addition to `startup`/`resume`.
- **Verified downloads** — `ensure-binary.sh` verifies the downloaded tarball against the release's `checksums.txt` (`sha256sum`/`shasum`) before installing, assembles the binary in a scratch directory and moves it into place atomically, and holds a simple lock so concurrent sessions don't race the same install.
- **CI** — added `gofmt -l .`, `go vet ./...`, `go test -race ./...`, `shellcheck` on the hook scripts, a check that every path in `.claude-plugin/plugin.json` exists, and a `site/` build job.
- **Tightened skill triggers** — `quest`, `council`, `gather-lore`, and `warden` descriptions now name their actual invocation scope instead of "any non-trivial task", reducing over-triggering.
- **Removed orphaned `quest-runner` agent** — never spawned (quest teammates use `general-purpose`); removed from the plugin manifest, README, and the site's Agents and How It Works pages.
- **Documentation drift fixes** — corrected `gates.autoApprove` valid values on the site config page, replaced the removed `using-git-worktrees` dependency with `writing-plans` (Plan phase), added the missing v1.6.1 changelog entry, fixed the quest phase/gate count, documented `autopsy.expiryDays` and added the missing `dataDir` row to `/settings`' schema table, corrected the `.fellowship/` gitignore wording in lembas, corrected palantir's Bash tool description, and fixed several command titles and skill/command wording.
- **Archived the `gate-state-machine` OpenSpec change** — superseded by the Go CLI + SQLite gate enforcement design (v1.5.1–v2.2.0); moved to `openspec/changes/archive/` with a SUPERSEDED note.
- **Documented CLI invocations now work** — `--dir <path>` is accepted by `gate status|approve|reject`, `state add-quest|add-scout|add-company|update-quest|show`, `errand init|list|add|update|show`, `autopsy create|scan|infer`, and `tome show`, resolving the quest exactly as if the process were running in that directory. `gate` previously had no flag parsing at all, so every documented `--dir` call failed.
- **`fellowship init` name resolution** — Without `--quest`, init now uses the quest name the lead registered with `state add-quest` for that worktree, falling back to the directory name only when the worktree is unregistered.
- **`fellowship init` reads `gates.autoApprove`** — Auto-approved gates come from the merged config (project `.fellowship/config.json`, then `~/.claude/fellowship.json`) instead of always being empty. Unknown phase names are rejected.
- **`fellowship status` honors the base branch** — Merged-branch detection compares against the fellowship's stored `base_branch` instead of a hardcoded `main`.
- **`fellowship herald post`** — Records a tiding from the CLI, so the palantir logs alerts without `jq` or a hand-written JSONL file. `herald` gained `--quest` and `--limit`; `autopsy scan --all` returns every unexpired autopsy.
- **Prompt layer matches the binary** — Skills, agents, and commands now call the CLI by its full path, use only flags that exist, and read state through the CLI instead of the pre-2.0 JSON files (`quest-state.json`, `fellowship-state.json`, `quest-tome.json`, `quest-herald.jsonl`, `quest-errands.json`, `palantir-alerts.jsonl`, `autopsies/`).

### v2.2.0

- **Model routing** — Every subagent spawn point now routes to a cost-appropriate model: palantir defaults to `haiku`, scout and the validator to `sonnet`, Explore scans to `haiku`, while quest teammates and balrog keep the session model. Override any role via the new `models.*` config block.
- **Validator agent** — Scout's adversarial validation runs in a dedicated read-only agent (Read/Glob/Grep only, enforced by tool restrictions) instead of an unrestricted general-purpose subagent.
- **Mode-aware gate accounting** — The lead verifies quest completion against the gates the quest's mode actually requires: 6 for standard/promoted quests (Adversarial included), 3 for plan-driven. Progress tracking and phase enumerations now include Adversarial everywhere.
- **Spawn prompt consolidation** — The three quest spawn prompt variants collapsed into one base template with per-variant deltas, eliminating ~250 lines of drift-prone duplication and unifying hold/shutdown language.
- **Project config layer** — Fellowship startup and quest onboard now merge `.fellowship/config.json` (project) with `~/.claude/fellowship.json` (user) as defaults → project → user, matching `/settings`.
- **CLI phase fix** — `fellowship init --phase Adversarial` was rejected and company progress ranked Adversarial-phase quests as zero; phase lists now derive from a single canonical order in the state package.
- **Messaging protocol fixes** — SendMessage recipients are teammate names (task-ID addressing never delivered); balrog and scout embed the full report envelope inline; balrog gained Write/Edit scoped strictly to test files ("report, don't repair").
- **Docs refresh** — README and site document all 10 skills, 6 commands, and 5 agents; the skills page separates auto-invoked skills from user-invoked commands; `/validate-docs` gained a config-schema cross-check.

### v2.1.0

- **Worktree isolation guard** — A fail-closed hook blocks quest teammates from writing source into the main working tree when isolation is skipped. `fellowship state init` registers it in the git-ignored `.claude/settings.local.json` (no commits to your repo), and it arms only while a quest worktree is live, so it never blocks ordinary solo work.
- **Lead cd-guard hardening** — Gandalf is now blocked from `cd`-ing into quest worktrees created outside `.claude/worktrees/` (e.g. lead-provisioned worktrees), preventing the lead from inheriting a quest's gate or hold state.

### v2.0.0

- **SQLite storage** — All state (quests, gates, tome, errands, herald, bulletin, autopsy) migrated from JSON files to SQLite with WAL mode. Eliminates file locking issues and race conditions in parallel quests. Run `fellowship migrate` to upgrade existing data.
- **Interactive `/guide`** — Rewrote the guide from a passive concept explainer to a learn-by-doing walkthrough. Walks beginners through a real quest on their own codebase, then introduces `/quest` and `/fellowship`.
- **Concepts page** — New docs site page explaining agentic workflows, orchestration, isolation, context engineering, and human-in-the-loop.
- **Quest autopsy** — Failure memory that persists across sessions. When a quest fails, records what went wrong so future quests can learn from past failures.
- **Bulletin board** — Cross-quest knowledge sharing. Quests post discoveries to a shared bulletin during Research and Implement.
- **Gate enrichment** — Gate submissions now include structured context (diff stats, test results, phase summary) for informed approval decisions.
- **WorktreeGuard** — Blocks the lead session from accidentally `cd`-ing into quest worktrees.

### v1.9.2

- **Stale gate state fix** — Gate guard hook no longer blocks Gandalf when a previous quest's gate state file is present in a fresh worktree. Prevents stale state from causing spurious tool blocks at session start. ([#56](https://github.com/justinjdev/fellowship/issues/56))

### v1.9.1

- **Fellowship startup fix** — `ensure-binary.sh` now runs before any fellowship operations, removing the PATH dependency. All CLI calls use the full binary path (`~/.claude/fellowship/bin/fellowship`).
- **`state init` overwrite warning** — Instead of erroring when `fellowship-state.json` already exists, `fellowship state init` now warns and proceeds (shows existing name and quest count).
- **`validate-docs` marketplace check** — Validates that skill and agent counts in the marketplace description match the actual plugin.
- **Deprecated commands removed** — `fellowship install` and `fellowship uninstall` CLI subcommands removed.

### v1.9.0

- **`/missive` skill** — Fetches GitHub issue context for quest spawning. Pulls title, body, labels, and comments via `gh` CLI. Returns issue context, suggested branch name (with issue number), and PR closing keywords. Gandalf invokes it automatically on `#N` references; also usable standalone as `/missive 42`.
- **Balrog agent** — Adversarial validation agent. Reviews code for structural quality: factoring, coupling, cohesion, abstraction levels, information hiding. Challenges every design decision, not just obvious violations.
- **Per-project config** — Committable `.fellowship/config.json` for team-shared settings. Three-way merge: defaults → project → user (user always wins). `/settings` shows merged config with provenance annotations.
- **`issues.autoClose` config** — When true (default), `/missive` adds `Closes #N` to PR keywords so issues close on merge.
- **Base branch fixes** — Worktrees receive the correct base branch; handles detached HEAD and dirty working tree edge cases.

### v1.8.0

- **Scout-to-quest promotion** — Say `promote scout-X to a quest` during a fellowship. Gandalf reads the scout's findings file, spawns a quest pre-loaded with the research, and the quest enters validation mode instead of researching from scratch.
- **`/retro` skill** — Post-fellowship retrospective. Analyzes gate history, palantir alerts, and quest metrics. Recommends configuration changes like auto-approving gates with zero rejection rates. Integrated into the fellowship disband flow.
- **Plan-driven quests** — Provide a pre-existing plan file and quests skip Research and Plan phases, jumping straight to Implement. Gandalf can fan out large plans into multiple parallel quests.
- **Structured conflict resolution** — Hold mechanism for quests with file conflicts. Gandalf detects overlapping file sets and holds conflicting quests until dependencies complete.
- **Herald logging** — Dashboard gate handlers and company batch approve now emit herald events for observability.
- **Palantir alert persistence** — Alerts persisted to JSONL log for post-fellowship analysis by `/retro`.
- **`/release` command** — Repo-level release automation. Suggests version based on conventional commits, audits docs/site/changelog, bumps plugin.json, tags, pushes, and updates marketplace.

### v1.7.5

- **Fix** — Hook binary distribution fixes (v1.7.1–v1.7.5). Use binary directly in hooks, bootstrap via SessionStart, remove duplicate hook installation.

### v1.7.0

- **Eagles** — quest health monitoring daemon. Detects stuck quests, scope drift, and file conflicts via periodic patrol scans.
- **Tome** — persistent agent identity with quest CV chains. Tracks phases completed, gate history, and files touched across quest lifetimes.
- **Company** — work bundling for quest grouping. Groups related quests into a company for coordinated tracking and status reporting.
- **Herald** — activity feed with event logging, problem detection, and dashboard integration. Surfaces quest events and auto-detected problems.
- **State CLI** — `fellowship state` commands for inspecting and managing quest state, plus `fellowship state add-company` for company management.
- **File locking** — mutex-based file locking for concurrent state mutations across parallel quests.
- **`.fellowship/` data directory** — working files (state, checkpoints, errands, tome) now use `.fellowship/` instead of `tmp/`. Configurable via `dataDir` setting.
- **CI** — PR workflow to run Go tests on pull requests.
- **LOTR theming** — renamed internals: patrol→eagles, convoy→company, cv→tome, events/feed→herald.
- **Shared helpers** — extracted common git/file utilities into `internal/gitutil` package.
- **Fix** — phase tracking for auto-approved gates and pending submissions.
- **Fix** — hook errors silenced in non-quest contexts.

### v1.6.3

- **Fix plugin discovery** — moved `.claude-plugin/plugin.json` to repo root with explicit path fields for skills, agents, commands, and hooks. Fixes skills not showing up after install.

### v1.6.1

- **GitHub Pages site** — SvelteKit static site with LOTR theme, all documentation pages, and CI deployment.
- **`/rekindle` skill** — Crash recovery. Scans worktrees and state files, presents a recovery dashboard, and re-spawns Gandalf with recovered quest context.
- **`/lorebook` skill** — Loads phase-specific guidance from quest templates created by `/scribe`.
- **Skills to commands migration** — 5 user-only skills moved to `commands/` for lower base context cost.
- **LOTR theming** — Internal renames: convoy → company, cv → tome, patrol → eagles, work/hook → errand, events/feed → herald.

### v1.6.0

- **`/scout` skill** — research & analysis workflow for lightweight research teammates alongside code quests. Autonomous (no gates/hooks), optional adversarial validation via fresh subagent. ([#12](https://github.com/justinjdev/fellowship/pull/12))
- **Fellowship scouts** — Gandalf learns to spawn scouts via `"scout: <question>"` alongside code quests, with status tracking and optional routing to other teammates.

### v1.5.1

- **Go CLI** — `fellowship` binary replaces bash hook scripts. Handles hook logic, gate approval/rejection, install/uninstall, and status. Distributed via GitHub releases, auto-downloaded on first use.
- **Plugin subfolder** — plugin files moved to `plugin/` for clean installs via marketplace `git-subdir`. Go source, CI, and build config stay at repo root.
- **Quest runner agent** — `agents/quest-runner.md` for CLI-driven quest execution.
- **BREAKING** — bash hook scripts replaced by Go CLI binary. `jq` no longer required.

### v1.5.0

- **Gate state machine** — structural enforcement of quest phase gates via plugin hooks. Teammate tools are blocked after gate submission until the lead approves. Prerequisites (lembas + metadata) are verified before submission. Self-approval is structurally impossible. Observed compliance: ~33% with prompt-only → ~95%+ with hooks. ([#5](https://github.com/justinjdev/fellowship/pull/5))
- **Hook scripts** — 4 plugin hooks (`gate-guard`, `gate-submit`, `gate-prereq`, `metadata-track`) with test suite
- **`jq` dependency** — required for gate enforcement. Hooks fail-closed if `jq` is missing.
- **BREAKING** — plugin now ships executable bash scripts (`hooks/scripts/`). Previously pure markdown only.

### v1.4.0

- **gather-lore rewrite** — simplified to study-only (pattern extraction). Code generation and diff checking removed as redundant with quest Implement + warden Review phases.
- **`/red-book` skill** — new skill for capturing conventions from PR reviewer feedback into CLAUDE.md. Closes the convention learning loop.
- **Quest recovery** — Phase 3 now has explicit recovery procedure: when implementation hits a wall, stop, commit partial work, document the blocker, return to Plan phase.
- **Quest resume** — failed/dead quests can be respawned into their existing worktree. Council finds the lembas checkpoint and offers to resume.
- **Palantir fix** — spawned as `fellowship:palantir` (custom agent with restricted tools) instead of `general-purpose`.
- **Palantir cadence** — event-driven monitoring triggered by Gandalf after gate transitions and quest spawns, instead of unbounded.
- **Worktree ownership** — quest Phase 0 owns worktree creation. Fellowship no longer passes `isolation: "worktree"`, eliminating double-worktree conflicts and unused branch naming logic.
- **Config schema dedup** — canonical schema lives in `/settings`. Fellowship references it instead of duplicating.
- **`branchPrefix` removed** — deprecated key fully removed from all skills and config.
- **Escape hatch criteria** — concrete heuristics (single file, < 50 lines, no new patterns, familiar area) replace "use judgment".
- **Monorepo conditional** — council package scope step now skips for single-package repos.
- **Nested subagent worktrees removed** — if plan subtasks have file conflicts, fix the plan.

### v1.3.0

- **Branch name patterns** — `branch.pattern` config with a flexible template system. Supports `{slug}`, `{ticket}`, and `{author}` placeholders for team-specific branch naming conventions (e.g., `"{author}.{ticket}.{slug}"` produces `justin.JIRA-123.fix-auth-bug`). Missing placeholders are prompted interactively. **Breaking:** removed `branchPrefix` (deprecated in v1.3.0). Use `branch.pattern` instead — e.g., `"myprefix/{slug}"` replaces `"branchPrefix": "myprefix/"`.

### v1.2.0

- **`/config` command** — interactive skill to view, edit, and reset fellowship settings
- **Config moved to personal directory** — `~/.claude/fellowship.json` is now loaded from the user's personal Claude directory instead of the project root, making settings cross-project
- **Custom worktree directory** — `worktree.directory` config option for organizations that don't use Claude Code's default worktree location
- **Removed superpowers:using-git-worktrees dependency** — quest now uses `EnterWorktree` directly for worktree isolation

### v1.1.0

- **Config file support** — `~/.claude/fellowship.json` for customizing branch prefixes, gate auto-approval, PR defaults, worktree strategy, and palantir settings ([#3](https://github.com/justinjdev/fellowship/pull/3))
- **Palantir rewrite** — rewrote from dead code into a functional monitoring agent that watches quest progress, detects stuck quests and scope drift, and alerts Gandalf via SendMessage ([#2](https://github.com/justinjdev/fellowship/pull/2))
- **Progress tracking** — teammates report current phase via task metadata; say "status" during a fellowship for a structured progress table ([#1](https://github.com/justinjdev/fellowship/pull/1))
- **Gate blocking fix** — replaced ineffective "WAIT" instruction with explicit turn-ending so agents actually stop at gates ([#1](https://github.com/justinjdev/fellowship/pull/1))
- **Lembas compaction at all transitions** — added missing `/lembas` invocations at Implement→Review and Review→Complete ([#1](https://github.com/justinjdev/fellowship/pull/1))
- **Steward removed** — deleted dead agent; decomposition logic was already inlined in quest Phase 3 ([#1](https://github.com/justinjdev/fellowship/pull/1))
- **Gate discipline** — Gandalf must never combine or skip gate approvals
- **Conventional commits** — spawn prompt and quest guidelines now enforce conventional commit format

### v1.0.0

- Initial release: quest lifecycle, fellowship orchestration, council, gather-lore, lembas, warden, chronicle

## License

MIT
