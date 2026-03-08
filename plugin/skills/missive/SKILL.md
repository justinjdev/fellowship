---
name: missive
description: Fetches GitHub issue context for quest spawning. Parses issue references, retrieves structured data via gh, and produces branch suggestions and PR keywords. Used standalone or as input to quest orchestration.
---

# Missive — GitHub Issue Context

## Overview

Fetches structured context from one or more GitHub issues to inform quest planning. Extracts titles, bodies, labels, comments, and produces branch name suggestions and PR close keywords. Designed for both standalone use (display to user) and as input to Gandalf's quest spawning.

## When to Use

- Before spawning a quest from a GitHub issue
- When you need structured issue context for planning
- Invoked with `/missive #42` or `/missive 42 51 67`

## Input

Accepts one or more GitHub issue references as arguments:
- Single: `#42` or `42`
- Multiple: `#42 #51 #67` or `42 51 67`
- Mixed: `#42 51 #67`

If no arguments are provided, respond with:
> "Usage: `/missive <issue>...` — provide one or more issue numbers (e.g., `/missive #42` or `/missive 42 51 67`)."

## Process

### Step 1: Parse Issue Numbers

Extract all issue numbers from the args using the pattern `#?\d+`. Strip any `#` prefix to get the raw number.

### Step 2: Load Configuration

Read `~/.claude/fellowship.json` if it exists. Extract:
- `branch.pattern` — branch name pattern with placeholders (default: `null`, effective: `fellowship/{slug}`)
- `branch.ticketPattern` — regex for ticket extraction (default: `[A-Z]+-\d+`)
- `issues.autoClose` — whether PRs should auto-close issues (default: `true`)

If the config file doesn't exist, use all defaults.

### Step 3: Verify gh CLI

Before fetching, confirm `gh` is available:

```bash
gh --version
```

If `gh` is not installed or not authenticated, respond with:
> "The `gh` CLI is required but not available. Install it from https://cli.github.com/ and authenticate with `gh auth login`."

### Step 4: Fetch Issue Data

For each issue number, run:

```bash
gh issue view <number> --json title,body,labels,comments,assignees,milestone
```

If an issue is not found (non-zero exit), note the error and continue with remaining issues. Do not abort the entire operation for a single missing issue.

### Step 5: Build Structured Output

For each successfully fetched issue, produce a structured block:

#### Context

- **Title:** The issue title
- **Body:** The issue body, truncated to 2000 characters if longer. Append `... [truncated]` if truncated.
- **Labels:** Comma-separated list of label names, or "none" if empty
- **Comments:** Up to 5 most recent comments, each truncated to 500 characters. Include author and timestamp. Omit if no comments.
- **Assignees:** Comma-separated list, or "none"
- **Milestone:** Milestone title, or "none"

#### Branch Suggestion

Resolve the branch name:
1. If `branch.pattern` is configured: substitute placeholders per the quest skill's rules, but override `{slug}` with a slug derived from the issue title (not the task description). The `{ticket}` placeholder matches `branch.ticketPattern` against the issue title — if no match, replace with empty string. If `{author}` is in the pattern but `branch.author` is not set, replace with empty string. After all substitutions, collapse any resulting double-separators (e.g., `//` → `/`, `--` → `-`).
2. If no pattern configured (default): use `fellowship/<number>-<slugified-title>` — incorporating the issue number for traceability (e.g., `fellowship/42-fix-auth-bug`).

Slug generation: lowercase the issue title, replace spaces with hyphens, strip non-alphanumeric characters (except hyphens), collapse consecutive hyphens, max 50 characters.

#### PR Keywords

- If `issues.autoClose` is true (default): `Closes #<number>`
- If `issues.autoClose` is false: omit the close keyword, note: "Auto-close disabled — link issue manually."

### Step 6: Format Output

Output one block per issue in this format:

```
## Issue #<number>: <title>

### Context
**Labels:** <labels>
**Assignees:** <assignees>
**Milestone:** <milestone>

<body text>

**Recent Comments:**
- [@<author> <timestamp>]: <comment text>
- [@<author> <timestamp>]: <comment text>

### Branch
`<resolved branch name>`

### PR Keywords
`Closes #<number>`
```

If multiple issues were fetched, output all blocks sequentially.

If any issues failed to fetch, append a summary:

```
### Errors
- Issue #<number>: <error message>
```

## Key Principles

- **Fail gracefully.** One bad issue number shouldn't block the rest.
- **Truncate aggressively.** Issue bodies and comments can be enormous — cap them to keep context lean.
- **Config-aware.** Respect the user's branch naming and auto-close preferences.
- **Composable.** Output is structured so Gandalf or quest can consume it directly without re-parsing.
