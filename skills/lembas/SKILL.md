---
name: lembas
description: Use between workflow phases or when context feels bloated. Compresses the current conversation into a structured summary capturing task, findings, files, state, and next steps. Invoke standalone or automatically between quest phases.
---

# Lembas — Intentional Context Compression

## Overview

Compresses the current conversation into a structured summary to keep the context window in the "smart zone." This is the intentional compaction pattern from context engineering — proactively trimming context between phases rather than waiting for overflow.

The ~40% context utilization mark is where reasoning quality starts to degrade. This skill is the mechanism for staying under that threshold.

## When to Use

- Between phases of the quest workflow (invoked automatically by `/quest`)
- When a conversation feels bloated after verbose output (build logs, long file reads, exploratory searches)
- Before switching focus within a session
- Standalone via `/lembas`

## Process

### Step 1: Identify Current Phase

Determine what just completed:
- **Research:** Understanding the system, identifying files
- **Plan:** Outlining steps, getting approval
- **Implement:** Writing code, running tests
- **Review:** Checking against conventions
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

Write the Compacted Context block to `tmp/checkpoint.md` (repo root) so it survives session crashes and context exhaustion:

1. Create `tmp/` directory in repo root if it doesn't exist
2. Write the Compacted Context block to `tmp/checkpoint.md` with a timestamp header:

```
<!-- Checkpoint: YYYY-MM-DD HH:MM -->
<!-- Phase: [completed phase] -->
<!-- Branch: [current git branch] -->

[Compacted Context block from Step 3]
```

The `tmp/` directory is gitignored — checkpoints are developer-local ephemeral state, not shared via git. They only need to survive a session crash, not persist across machines.

### Step 5: Trigger Built-in Compaction

After persisting the checkpoint, instruct:

> "Checkpoint saved to `tmp/checkpoint.md`. Now run `/compact` to compress the conversation window. The Compacted Context block above will be preserved as the key context. If this session dies, the next session will find the checkpoint and offer to resume."

## Key Principles

- **Aggressive compression.** If in doubt about whether to keep something, discard it. You can always re-read a file; you can't un-bloat a context window.
- **Structured format.** The template ensures nothing critical is lost while everything noisy is dropped.
- **Phase awareness.** What you keep depends on what's coming next, not what just happened.
- **Frequency over perfection.** Compact often with a good-enough summary rather than rarely with a perfect one.
- **Persist to survive.** Always write the checkpoint to `tmp/`. Sessions are ephemeral; the filesystem outlasts them. If context fills up or a session crashes, the checkpoint is the lifeline.
