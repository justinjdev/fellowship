<img width="1512" height="982" alt="Screenshot 2026-02-27 at 14 55 56" src="https://github.com/user-attachments/assets/a4fb319e-20ca-4bba-8595-134cbd06f4b6" />

# Fellowship

A Claude Code plugin that orchestrates multi-task workflows through structured research-plan-implement lifecycles. Named after the obvious — a fellowship of agents, each on their own quest, coordinated by a wizard who never writes code.

## What It Does

Fellowship gives Claude Code a disciplined workflow engine. Instead of diving straight into code, tasks go through phased lifecycles with hard gates between them: research the system, plan the changes, implement with TDD, review against conventions, then ship.

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
| **superpowers** | `using-git-worktrees`, `test-driven-development`, `verification-before-completion`, `finishing-a-development-branch` | Onboard, Implement, Review, Complete |
| **pr-review-toolkit** | `review-pr` | Review |

These are referenced by name in skill prompts. If a dependency isn't installed, Claude will skip that step rather than fail — but you lose the discipline that step provides.

```
/plugin marketplace add obra/superpowers-marketplace
/plugin install superpowers@superpowers-marketplace
/plugin install pr-review-toolkit@claude-plugins-official
```

#### System Dependencies

- **Go CLI binary** — gate enforcement hooks use a Go binary that is automatically downloaded from GitHub releases on first use. No manual installation required.

### Project Setup (Optional)

Add this hook to `.claude/settings.local.json` in repos where you use fellowship. It detects `/lembas` checkpoints from previous sessions and offers to resume:

```json
{
  "hooks": {
    "SessionStart": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "if [ -f .fellowship/checkpoint.md ]; then echo '--- CHECKPOINT DETECTED ---'; cat .fellowship/checkpoint.md; echo '--- END CHECKPOINT ---'; echo 'A checkpoint from a previous session was found. Use /council to resume or start fresh.'; fi"
          }
        ]
      }
    ]
  }
}
```

Also add `.fellowship/` to your `.gitignore` — checkpoints are local ephemeral state. If you have configured a custom `dataDir` in `~/.claude/fellowship.json`, use that directory name instead.

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
  }
}
```

| Setting | Default | Description |
|---------|---------|-------------|
| `dataDir` | `".fellowship"` | Directory name for fellowship working files (state, checkpoints, errands, tome). Created inside each worktree and the main repo root. |
| `branch.pattern` | `null` | Branch name template with placeholders: `{slug}` (task description), `{ticket}` (extracted from description), `{author}` (from config). When `null`, defaults to `"fellowship/{slug}"`. |
| `branch.author` | `null` | Static value for the `{author}` placeholder. If not set and pattern uses `{author}`, you'll be prompted. |
| `branch.ticketPattern` | `"[A-Z]+-\\d+"` | Regex to extract ticket IDs from quest descriptions. Default matches Jira-style IDs (e.g., `PROJ-123`). |
| `worktree.enabled` | `true` | Whether quests create isolated worktrees. Set to `false` to work on the current branch. |
| `worktree.directory` | `null` | Parent directory for worktrees. `null` uses Claude Code's default (`.claude/worktrees/`). |
| `gates.autoApprove` | `[]` | Gate names to auto-approve: `"Research"`, `"Plan"`, `"Implement"`, `"Review"`, `"Complete"`. Gates not listed still surface to you for approval. |
| `pr.draft` | `false` | Create PRs as drafts. |
| `pr.template` | `null` | PR body template string. Supports `{task}`, `{summary}`, and `{changes}` placeholders. |
| `palantir.enabled` | `true` | Whether to spawn a palantir monitoring agent during fellowships. |
| `palantir.minQuests` | `2` | Minimum active quests before palantir is spawned. |

The config is read at fellowship startup and quest onboard (Phase 0). Changes to the file take effect on the next fellowship or quest invocation.

## Skills

| Skill | Purpose |
|-------|---------|
| `/quest` | Full Research → Plan → Implement lifecycle for non-trivial tasks. The hub that orchestrates everything else. |
| `/fellowship` | Multi-task orchestrator. Spawns parallel agent teammates running `/quest` (code) or `/scout` (research). |
| `/scout` | Research & analysis workflow. Investigates questions, optionally validates with a fresh adversarial subagent. No code, no PRs, no commits. |
| `/council` | Context-aware onboarding. Loads task-relevant files, conventions, and architecture at session start. |
| `/gather-lore` | Studies reference files to extract conventions before writing code. Prevents "wrong approach" rework. |
| `/lembas` | Context compression between phases. Keeps the context window in the reasoning sweet spot. |
| `/warden` | Pre-PR convention review. Compares changes against reference files and documented patterns. |
| `/chronicle` | One-time codebase bootstrapping. Walks through your project to extract conventions into CLAUDE.md. |
| `/red-book` | Post-PR convention capture. Extracts conventions from reviewer comments and adds them to CLAUDE.md. |
| `/settings` | View or edit fellowship settings (`~/.claude/fellowship.json`). Interactive setup for all configuration options. |

## Agents

| Agent | Role |
|-------|------|
| **palantir** | Background monitor during fellowship execution. Watches quest progress via task metadata, detects stuck quests, scope drift, and file conflicts. Reports to Gandalf. |
| **quest-runner** | Quest execution agent. Uses the fellowship CLI for gate management, status checks, and phase transitions. |

## How It Works

**Single task** — run `/quest`:

```
Phase 0: Onboard    → worktree isolation + /council context loading
Phase 1: Research   → explore agents + /gather-lore
Phase 2: Plan       → plan mode with file:line references + user approval
Phase 3: Implement  → TDD (red-green-refactor)
Phase 4: Review     → /warden conventions + code quality + verification
Phase 5: Complete   → PR creation + worktree cleanup
```

**Research** — run `/scout`:

```
Investigate → (Validate) → Deliver
```

Autonomous research with confidence levels. For complex questions, spawns a fresh validator subagent to adversarially verify findings. Produces a structured report — no code changes, no PRs.

**Multiple tasks** — run `/fellowship`:

Gandalf (the coordinator) spawns quest and scout teammates. Quests run in isolated worktrees and produce PRs. Scouts research questions and deliver findings. Say "status" to see a progress table. By default, all quest gates surface to you for approval — auto-approve specific gates via `~/.claude/fellowship.json` (see Configuration).

**Gate enforcement** — gates are structurally enforced via plugin hooks. After a teammate submits a gate, their work tools (Edit, Write, Bash, etc.) are blocked until the lead approves by writing to the quest state file. Prerequisites (running `/lembas` and updating task metadata) are verified before gate submission is allowed. Self-approval is structurally impossible.

## Design Principles

- **Context is the bottleneck.** Compact between every phase. Don't let research noise degrade implementation reasoning.
- **Hard gates prevent drift.** No planning without understanding. No implementing without a plan. No PR without review.
- **Compose, don't rebuild.** Skills call other skills. No new runtime code — just orchestration over Claude Code primitives.
- **Human in the loop.** By default, all gates require your approval. You can opt into auto-approval for specific gates via config. Gandalf never merges PRs.
- **Isolation by default.** Every quest gets its own worktree. No shared in-progress state.
- **Local scope only.** Teammates are restricted to code, tests, git, and the filesystem. MCP tools and external services (Notion, Slack, Jira, etc.) require explicit approval.

## Changelog

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
