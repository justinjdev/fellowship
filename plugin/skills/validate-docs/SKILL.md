---
name: validate-docs
description: Validate that site and README documentation is current. Report-only — flags issues without modifying anything.
---

# Validate Docs

Checks that documentation is current against the codebase. Reports issues; does not fix them.

## Step 1: Changelog (site)

Read `site/src/routes/changelog/+page.svelte`.

- If there is an `<!-- unreleased -->` section, flag it: changelog has unreleased changes that need a version.
- If there is no unreleased section, check commits since the last tag (`git log $(git describe --tags --abbrev=0)..HEAD --oneline --no-merges`). If there are feat/fix commits not reflected in the changelog, flag it.

## Step 2: Changelog (README)

Read `README.md` and find the `## Changelog` section.

Check commits since the last tag. If there are feat/fix commits not covered by the latest README changelog entry, flag it.

## Step 3: Skills and commands page

Read `site/src/routes/skills/+page.svelte`.

List all skills in `plugin/skills/*/SKILL.md` and all commands in `plugin/commands/*.md`. Cross-reference against what's documented on the skills page. Flag any that are present in the plugin but missing from the page.

## Step 4: Agents page

Read `site/src/routes/agents/+page.svelte`.

List all agents in `plugin/agents/*.md`. Cross-reference against what's documented on the agents page. Flag any that are present in the plugin but missing from the page.

## Step 5: Report

If no issues:

```
Docs validation

  Site changelog  ✓
  README          ✓
  Skills page     ✓
  Agents page     ✓

  ✓ All docs current
```

If issues found:

```
Docs validation

  Site changelog  ✗ unreleased changes present
  README          ✓
  Skills page     ✗ missing: validate-docs
  Agents page     ✓

  2 issues found
```
