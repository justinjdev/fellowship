---
name: gather-lore
description: Use BEFORE writing any code in an unfamiliar area. Studies reference files from CLAUDE.md to extract conventions and patterns. Prevents "wrong approach" code review rework by understanding how things are done before writing anything.
---

# Gather Lore — Study Patterns Before Writing Code

## Overview

Studies existing code to extract the specific patterns and conventions in play. Run this during research — before planning or writing anything — in areas where conventions matter. The patterns you extract here flow into the plan and constrain implementation downstream.

Code generation and deviation checking happen later in the workflow: implementation applies these patterns (quest Phase 3 / TDD), and warden verifies compliance (quest Phase 4).

## When to Use

- Entering a part of the codebase you haven't worked in before
- You've had PRs rejected for "wrong approach" in this area
- The task touches patterns you're unsure about (DI, error handling, data access)

## Process

### Step 1: Find Reference Files

Check CLAUDE.md for a `## Reference Files` section. If it exists, read the files listed for the relevant area.

If no reference files are documented, ask the user:
> "I need 1-2 examples of files that do something similar to what we're building, that your reviewer would approve of. Can you point me to any?"

If the user can't identify any:
> "Let me look at recent merges in this area to find approved patterns."
> Run: `git log --oneline --diff-filter=A -- [relevant directory] | head -10` to find recently added files.

### Step 2: Extract Patterns

Read each reference file and produce a structured analysis. Be exhaustive — the patterns you miss are the ones that get flagged in review:

```
## Patterns in [filename]

### Structure
- [How the file is organized — what comes first, ordering]
- [Import grouping and ordering]
- [Export patterns]

### Dependencies
- [How external dependencies are accessed]
- [How internal dependencies are accessed]
- [Any DI/context patterns]

### Error Handling
- [Error types used]
- [How errors are propagated]
- [How errors are surfaced to callers]

### Data Flow
- [Input validation — where and how]
- [Transformations — where data changes shape]
- [Output — how results are returned]

### Naming
- [Variable naming patterns]
- [Function naming patterns]
- [Type/interface naming patterns]

### Things NOT Done
- [Patterns conspicuously absent — no direct DB calls, no raw HTTP, etc.]
```

### Step 3: Confirm with the User

Present the analysis: **"Here's what I observed. Does this look right? Anything I'm missing or misreading?"**

### Step 4: Update Conventions

If this study reveals patterns not yet captured in CLAUDE.md, offer to add them:
> "I noticed [pattern] in the reference files that isn't documented in CLAUDE.md yet. Want me to add it to the Review Conventions section?"

## Output

A structured pattern analysis that serves as a constraint document for downstream work. The patterns extracted here should be carried forward through lembas compaction so they inform the plan and constrain implementation.

## Key Principles

- **Study, don't generate.** This skill extracts patterns. Code generation happens during implementation, constrained by what you found here.
- **Be exhaustive in extraction.** The patterns you miss are the ones that get flagged in review.
- **Absences matter as much as presences.** What the reference files don't do is as important as what they do.
- **When in doubt during later implementation, be more similar to reference files, not less.**
