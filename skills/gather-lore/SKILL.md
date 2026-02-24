---
name: gather-lore
description: Use BEFORE writing any code in an unfamiliar area. Studies reference files from CLAUDE.md to extract conventions, then constrains code generation to follow those patterns exactly. Prevents "wrong approach" code review rework.
---

# Gather Lore — Study Patterns Before Generating Code

## Overview

Two-pass workflow: study existing code to extract the specific patterns in play, then generate new code constrained by what you observed. Run this before writing anything in an area where you've been burned by "wrong approach" review feedback.

## When to Use

- Writing code in a part of the codebase you haven't worked in before
- You've had PRs rejected for "wrong approach" in this area
- The task touches patterns you're unsure about (DI, error handling, data access)

## Process

### Pass 1: Study

**1. Find reference files.**

Check CLAUDE.md for a `## Reference Files` section. If it exists, read the files listed for the relevant area.

If no reference files are documented, ask the user:
> "I need 1-2 examples of files that do something similar to what we're building, that your reviewer would approve of. Can you point me to any?"

If the user can't identify any:
> "Let me look at recent merges in this area to find approved patterns."
> Run: `git log --oneline --diff-filter=A -- [relevant directory] | head -10` to find recently added files.

**2. Extract the specific patterns.**

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

### Things NOT done
- [Patterns conspicuously absent — no direct DB calls, no raw HTTP, etc.]
```

**3. Confirm with the user.**

Present the analysis: **"Here's what I observed. Does this look right? Anything I'm missing or misreading?"**

### Pass 2: Generate

Now write the code with these constraints:

1. **Match structure exactly.** Same file organization, same section ordering.
2. **Use the same abstractions.** Don't invent alternatives.
3. **Copy error handling verbatim.** Same types, same propagation style.
4. **Follow naming patterns.** If references use `getUserById`, don't write `fetchUser`.
5. **Respect the absences.** If reference files don't do something directly, neither should you.
6. **When in doubt, be more similar, not less.**

### Pass 3: Diff Check

After generating, compare your output against the reference:

```
## Deviation Report

| Aspect | Reference Pattern | My Code | Intentional? |
|--------|------------------|---------|-------------|
| [e.g., error type] | [AppError] | [custom Error] | No — fix |
| [e.g., return type] | [Result<T>] | [Option<T>] | Yes — different semantics |
```

Fix all unintentional deviations. For intentional ones, note why they're necessary — these are the things most likely to get review comments, so be prepared to justify them.

## Output

Your generated code should look like it was written by the same person who wrote the reference files. If a reviewer can't distinguish new code from existing code by style alone, you've succeeded.

## Updating Conventions

If this study pass reveals patterns not yet captured in CLAUDE.md, offer to add them:
> "I noticed [pattern] in the reference files that isn't documented in CLAUDE.md yet. Want me to add it to the Review Conventions section?"
