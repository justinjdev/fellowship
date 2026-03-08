---
description: Release a new version. Bumps version, updates docs/site/changelog, tags, pushes, and updates the marketplace repo.
---

# Release

Automates the full release process for fellowship.

## Step 1: Determine Version

Read the current version from `.claude-plugin/plugin.json`.

Analyze commits since the last tag (`git log $(git describe --tags --abbrev=0)..HEAD --oneline --no-merges`) and suggest a version based on conventional commit types:

- **feat:** → minor bump
- **fix:** only → patch bump
- Breaking changes → minor bump (pre-1.0: major)

Present the suggestion and ask the user to confirm or override:

```
Current version: X.Y.Z
Commits since last tag: N

Suggested next version: X.Y.Z (reason)

Enter version to release (or confirm suggested):
```

Use `AskUserQuestion` with the suggested version as default and a free-text option.

## Step 2: Audit Docs and Site

Before bumping anything, verify all documentation is current. Check each item and report status:

### Changelog (site)
Read `site/src/routes/changelog/+page.svelte`. If there is an "Unreleased" section, it should be renamed to the new version. If there is no unreleased section and there are commits since the last changelog entry, flag it as needing an update.

### Changelog (README)
Read `README.md` and check the `## Changelog` section. If the latest entry doesn't cover the commits being released, flag it.

### Skills page
Read `site/src/routes/skills/+page.svelte`. Cross-reference against `plugin/skills/` and `plugin/commands/` directories. Flag any skills or commands that exist but aren't documented.

### Agents page
Read `site/src/routes/agents/+page.svelte`. Cross-reference against `plugin/agents/` directory. Flag any agents that exist but aren't documented.

**If any flags were raised**, present them and ask the user whether to fix them now or proceed anyway. If fixing, make the updates before continuing.

## Step 3: Bump Version

Update the version string in `.claude-plugin/plugin.json`.

If the site changelog has an "Unreleased" section, rename it to the new version with the standard format:

```html
<section class="version" id="v{version-dashed}">
    <h2 class="version-heading"><a href="{base}/changelog#v{version-dashed}">v{version}</a></h2>
```

Also update the HTML comment above it from `<!-- unreleased -->` to `<!-- v{version} -->`.

## Step 4: Commit, Tag, Push

```
git add -A
git commit -m "chore: release v{version}"
git tag v{version}
git push && git push --tags
```

Verify the push succeeded. If it fails, stop and report.

## Step 5: Update Marketplace

Read `~/git/claude-plugins/.claude-plugin/marketplace.json`. Update the fellowship plugin's `version` field to the new version.

```
cd ~/git/claude-plugins
git add .claude-plugin/marketplace.json
git commit -m "bump fellowship to v{version}"
git push
```

If the marketplace repo doesn't exist at that path or the push fails, report the manual step needed.

## Step 6: Confirm

Report the release summary:

```
Released v{version}

  plugin.json    ✓ bumped
  site changelog ✓ updated
  README         ✓ current
  tag            ✓ v{version} pushed
  marketplace    ✓ bumped to v{version}

CI will build binaries from the tag.
```
