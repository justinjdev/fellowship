# Contributing to Fellowship

Thanks for your interest in contributing to Fellowship — a Claude Code plugin for orchestrating multi-task workflows.

## What This Repo Is

Fellowship is a Claude Code plugin. Skills, agents, and docs are markdown. Gate enforcement hooks are bash scripts. There is no build system and no dependencies beyond `jq` (for hooks).

## Getting Started

1. Clone the repo
2. Test locally: `claude --plugin-dir .`
3. Run hook tests: `./hooks/test-hooks.sh`

## What We're Looking For

- **Bug reports** — especially around gate enforcement, quest lifecycle, and fellowship coordination
- **Skill improvements** — clearer prompts, better phase transitions, edge case handling
- **Hook hardening** — error handling, new test cases, edge case coverage
- **Documentation** — README clarity, config examples, usage guides

## How to Contribute

1. Open an issue first for non-trivial changes so we can discuss the approach
2. Fork the repo and create a branch from `main`
3. Make your changes
4. Run `./hooks/test-hooks.sh` and confirm all tests pass
5. Open a PR with a clear description of what and why

## Conventions

- **Commits**: use conventional commits (`feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `chore:`)
- **Skills**: SKILL.md files with YAML frontmatter (`name` and `description` fields)
- **Skill names**: must not collide with Claude Code built-in commands (`help`, `clear`, etc.)
- **Hooks**: bash scripts in `hooks/scripts/`, sourcing `_common.sh` for shared state file logic
- **Tests**: add test cases to `hooks/test-hooks.sh` for any hook behavior changes
- **Changelog**: don't edit — maintainer updates it at release time

## Repo Structure

```
.claude-plugin/plugin.json   # Plugin manifest
skills/<name>/SKILL.md       # Skills (markdown with YAML frontmatter)
agents/<name>.md             # Agent definitions
hooks/hooks.json             # Plugin hook definitions
hooks/scripts/*.sh           # Gate enforcement scripts (require jq)
hooks/test-hooks.sh          # Hook test suite
README.md                    # User-facing docs
CLAUDE.md                    # AI assistant conventions
```

## Testing

The hook test suite is the primary automated test:

```bash
./hooks/test-hooks.sh
```

For end-to-end testing, run a fellowship or quest locally with `claude --plugin-dir .` and verify gate behavior manually.

## Questions?

Open an issue or start a discussion on the repo.
