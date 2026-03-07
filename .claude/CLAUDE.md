# Fellowship Project

## Preferences

- Do not commit docs/ files (design docs, plans). They are local working artifacts, not part of the repo.

## Site

- The GitHub Pages site lives in `site/` (SvelteKit static site).
- When adding, changing, or removing skills, agents, configuration options, or other user-facing features, update the corresponding page in `site/src/routes/`.
- Page mapping: skills → `skills/`, agents → `agents/`, config → `configuration/`, changelog → `changelog/`.
- Do not update the site for internal refactors that don't change user-facing behavior.
