---
description: Interactive guide to how fellowship works. Use when you're new to fellowship or need a refresher on concepts, commands, and workflow.
---

# Fellowship Guide

Present the following overview to the user, then transition to Q&A mode.

---

## What is Fellowship?

Fellowship is a multi-quest orchestrator for Claude Code. It coordinates parallel tasks — code quests and research scouts — using agent teams. A coordinator named Gandalf manages the fellowship: spawning teammates, routing approvals, and reporting progress. Gandalf never writes code.

## Core Concepts

### Quests

A quest is a structured code task that moves through 6 phases:

```
Onboard -> Research -> Plan -> Implement -> Review -> Complete
```

Each quest runs in an isolated git worktree so teammates don't interfere with each other. At the end, each quest produces a PR.

- **Onboard** — create worktree, load project context
- **Research** — explore the codebase, understand the system
- **Plan** — outline explicit steps with file paths and test strategy
- **Implement** — execute the plan using TDD (test-driven development)
- **Review** — convention checks, code quality review, verification
- **Complete** — create PR, clean up

### Gates

Between each phase is an approval gate. By default, every gate surfaces to you for approval — you see a summary of what the quest accomplished and decide whether to proceed. This keeps you in the loop without micromanaging.

You can auto-approve specific gates via config (e.g., always auto-approve Research and Plan gates) so only the high-stakes transitions need your attention.

### Gandalf (The Coordinator)

When you run `/fellowship`, you become Gandalf — the coordinator who:
- Spawns quest runners and scouts as teammates
- Routes gate approvals between you and teammates
- Tracks progress across all active quests
- Never writes code directly

### Worktrees

Each quest gets its own git worktree — a separate working directory on its own branch. This means multiple quests can modify code simultaneously without conflicts. Worktrees are created automatically in Phase 0.

### Lembas (Context Compression)

Between each phase, the quest runner compresses its conversation context into a structured checkpoint (`.fellowship/checkpoint.md`). This keeps the context window in the "smart zone" and provides crash recovery — if a session dies, the checkpoint survives on disk.

### Scouts

Scouts are research-only tasks. They investigate questions, analyze code, and produce reports — but never modify code or create PRs. Use scouts for questions like "how does the auth middleware chain work?" or "list all API endpoints." Scout findings can be routed to specific quest runners.

## Commands

These are the commands you'll use directly:

| Command | What it does |
|---------|-------------|
| `/fellowship` | Start a new fellowship — become Gandalf and coordinate parallel quests |
| `/quest` | Run a single quest outside of a fellowship (standalone mode) |
| `/guide` | This guide — overview of how fellowship works |
| `/rekindle` | Recover a fellowship after a session crash |
| `/settings` | View or edit fellowship config (`~/.claude/fellowship.json`) |
| `/scribe` | Create a project-specific quest template from codebase conventions |
| `/chronicle` | Generate a CLAUDE.md conventions section by analyzing your codebase |
| `/red-book` | Capture PR review feedback as conventions in CLAUDE.md |

## Configuration

Fellowship config lives at `~/.claude/fellowship.json`. It's optional — sensible defaults apply without it. Key settings:

- **`gates.autoApprove`** — list of gates to auto-approve (e.g., `["Research", "Plan"]`)
- **`branch.pattern`** — branch naming pattern (default: `fellowship/{slug}`)
- **`pr.draft`** — create PRs as drafts
- **`palantir.enabled`** — enable/disable the background monitor

Run `/settings` to view or modify your config interactively.

## Quick Start

1. Run `/fellowship`
2. Say `quest: <describe your task>` to add quests
3. Say `scout: <your question>` to add research scouts
4. Approve gates as they come in
5. Review PRs when quests complete
6. Say `wrap up` when done

You can add quests and scouts at any time — fellowships are dynamic.

---

Now ask the user: **"What would you like to know more about?"**

If the user asks about a specific concept, explain it in more depth with examples. If they ask about a command, describe its full workflow. If they say they're ready, suggest they start with `/fellowship`.
