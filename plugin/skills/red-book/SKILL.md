---
name: red-book
description: Use after receiving PR review feedback. Extracts conventions from reviewer comments and offers to add them to CLAUDE.md. Closes the convention learning loop.
---

# Red Book — Learn From PR Reviews

## Overview

Extracts conventions from PR review feedback and adds them to CLAUDE.md. This is the missing piece of the convention learning loop: chronicle bootstraps conventions, gather-lore studies them, warden enforces them, and this skill captures new ones from real reviewer feedback.

## When to Use

- After receiving PR review comments that reveal a convention you didn't know about
- After a "wrong approach" rejection — the reviewer's feedback is the most valuable signal
- Periodically, to process accumulated review feedback across recent PRs

## Process

### Step 1: Gather Feedback

Ask the user for the source:

> "Paste the review comments, or give me a PR URL and I'll read the comments."

If given a PR URL, use `gh api repos/{owner}/{repo}/pulls/{number}/comments` and `gh api repos/{owner}/{repo}/pulls/{number}/reviews` to fetch review comments.

### Step 2: Classify Comments

For each review comment, classify it:

| Category | Description | Action |
|----------|-------------|--------|
| **Convention** | "We don't do it that way" — reveals a pattern or rule | Extract as a convention |
| **Bug/Logic** | Points out a functional error | Skip — not a convention |
| **Nit/Style** | Minor formatting preference | Extract only if it recurs across reviews |
| **Question** | Reviewer asking for clarification | Skip — not a convention |

Present the classification to the user: **"Here's how I categorized the feedback. Any I got wrong?"**

### Step 3: Extract Conventions

For each comment classified as **Convention**, formalize it:

```
- **[Category]: [Rule]** — [Why, from reviewer's comment]
  Reference: [file:line from the PR where this was flagged]
```

Categories should match existing CLAUDE.md sections (e.g., Structure, Error Handling, Naming, Data Flow). If a comment doesn't fit an existing category, propose a new one.

### Step 4: Check for Duplicates

Read the current CLAUDE.md `## Review Conventions` section. For each extracted convention:
- If it's already documented: skip, note that it was missed during implementation (warden should have caught it)
- If it's a refinement of an existing rule: propose updating the existing rule
- If it's new: propose adding it

### Step 5: Update CLAUDE.md

Present the proposed additions/updates:

> "Here's what I'd add to CLAUDE.md. Approve, edit, or skip each one."

For approved conventions, add them to the `## Review Conventions` section under the appropriate category. If the section doesn't exist, create it.

### Step 6: Cross-Reference

Check if the new conventions would be caught by existing skills:
- **gather-lore**: Would studying reference files have revealed this pattern? If yes, note which reference file demonstrates it.
- **warden**: Is the convention specific enough for warden to check mechanically? If not, refine the wording until it is.

Offer to add reference file entries to `## Reference Files` if appropriate.

## Key Principles

- **Reviewer feedback is the highest-signal source.** It tells you what actually gets flagged, not what might get flagged.
- **One convention per comment.** Don't over-generalize from a single piece of feedback.
- **Specific and actionable.** "Use proper error handling" is useless. "Service errors must use AppError with HTTP status codes, not raw Error" is useful.
- **Warden-checkable.** If you can't imagine warden mechanically verifying the rule, it's too vague.
