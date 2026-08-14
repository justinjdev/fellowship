---
description: Validate that site and README documentation is current. Report-only — flags issues without modifying anything.
---

# Validate Docs

> **Layout assumption:** This skill expects the standard fellowship layout — site at `site/src/routes/`, plugin at `plugin/`. If your project uses a different structure, the path checks below will not match.

## Overview

Checks that documentation is current against the codebase: the site changelog, README changelog, skills/commands page, agents page, marketplace description, and config schema. Collects all findings across Steps 1–6, then emits a single structured report in Step 7. Does not modify any files — report-only.

## Process

### Step 1: Changelog (site)

Read `site/src/routes/changelog/+page.svelte`. If the file does not exist, flag it as missing and continue to Step 2.

- If there is an `<!-- unreleased -->` section, flag it: changelog has unreleased changes that need a version.
- If there is no unreleased section, check commits since the last tag:
  ```
  git log $(git describe --tags --abbrev=0)..HEAD --oneline --no-merges
  ```
  If `git describe --tags --abbrev=0` fails (no tags exist), skip the commit comparison and note "no tags found." Otherwise, if there are feat/fix commits not reflected in the changelog, flag it.

### Step 2: Changelog (README)

Read `README.md` and find the `## Changelog` section. If `README.md` does not exist, flag it as missing and continue to Step 3.

Check commits since the last tag:
```
git log $(git describe --tags --abbrev=0)..HEAD --oneline --no-merges
```
If `git describe --tags --abbrev=0` fails (no tags exist), skip the commit comparison and note "no tags found." Otherwise, if there are feat/fix commits not covered by the latest README changelog entry, flag it.

### Step 3: Skills and commands page

Read `site/src/routes/skills/+page.svelte`. If the file does not exist, flag it as missing and continue to Step 4.

List all skills in `plugin/skills/*/SKILL.md` and all commands in `plugin/commands/*.md`. Cross-reference against what's documented on the skills page. Flag any that are present in the plugin but missing from the page.

### Step 4: Agents page

Read `site/src/routes/agents/+page.svelte`. If the file does not exist, flag it as missing and continue to Step 5.

List all registered agents from `.claude-plugin/plugin.json`. If reading from `plugin/agents/*.md` instead, ignore helper docs such as underscore-prefixed files (e.g., `_protocol.md`). Cross-reference against what's documented on the agents page. Flag any registered agents that are missing from the page.

### Step 5: Marketplace description

Read `~/git/claude-plugins/.claude-plugin/marketplace.json`. If it does not exist, skip and note "marketplace not found."

Find the fellowship plugin entry and check that:
- The skill count in `description` matches the actual number of skills in `plugin/skills/*/SKILL.md`
- The agent count in `description` matches the number of registered agents in `.claude-plugin/plugin.json` (excluding underscore-prefixed files)

Flag any mismatch.

### Step 6: Config schema

Read the `## Schema Reference` table in `plugin/commands/settings.md` — the canonical config schema. If the file or section does not exist, flag it as missing and continue to Step 7.

Extract the full set of config keys (e.g. `branch.pattern`, `gates.autoApprove`, `models.quest`) with their documented defaults. Cross-reference each key against both documentation surfaces:

1. **README** — the Configuration section's settings table AND its example JSON block
2. **Site** — `site/src/routes/configuration/+page.svelte` (the settings array and its example JSON)

Flag:
- Any schema key missing from the README table or example JSON
- Any schema key missing from the site configuration page
- Any key documented in README or on the site that is absent from the settings.md schema (stale key)
- Default values that disagree between settings.md and either surface

### Step 7: Report

Emit all findings collected from Steps 1–6. Do not output inline findings during earlier steps — accumulate them and report here.

If no issues:

```
Docs validation

  Site changelog  ✓
  README          ✓
  Skills page     ✓
  Agents page     ✓
  Marketplace     ✓
  Config schema   ✓

  ✓ All docs current
```

If issues found:

```
Docs validation

  Site changelog  ✗ unreleased changes present
  README          ✓
  Skills page     ✗ missing: some-skill
  Agents page     ✓
  Marketplace     ✗ description says 2 agents, found 4
  Config schema   ✗ issues.autoClose missing from README table

  3 issues found
```

## Key Principles

- **Report-only.** Never modify any files. Flag issues; do not fix them.
- **Accumulate before reporting.** Collect all findings across Steps 1–6, then emit the single structured report in Step 7.
- **Graceful degradation.** If a file or data source is missing, flag it as missing, note it in the report, and continue to the next step.
