---
name: scout
description: Research & analysis workflow. Investigates questions, gathers findings, optionally validates with a fresh subagent. Produces a report — no code, no PRs, no commits. Use standalone or as a fellowship teammate.
---

# Scout — Research & Analysis

## Overview

Investigates questions and analyzes codebases without producing code changes. Runs autonomously through Investigate → (Validate) → Deliver phases. When used as a fellowship teammate, the `scout` agent definition enforces read-only tool access. Also works standalone.

## When to Use

- Research questions about a codebase ("how does X work?")
- Deep analysis ("what are all the entry points for auth?")
- Data collection ("list all API endpoints and their middleware chains")

## Phase Flow

```
Investigate ──→ (Validate) ──→ Deliver
```

## Process

### Investigate

Goal: Gather thorough, factual findings about the question.

**Actions:**
1. Invoke `/council` to load task-relevant context
2. Use Explore agents (Agent tool, subagent_type=Explore) to scan relevant code paths
3. Read key files, trace call chains, understand behavior
4. Document findings with specific file paths and line references

**Investigate must produce:**
- [ ] Specific file paths and line ranges for every claim
- [ ] Clear explanation of how things work (not just where they are)
- [ ] Constraints, edge cases, and dependencies identified
- [ ] Confidence level for each finding (High/Medium/Low)

**High confidence** = verified by reading actual code at specific lines.
**Medium confidence** = inferred from patterns, naming, or partial evidence.
**Low confidence** = assumption based on conventions or incomplete information.

If findings are incomplete, keep investigating. Don't move to validation or delivery with gaps.

### Validate (conditional)

Goal: Adversarially verify findings using a fresh subagent with no context pollution.

**When to validate:**
- Deep analysis involving multiple systems or complex interactions
- Findings that will inform architectural decisions or code changes
- Questions where being wrong would waste significant downstream effort
- Any time there are Medium or Low confidence findings that matter

**When to skip validation:**
- Simple lookups ("where is X defined?")
- Straightforward questions with all High confidence answers
- Time-sensitive quick questions

**Validation procedure:**
1. Write your findings to a working file (e.g., `docs/research/<topic>.md`) — do NOT commit
2. Spawn a validator subagent via the Agent tool with subagent_type "general-purpose":

```
You are a research validator. Your job is adversarial: challenge
assumptions, verify factual claims, and flag anything wrong or unsupported.

FINDINGS TO VALIDATE:
<paste findings here, including file paths and line references>

INSTRUCTIONS:
1. For each factual claim, read the referenced file and line range.
   Does the code actually do what the finding says?
2. For each Medium/Low confidence finding, investigate independently.
   Can you confirm or refute it?
3. Produce a validation report:
   - CONFIRMED: claims you verified are correct
   - CONTESTED: claims that are wrong or misleading, with evidence
   - UNVERIFIED: claims you couldn't confirm or deny

BOUNDARIES:
- Read any file. Do NOT modify any files or run commands.
- Be adversarial — your value is in catching errors, not agreeing.
```

3. Review the validation report
4. For contested findings: re-investigate, correct or remove the finding
5. For unverified findings: downgrade confidence or investigate further

### Deliver

Goal: Send findings to the requester in a structured format.

**Fellowship teammate:** Send via `SendMessage` to the lead (Gandalf).
**Standalone:** Present findings directly to the user.

**Report format:**

```
## Scout Report: <question>

### Findings
- <finding 1> (`path/to/file.ts:42-58`)
- <finding 2> (`path/to/other.ts:10-25`)
- ...

### Confidence
- High: <list>
- Medium: <list>
- Low: <list>

### Validation
- Confirmed: <validated claims>
- Contested: <claims corrected after validation>
- Skipped: <if validation was not performed, say why>

### Files Written
- <list any research files written, noting they are not committed>
```

**Routing (fellowship only):** If the spawn prompt includes a routing target (e.g., "→ send to quest-auth-bug"), also send findings to that teammate via SendMessage in addition to the lead.

## Fellowship Integration

When running as a fellowship teammate, the `scout` agent definition restricts your tools to read-only access (Read, Glob, Grep, Agent, Skill, TaskUpdate, SendMessage). Phase metadata names for task updates: `"Investigating"`, `"Validating"`, `"Done"`. See `agents/scout.md` for full details.

## Key Principles

1. **Evidence over opinion.** Every finding needs a file path and line reference.
2. **Confidence is honest.** Don't mark something High if you inferred it.
3. **Validation catches errors.** When in doubt, validate. Fresh eyes find what you missed.
4. **No side effects.** In fellowship, tool restrictions enforce this. Standalone, respect it by convention.
