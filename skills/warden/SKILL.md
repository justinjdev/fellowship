---
name: warden
description: Use AFTER writing code and BEFORE submitting a PR. Simulates a strict code reviewer by comparing changes against reference files and documented conventions. Catches "we don't do it that way" feedback before your human reviewer does.
---

# Warden — Catch Convention Violations Before Your Reviewer Does

## Overview

Strict review simulation. Compares your code against the project's reference files and documented conventions, flagging every deviation. The goal is to catch the feedback that causes multi-day review rework *before* you open the PR.

## When to Use

- Code is functionally complete and you're about to open a PR
- Any time you've written more than ~50 lines of new code
- Working in an area where past reviews have been painful

## Process

### Step 1: Gather Context

Read three things:

1. **Your changes** — every file modified or created in this PR
2. **Reference files** — from CLAUDE.md `## Reference Files` section for the relevant area
3. **Conventions** — from CLAUDE.md `## Review Conventions` section

If reference files aren't documented, ask:
> "I need reference files to compare against. Can you point me to a similar file that your reviewer would approve of?"

If conventions aren't documented, proceed with reference file comparison only and note: "You should run `/chronicle` to build up your conventions list."

### Step 2: Convention Check

For every rule in `## Review Conventions`, check compliance:

```
| # | Convention | Status | Detail |
|---|-----------|--------|--------|
| 1 | [rule text] | PASS/FAIL/N/A | [specific evidence] |
| 2 | ... | ... | ... |
```

### Step 3: Reference Comparison

Compare your code against reference files across every dimension:

**Structure**
- [ ] File organization matches reference
- [ ] Import ordering matches reference
- [ ] Section ordering matches reference

**Patterns**
- [ ] Dependency access matches reference approach
- [ ] Error handling follows same pattern (types, propagation, messaging)
- [ ] Data flow follows same layers
- [ ] No bypassed abstractions (e.g., direct DB calls when reference uses repository)

**Naming**
- [ ] Variable naming matches reference style
- [ ] Function naming matches reference verb patterns
- [ ] Type/interface naming matches reference conventions

**Absences**
- [ ] Nothing added that reference files conspicuously avoid
- [ ] No "improvements" that deviate from established patterns

**Hygiene**
- [ ] No unused imports or dependencies
- [ ] No dead code or commented-out blocks
- [ ] No TODO comments (unless project convention)
- [ ] All new dependencies are actually used

### Step 4: Report

Split findings into two categories:

**BLOCKING — fix before PR:**
- Convention violations (documented rules that are broken)
- Structural deviations from reference patterns
- Unused imports/dependencies
- Bypassed abstraction layers
- Naming inconsistencies with reference files

**ADVISORY — consider fixing:**
- Minor style differences that might trigger a nit
- Patterns that work but differ from reference
- Things that might prompt "have you considered..." feedback

For each finding:
```
[BLOCKING/ADVISORY] [file:line]
Issue: [what's wrong]
Reference: [what the reference file does instead]
Fix: [what to change]
```

### Step 5: Fix

Offer to fix all BLOCKING issues. For ADVISORY items, let the user decide.

After fixing, re-run steps 2-3 to verify fixes didn't introduce new issues.

### Step 6: Learn

If this review caught things not yet in CLAUDE.md conventions, offer to add them:
> "The warden caught [N] issues not covered by documented conventions. Want me to add these to CLAUDE.md so they're caught automatically next time?"

This is how the conventions list grows over time without requiring manual documentation effort.

## Philosophy

**Be harsher than the reviewer.** False positives are cheap — a flagged non-issue costs you 5 seconds to dismiss. A missed issue costs days in review rework. Err aggressively on the side of flagging.

**Compare, don't evaluate.** This skill doesn't judge whether your code is "good." It judges whether it *matches the established patterns.* Working code that violates conventions will still get flagged in review. The question isn't "does this work?" — it's "will this pass?"
