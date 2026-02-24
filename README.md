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
| **steward** | Breaks plans into parallel work units and spawns focused sub-agents. Manages scope boundaries and synthesizes results. |
| **palantir** | Background monitor during parallel execution. Detects stuck agents, scope drift, and file conflicts. |

## How It Works

**Single task** — run `/quest`:

```
Phase 0: Onboard    → worktree isolation + /council context loading
Phase 1: Research   → explore agents + /gather-lore
Phase 2: Plan       → plan mode with file:line references + user approval
Phase 3: Implement  → TDD (red-green-refactor), parallel subagents if independent
Phase 4: Review     → /warden conventions + code quality + verification
Phase 5: Complete   → PR creation + worktree cleanup
```

**Multiple tasks** — run `/fellowship`:

Gandalf (the coordinator) spawns quest-running teammates, each in an isolated worktree. Research and plan gates auto-approve. Implement and complete gates surface to you for approval. Each quest produces a PR.

## Design Principles

- **Context is the bottleneck.** Compact between every phase. Don't let research noise degrade implementation reasoning.
- **Hard gates prevent drift.** No planning without understanding. No implementing without a plan. No PR without review.
- **Compose, don't rebuild.** Skills call other skills. No new runtime code — just orchestration over Claude Code primitives.
- **Human in the loop.** Plans require your approval. Gandalf doesn't merge PRs.
- **Isolation by default.** Every quest gets its own worktree. No shared in-progress state.

## License

MIT
