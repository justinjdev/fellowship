---
description: Interactive guide to fellowship. Walks you through a real task using the structured research-plan-implement flow, then shows you what's next.
---

# Fellowship Guide — Learn by Doing

## Overview

This guide teaches fellowship by running a real task on the user's codebase. It does NOT explain concepts upfront — it demonstrates them, then names them afterward.

**Do not mention** gates, lembas, tome, worktree, balrog, warden, council, palantir, herald, eagles, errands, companies, or scouts until Act 3 (Graduation). Use plain language throughout Acts 1–2.

## Act 1: Pitch

Present this to the user:

> **Fellowship helps Claude work through complex tasks more carefully** — researching before planning, planning before coding. Each step has a checkpoint where you review the work before moving on.
>
> Let's try it on something real in your codebase.

Then ask using AskUserQuestion:

> **What would you like to work on?**
>
> Pick something real — a bug to fix, a feature to add, or a refactor to do. It should be non-trivial (more than a one-line change) but completable in a single session.

Wait for their response. This is the task description used for the rest of the guide.

## Act 2: Guided Quest

### Setup

1. Determine the current branch: `git branch --show-current`
2. Create a new branch: `git checkout -b guide/<slug>` where `<slug>` is a short kebab-case summary of the task (e.g., `guide/fix-auth-redirect`)
3. Read the project's CLAUDE.md (if it exists) to understand conventions and reference files. Do not present this to the user — absorb it silently.

### Stage 1: Research

**Goal:** Understand the relevant parts of the codebase before making a plan.

1. Use the Explore agent (Agent tool with subagent_type=Explore) to find files related to the task. Focus on:
   - Files that will likely need modification
   - Files that define patterns to follow
   - Test files for the affected area
   - Types, configs, or dependencies that constrain the work
2. If CLAUDE.md lists reference files for the relevant area, read them and note the patterns (naming, error handling, structure, data flow). Do not present the raw pattern analysis — internalize it.
3. Present a structured research summary to the user:

> **Here's what I found:**
>
> **Relevant files:**
> - `path/to/file.go:20-45` — [why it matters]
> - `path/to/other.go:10-30` — [why it matters]
>
> **How it works:** [2-3 sentence explanation of the relevant system]
>
> **Approach:** [1-2 sentences on how you'd tackle the task based on what you found]
>
> **Does this look right before I make a plan?**

Wait for the user's response. Revise if they correct anything.

### Stage 2: Plan

**Goal:** Outline explicit steps before writing any code.

1. Enter plan mode (EnterPlanMode) and draft a concise implementation plan:
   - Numbered steps with specific file paths
   - What each step changes and why
   - Which tests to add or modify
   - Expected behavior after each step
2. Present the plan and ask:

> **Does this plan look good before I start coding?**

Wait for approval. Revise if needed. Exit plan mode (ExitPlanMode) once approved.

### Context Checkpoint (silent)

Before starting implementation, compress your context. Discard verbose research output (file contents, search results) and carry forward only:
- The approved plan
- Key file paths and line ranges
- Patterns/conventions to follow
- The task description

Do not mention this compression to the user.

### Stage 3: Implement

**Goal:** Execute the plan, writing tests alongside code.

1. Work through the plan step by step
2. Write tests for new behavior (before or alongside implementation, depending on what's natural for the change)
3. Run tests after implementation to verify: `output=$(test_command 2>&1) || { echo "$output" | tail -40; false; }; echo "✓"`
4. When implementation is complete and tests pass, present a summary:

> **Implementation complete.** Here's what changed:
>
> **Files modified:**
> - `path/to/file.go` — [what changed]
> - `path/to/test.go` — [what was tested]
>
> **Tests:** All passing
>
> **Ready for me to wrap this up as a PR?**

Wait for approval.

### Complete

1. Stage and commit the changes with a descriptive commit message
2. Push the branch: `git push -u origin guide/<slug>`
3. Create a PR: `gh pr create --title "<title>" --body "<body>"`
4. Present the PR URL to the user

## Act 3: Graduation

After the PR is created, present:

> **That structured flow is called a quest** — research before planning, planning before coding, with checkpoints at each stage so you stay in control.
>
> Two ways to use this going forward:
>
> - **`/quest`** — Run a quest with the full toolkit: automated convention checks, adversarial testing, crash recovery. Same rhythm you just experienced, with more guardrails.
> - **`/fellowship`** — Run multiple quests in parallel. A coordinator manages separate tasks in isolated branches, each producing its own PR.

If `~/.claude/fellowship.json` does not exist, also offer:

> Want me to set up a config file so your first `/quest` auto-approves the early checkpoints? (Recommended — it keeps the flow moving while still pausing before code is written.)

If the user says yes, create `~/.claude/fellowship.json` with:

```json
{
  "gates": {
    "autoApprove": ["Research", "Plan"]
  }
}
```

End with:

> **Run `/quest` next time you have a task, or `/fellowship` when you have several.**
