---
name: council
description: Invoke only when the user runs /council. Quest inlines this orientation as its Research step 2 and does not call it. Loads focused, task-relevant context by reading CLAUDE.md, scanning for related files, and producing a structured Session Context block.
---

# Council — Context-Aware Task Onboarding

## Overview

Loads focused, task-relevant context at the start of any non-trivial work session. Produces a structured Session Context block that serves as the foundation for all downstream work. This is the "onboarding the agent within every repo" pattern from context engineering.

## When to Use

- The user runs `/council` — starting a task that involves more than a quick fix, or picking up in-progress work

This skill is **not** a step of the quest lifecycle. Quest inlines the same orientation as its Research step 2 rather than invoking it, so the two cannot drift apart.

Council does not look for a `/lembas` checkpoint. Exactly one place checks for one during a quest — quest's Research step 0 — and `/rekindle` is the recovery path outside a quest.

## Process

### Step 1: Read Project Context

Read the root CLAUDE.md. Extract:
- Reference files relevant to the task area
- Review conventions that apply
- Architecture constraints

If no CLAUDE.md exists, note: "Consider running `/chronicle` to set up project context."

Also check for `~/.claude/fellowship.json` (the user's personal Claude directory). If it exists, read it and note any non-default settings. These will be included in the Session Context block under Architecture Notes so downstream skills (lembas, warden) are aware of the active configuration.

### Step 2: Understand the Task

Ask one focused question:

> "What are you working on? (One sentence describing the task and which area of the codebase it touches.)"

If the user already stated the task, skip the question.

### Step 3: Identify Package Scope

Determine the scope of the task within the project:

**Monorepo** (has `packages/`, `apps/`, or a workspace config like `pnpm-workspace.yaml`):
1. Match the task area to a package directory (e.g., `packages/<name>/`, `apps/<name>/`)
2. If ambiguous, check the monorepo structure (`ls` top-level directories) and ask the user
3. If the task spans multiple packages, list all affected packages
4. If a package-level CLAUDE.md exists (e.g., `packages/<name>/CLAUDE.md`), read it and merge its conventions with the root CLAUDE.md. Package-level conventions override root conventions where they conflict.

**Single-package repo:** Skip this step — the scope is the whole project. Proceed to Step 4.

This scope constrains all downstream scanning and verification.

### Step 4: Scan for Relevant Files

Use the Explore agent (Agent tool with subagent_type=Explore, passing `model: "haiku"` — file scanning does not need the session model; use `models.explore` from fellowship config instead if set) to find files related to the task description. **In a monorepo, scope the search to the identified package(s)** — do not scan the entire repo.

Focus on:
- Files that will likely need modification
- Files that define the patterns to follow (reference files)
- Test files for the affected area
- Config or type files that constrain the work

Keep the scan targeted — 5-10 key files maximum, not an exhaustive listing.

### Step 5: Produce Session Context Block

Output a structured block in this exact format:

```
## Session Context

**Task:** [one-line description]

**Package(s):** [package name(s) and path(s)]

**Key Files:**
- [path/to/file:lines] — [why it's relevant]
- [path/to/file:lines] — [why it's relevant]

**Relevant Conventions:**
- [convention from root CLAUDE.md that applies]
- [convention from package CLAUDE.md that applies, if exists]

**Architecture Notes:**
- [constraints, patterns, or dependencies to be aware of]

**Out of Scope:**
- [things explicitly not to touch or change]
- [other packages not affected by this task]
```

### Step 6: Confirm with User

Present the Session Context block and ask:

> "Does this capture the right scope? Anything missing or out of bounds?"

Revise based on feedback.

## Key Principles

- **Targeted, not exhaustive.** 5-10 key files, not every file in the directory.
- **Carry forward.** The Session Context block is referenced by lembas throughout the session.
- **Minimal questions, never stall.** One focused task question (Step 2) when the task wasn't already provided; Steps 3 and 6 ask only when genuinely ambiguous. As a fellowship teammate there is no direct user — route blocking questions to the lead via SendMessage, and otherwise proceed with documented assumptions rather than waiting on confirmation.
