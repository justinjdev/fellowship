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

## Step 2: Validate Docs

Invoke the `validate-docs` skill using the Skill tool. Review any flagged issues and ask the user whether to fix them now or proceed anyway.

If fixing: make the updates before continuing. Renaming an "Unreleased" changelog section to the new version is handled in Step 3 below — skip that specific fix here.

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
  tag            ✓ v{version} pushed
  marketplace    ✓ bumped to v{version}

CI will build binaries from the tag.
```
