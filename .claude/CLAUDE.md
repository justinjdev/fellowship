# Fellowship Project

## Preferences

- Do not commit docs/ files (design docs, plans). They are local working artifacts, not part of the repo.
- Each issue gets its own separate git branch unless explicitly specified otherwise. Do not re-use branches across issues.

## Site

- The GitHub Pages site lives in `site/` (SvelteKit static site).
- When adding, changing, or removing skills, agents, configuration options, or other user-facing features, update the corresponding page in `site/src/routes/`.
- Page mapping: skills → `skills/`, agents → `agents/`, config → `configuration/`, changelog → `changelog/`.
- Do not update the site for internal refactors that don't change user-facing behavior.

## Releasing

1. Bump version in `.claude-plugin/plugin.json`
2. Commit, tag (`v<version>`), and push both the commit and tag. CI (goreleaser) builds binaries on tag push.
3. Bump the version in `~/git/claude-plugins/.claude-plugin/marketplace.json` and push that repo.
