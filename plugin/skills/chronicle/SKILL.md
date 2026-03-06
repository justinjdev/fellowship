---
name: chronicle
description: Use when joining a new codebase or starting to use AI tooling on an existing project. Interactively walks through the codebase to extract conventions, identify reference files, and generate a project-specific CLAUDE.md conventions section. Run this once per codebase, then maintain with PR feedback capture.
---

# Chronicle — Bootstrap Conventions From Your Codebase

## Overview

Interactive skill that walks through your codebase and extracts the implicit conventions into explicit, AI-readable rules. Produces a `## Reference Files` section and a `## Review Conventions` section for your CLAUDE.md.

This is a one-time setup skill. After running it, conventions are maintained incrementally through PR feedback capture.

## Process

### Step 1: Understand the Codebase Shape

Ask the user:
1. "What kind of codebase is this?" (monorepo, single service, library, etc.)
2. "What areas do you work in most often?"
3. "Who are your primary code reviewers?"

Then explore the directory structure. Identify:
- Where source code lives
- How it's organized (by feature, by layer, by domain)
- What test patterns exist
- Config files that enforce standards (linters, formatters, CI checks)

### Step 2: Find Reference Files

For each area the user works in, ask:

> "Point me to 1-2 files in [area] that you know passed review cleanly — files your reviewer would consider 'the right way to do it.' If you're not sure, point me to the most recently merged PR in this area and I'll look at what was approved."

If the user can't identify reference files, help them:
- Look at recent merged PRs: `git log --oneline --merges -20`
- Find files with few review iterations
- Ask: "Which files does your reviewer point to when they say 'do it like X'?"

For each reference file identified, record:
```
### [Category]: [file path]
- Approved by: [reviewer, if known]
- Good example of: [what pattern this demonstrates]
- Last updated: [date of last significant change]
```

### Step 3: Extract Conventions by Comparison

Read 3-5 reference files across different areas. For each one, extract observable patterns in these categories:

**Structure & Organization**
- File layout (imports, types, constants, logic, exports)
- Naming conventions (files, functions, variables, types)
- Module/package organization

**Architecture & Patterns**
- How dependencies are accessed (DI, imports, singletons, context)
- Error handling (custom types, propagation style, recovery)
- Data flow (layers, where validation happens, where transformations happen)
- What abstractions are used (and which are NOT — equally important)

**Style & Idioms**
- Code formatting beyond what linters enforce
- Comment style and when comments are expected
- Logging patterns
- Test organization and naming

Present each extracted convention to the user and ask: **"Is this actually a convention, or just how this one file happens to do it?"**

Only keep conventions the user confirms.

### Step 4: Capture Known Review Feedback

Ask the user:

> "Think about your last 3-5 code reviews. What comments came up that surprised you or required significant rework? These are the conventions I most need to know about."

For each piece of feedback, formalize it:
```
- **[Category]: [Rule]** — [Why / who enforces it]
  Example: [brief code example of the right way, if helpful]
```

### Step 5: Generate CLAUDE.md Sections

Produce two sections ready to paste into the project's CLAUDE.md:

**`## Reference Files`** — organized by area, with file paths and descriptions of what each demonstrates.

**`## Review Conventions`** — organized by category, with each rule including:
- What the convention is
- Why it exists (if known)
- A brief example of the right way (if the rule isn't obvious)

### Step 6: Validate

Ask the user to review the generated sections. Specifically ask:
- "Is anything here wrong or overstated?"
- "Is anything critical missing — something that always gets flagged but I didn't capture?"
- "Would your reviewer agree with these rules?"

Revise based on feedback, then write the final output to CLAUDE.md.

## Key Principles

- **Only capture real conventions, not aspirations.** If the codebase doesn't actually follow a rule consistently, don't document it as a convention.
- **Prioritize the expensive ones.** A convention that causes a 2-line fix isn't worth documenting as urgently as one that causes a full rewrite.
- **Be specific, not generic.** "Use proper error handling" is useless. "All service-layer errors must use AppError with a domain-specific error code" is useful.
- **Include the why when possible.** "We use factory methods instead of constructors because [reason]" is much more useful than just the rule, because it helps AI make judgment calls in edge cases.
