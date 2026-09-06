---
name: lembas
description: Use between workflow phases or when context feels bloated. Writes a structured checkpoint capturing task, findings, files, state, and next steps, then continues from that summary instead of the full history. Invoke standalone or automatically between quest phases.
---

# Lembas — Intentional Context Compression

## Overview

Compresses the current conversation into a structured summary to keep the context window in the "smart zone." This is the intentional compaction pattern from context engineering — proactively trimming context between phases rather than waiting for overflow.

Reasoning quality degrades as a context window fills — well before it overflows. This skill is the mechanism for staying in the sweet spot by compacting early and often.

## When to Use

- Between phases of the quest workflow (invoked automatically by `/quest`)
- After any stretch of verbose output you would not re-read (build logs, long file reads, exploratory searches, failed-attempt noise) — if in doubt, compact
- Before switching focus within a session
- Standalone via `/lembas`

## Process

### Step 1: Identify Current Phase

Determine what just completed:
- **Research:** Understanding the system, identifying files
- **Plan:** Outlining steps, getting approval
- **Implement:** Writing code, running tests
- **Review:** Adversarial review, conventions, verification, PR
- **Ad hoc:** No formal phase — general work

### Step 2: Extract Essentials

Review the conversation and extract only what matters for the next phase. Be aggressive about discarding noise:

**Keep:**
- Decisions made and their rationale
- Files identified with specific line ranges
- Constraints discovered
- Open questions that still need answers
- Test results (pass/fail, not full output)

**Discard:**
- Raw grep/search output (keep only the conclusions)
- Full file contents (keep only relevant line ranges)
- Verbose build/test output (keep only the verdict)
- Exploratory dead ends (keep only what was learned)
- Repeated information

### Step 3: Produce Compacted Context Block

Output in this exact format:

```
## Compacted Context

### Phase Completed: [Research | Plan | Implement | Review | Ad hoc]

### Task
[one-line description, carried forward from Session Context]

### Package(s)
[package name(s) and path(s), carried forward from Session Context]

### Key Findings
- [decisions made this phase]
- [constraints discovered]
- [patterns identified]

### Files
- [file:lines] — [what's relevant and why]
- [file:lines] — [what's relevant and why]

### Current State
- [what's been done so far]
- [what's working / what's broken]

### Next Phase
- [what needs to happen next]
- [open questions to resolve]
```

### Step 4: Persist Checkpoint

Write the Compacted Context block to `.fellowship/checkpoint.md` in the data
directory of the tree you are working in — your worktree's, absolute, when
you are a fellowship teammate; the repo root's otherwise — so it survives
session crashes and context exhaustion. (`.fellowship/` is the default data
directory; users can override it via `dataDir` in `~/.claude/fellowship.json`.)

1. Create `.fellowship/` directory in that tree if it doesn't exist
2. Write the Compacted Context block to `.fellowship/checkpoint.md` with a timestamp header:

```
<!-- Checkpoint: YYYY-MM-DD HH:MM -->
<!-- Phase: [completed phase] -->
<!-- Branch: [current git branch] -->

[Compacted Context block from Step 3]
```

Add `.fellowship/*` to `.gitignore` but keep `.fellowship/config.json` trackable (`!.fellowship/config.json`) — checkpoints are developer-local ephemeral state, not shared via git. They only need to survive a session crash, not persist across machines.

### Step 5: Continue From the Summary

The checkpoint is now the working context. From here on, reason from the Compacted Context block rather than the conversation that produced it: quote the block when you need a fact, and re-read a file rather than scrolling back for what it said. Do not ask the user to run `/compact` — you cannot run it, and the checkpoint is what carries the session forward either way.

Say what you kept, in one line, so the next phase starts from the block and not from memory:

> "Checkpoint saved to `.fellowship/checkpoint.md` (phase: <phase>). Continuing from the summary above. If this session dies, quest's Research step 0 or `/rekindle` will pick it up."

## Key Principles

- **Aggressive compression.** If in doubt about whether to keep something, discard it. You can always re-read a file; you can't un-bloat a context window.
- **Structured format.** The template ensures nothing critical is lost while everything noisy is dropped.
- **Phase awareness.** What you keep depends on what's coming next, not what just happened.
- **Frequency over perfection.** Compact often with a good-enough summary rather than rarely with a perfect one.
- **Persist to survive.** Always write the checkpoint to `.fellowship/`. Sessions are ephemeral; the filesystem outlasts them. If context fills up or a session crashes, the checkpoint is the lifeline.
- **Compaction is what you do next, not what you ask for.** Writing the block is only half of it; the other half is working from the block afterwards.
