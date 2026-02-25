---
name: council
description: Use at the start of any non-trivial task. Loads focused, task-relevant context by reading CLAUDE.md, scanning for related files, and producing a structured Session Context block. Invoked automatically by quest or standalone via /council.
---

# Council — Context-Aware Task Onboarding

## Overview

Loads focused, task-relevant context at the start of any non-trivial work session. Produces a structured Session Context block that serves as the foundation for all downstream work. This is the "onboarding the agent within every repo" pattern from context engineering.

## When to Use

- Starting any task that involves more than a quick fix
- Beginning a new session on existing in-progress work
- Invoked automatically as Phase 0 of `/quest`

## Process

### Step 0: Check for Existing Checkpoint

Before doing anything else, check if `tmp/checkpoint.md` exists (in repo root).

If it does:
1. Read the checkpoint file
2. Present the checkpoint summary to the user
3. Ask: **"Found a checkpoint from [timestamp] on branch [branch]. Resume from where you left off, or start fresh?"**
4. If resuming: load the checkpoint as the Session Context. Skip Steps 1-4 and go directly to Step 5 (confirm with user).
5. If starting fresh: delete the checkpoint file and continue with Step 1.

### Step 1: Read Project Context

Read the root CLAUDE.md. Extract:
- Reference files relevant to the task area
- Review conventions that apply
- Architecture constraints

If no CLAUDE.md exists, note: "Consider running `/chronicle` to set up project context."

Also check for `~/.claude/fellowship.json` (the user's personal Claude directory). If it exists, read it and note any non-default settings. These will be included in the Session Context block under Architecture Notes so downstream skills (quest, lembas) are aware of the active configuration.

### Step 2: Understand the Task

Ask one focused question:

> "What are you working on? (One sentence describing the task and which area of the codebase it touches.)"

If invoked by quest, the task description is passed in — skip the question.

### Step 3: Identify Package Scope

From the task description, identify which package(s) are involved:
1. Match the task area to a package directory (e.g., `packages/<name>/`, `apps/<name>/`)
2. If ambiguous, check the monorepo structure (`ls` top-level directories) and ask the user
3. If the task spans multiple packages, list all affected packages

This scope constrains all downstream scanning and verification.

If a package-level CLAUDE.md exists (e.g., `packages/<name>/CLAUDE.md`), read it and merge its conventions with the root CLAUDE.md. Package-level conventions override root conventions where they conflict.

### Step 4: Scan for Relevant Files

Use the Explore agent (Task tool with subagent_type=Explore) to find files related to the task description. **Scope the search to the identified package(s)** — do not scan the entire monorepo.

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
- **Carry forward.** The Session Context block is referenced by lembas and quest throughout the session.
- **One question.** Don't interrogate the user. One focused question, then do the work.
