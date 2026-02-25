# Fellowship

A Claude Code plugin that orchestrates multi-task workflows through structured research-plan-implement lifecycles. Named after the obvious — a fellowship of agents, each on their own quest, coordinated by a wizard who never writes code.

## What It Does

Fellowship gives Claude Code a disciplined workflow engine. Instead of diving straight into code, tasks go through phased lifecycles with hard gates between them: research the system, plan the changes, implement with TDD, review against conventions, then ship.

For multiple independent tasks, it spins up parallel agent teammates — each in an isolated git worktree — coordinated by a lead agent (Gandalf) who routes approvals and reports progress.

## Install

From within Claude Code:

```
/plugin marketplace add justinjdev/claude-plugins
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
            "command": "if [ -f tmp/checkpoint.md ]; then echo '--- CHECKPOINT DETECTED ---'; cat tmp/checkpoint.md; echo '--- END CHECKPOINT ---'; echo 'A checkpoint from a previous session was found. Use /council to resume or start fresh.'; fi"
          }
        ]
      }
    ]
  }
}
```

Also add `tmp/` to your `.gitignore` — checkpoints are local ephemeral state.

### Configuration (Optional)

Create `.claude/fellowship.json` in your project root to customize fellowship behavior. All settings are optional — missing keys use sensible defaults that match the out-of-box behavior.

```json
{
  "branchPrefix": "fellowship/",
  "worktree": {
    "enabled": true
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
| `branchPrefix` | `"fellowship/"` | Prefix for worktree branch names. E.g., `"feature/"` produces `feature/{task-slug}`. |
| `worktree.enabled` | `true` | Whether quests create isolated worktrees. Set to `false` to work on the current branch. |
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
| `/fellowship` | Multi-quest orchestrator. Spawns parallel agent teammates, each running `/quest` in its own worktree. |
| `/council` | Context-aware onboarding. Loads task-relevant files, conventions, and architecture at session start. |
| `/gather-lore` | Studies reference files to extract conventions before writing code. Prevents "wrong approach" rework. |
| `/lembas` | Context compression between phases. Keeps the context window in the reasoning sweet spot. |
| `/warden` | Pre-PR convention review. Compares changes against reference files and documented patterns. |
| `/chronicle` | One-time codebase bootstrapping. Walks through your project to extract conventions into CLAUDE.md. |

## Agents

| Agent | Role |
|-------|------|
| **palantir** | Background monitor during fellowship execution. Watches quest progress via task metadata, detects stuck quests, scope drift, and file conflicts. Reports to Gandalf. |

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

**Multiple tasks** — run `/fellowship`:

Gandalf (the coordinator) spawns quest-running teammates, each in an isolated worktree. By default, all phase gates surface to you for approval. You can auto-approve specific gates via `.claude/fellowship.json` (see Configuration). Each quest produces a PR. Say "status" to see a progress table showing each quest's current phase with visual progress indicators.

## Design Principles

- **Context is the bottleneck.** Compact between every phase. Don't let research noise degrade implementation reasoning.
- **Hard gates prevent drift.** No planning without understanding. No implementing without a plan. No PR without review.
- **Compose, don't rebuild.** Skills call other skills. No new runtime code — just orchestration over Claude Code primitives.
- **Human in the loop.** By default, all gates require your approval. You can opt into auto-approval for specific gates via config. Gandalf never merges PRs.
- **Isolation by default.** Every quest gets its own worktree. No shared in-progress state.
- **Local scope only.** Teammates are restricted to code, tests, git, and the filesystem. MCP tools and external services (Notion, Slack, Jira, etc.) require explicit approval.

## Changelog

### v1.1.0

- **Config file support** — `.claude/fellowship.json` for customizing branch prefixes, gate auto-approval, PR defaults, worktree strategy, and palantir settings ([#3](https://github.com/justinjdev/fellowship/pull/3))
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
