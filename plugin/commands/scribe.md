---
description: Create a reusable quest template for a specific type of task (e.g., "API endpoint", "migration"). Encodes project-specific rules and conventions into phase guidance that loads automatically during quests.
---

# Scribe Template — Create Quest Templates

## Overview

Creates quest templates that encode **project-specific** knowledge — conventions Claude wouldn't know, domain rules that aren't in code, team workflows that matter. Templates are worthless if they contain generic advice ("write tests," "follow patterns"). They're valuable when they capture things like "payments changes require idempotency tests" or "all API routes need OpenAPI spec updates."

## When to Use

- Setting up fellowship for a new project
- After noticing quests repeatedly miss a project convention
- When a category of work has domain-specific requirements

## Process

### 1. Gather Context

Ask the user:

- **What kind of quest is this template for?** (e.g., "API endpoint," "database migration," "UI component," not generic categories like "bugfix")
- **What do quests of this type consistently get wrong or miss?** This is the core value — things Claude wouldn't infer from code alone.
- **Are there project-specific gates or checks?** (e.g., "security review required for auth changes," "design approval before UI work")

Then investigate the codebase:

- Read CLAUDE.md, CONTRIBUTING.md, and any project conventions docs
- Look at recent PRs or commits of the same type for patterns
- Identify project-specific tooling, scripts, or workflows (e.g., custom test runners, migration generators, code generators)

### 2. Draft the Template

Write a template with YAML frontmatter and one section per quest phase. The lifecycle has four phases, so a complete template has four sections. Every line of guidance must be **specific to this project** — if the advice would apply to any codebase, delete it.

Read the shipped `example` template first (`plugin/skills/lorebook/templates/example.md` in the installed plugin) for a worked one at the right specificity.

**Format:**

```markdown
---
name: {name}
description: {one-line description}
keywords: [{comma-separated trigger words}]
---

## Research Guidance
{what to read and understand first; the project's prior art for this work}

## Plan Guidance
{what a plan for this kind of work must name before it is complete}

## Implement Guidance
{project-specific rules, generators, and commands to use while writing}

## Review Guidance
{what adversarial review, convention review, and verification must confirm
 before the PR — Review is the last phase, so anything that must be true
 before the PR opens belongs here}
```

Emit all four sections. Leave one out only when you genuinely have nothing project-specific for that phase; never fold Review guidance into Implement, and never add a fifth section — `/lorebook` maps exactly these four.

**Rules for good templates:**

- Every bullet must reference something concrete: a file path, a tool, a convention, a domain rule
- If you can't point to a specific project artifact or convention, the bullet doesn't belong
- Keywords should match how the user naturally describes this kind of work
- Fewer, sharper bullets beat comprehensive generic lists

### 3. Choose Placement

- **Project template** (`.claude/fellowship-templates/{name}.md`): Checked into the repo. Shared with the team. Use for conventions that apply to all contributors.
- **User template** (`~/.claude/fellowship-templates/{name}.md`): Personal. Use for individual workflow preferences.

Default to project placement unless the user specifies otherwise.

### 4. Write and Confirm

Write the template file. Show the user what was created and where. Remind them that templates are loaded by `/lorebook` at the start of each quest phase — no other setup needed.

## Anti-Patterns

Do NOT create templates that contain:

- Generic software engineering advice ("write tests first," "keep changes small")
- Advice that duplicates CLAUDE.md or the quest skill's built-in phase guidance
- Aspirational rules nobody actually follows
- Kitchen-sink checklists — if a template has 10+ bullets per phase, it's too broad
- A section for a phase that no longer exists. The lifecycle is Research, Plan, Implement, Review; a `## Onboard Guidance` or `## Complete Guidance` section is never read.
