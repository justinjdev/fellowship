---
description: Audit plugin skills, commands, and agents for structure, size, and naming issues.
---

# Lint Skills

Checks plugin structure against best practices. Run during development to catch issues before release.

## Step 1: Collect Files

Gather all plugin files:
- Skills: `plugin/skills/*/SKILL.md`
- Commands: `plugin/commands/*.md`
- Agents: `plugin/agents/*.md`

## Step 2: Line Count Check

For each SKILL.md and command file, count lines. Flag any file over **500 lines** with a warning:

```
⚠ plugin/skills/quest/SKILL.md — 313 lines (limit: 500)
```

Only flag files that exceed the limit. For files that pass, no output needed.

**Why 500:** Skill content loads fully on invocation. Large skills bloat context and should extract detailed content into supporting files (resources/, reference files) that load on-demand.

## Step 3: Frontmatter Validation

For each file, verify YAML frontmatter:

**Skills (SKILL.md):**
- Must have `name` field matching the directory name
- Must have `description` field
- Flag if `name` doesn't match directory (e.g., `plugin/skills/missive/SKILL.md` should have `name: missive`)

**Commands:**
- Must have `description` field
- Must NOT have `name` field (commands use filename, not frontmatter name)

**Agents:**
- No frontmatter requirements (they use a different format)

## Step 4: Name Collision Check

Check skill and command names against Claude Code built-in commands. Flag any collisions:

Built-in names to check against: `help`, `clear`, `config`, `status`, `login`, `logout`, `init`, `doctor`, `listen`, `review`, `compact`, `cost`, `memory`, `permissions`, `mcp`, `bug`, `terminal-setup`, `fast`, `slow`, `model`, `vim`, `hooks`, `install-github-app`

```
✗ plugin/skills/config/SKILL.md — "config" collides with Claude Code built-in
```

## Step 5: Report

Summarize results:

```
Plugin Lint Results

  Skills:   9 checked
  Commands: 6 checked
  Agents:   3 checked

  ✓ All checks passed
```

Or if issues were found:

```
Plugin Lint Results

  Skills:   9 checked
  Commands: 6 checked
  Agents:   3 checked

  2 issues found:
    ⚠ plugin/skills/quest/SKILL.md — 512 lines (limit: 500)
    ✗ plugin/skills/config/SKILL.md — "config" collides with built-in
```
