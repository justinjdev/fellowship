# CLAUDE.md

Claude Code plugin. Skills, agents, and docs are pure markdown. Gate enforcement is a Go CLI binary distributed via GitHub releases (downloaded automatically on first use).

## Structure

```
.claude-plugin/plugin.json   # Plugin manifest (name, version, repo URL)
skills/<name>/SKILL.md       # Skills — auto-invocable by Claude (quest, scout, council, etc.)
commands/<name>.md            # Commands — user-invoked only, no base context cost
agents/<name>.md             # Agent definitions
hooks/hooks.json             # Plugin hook definitions (gate enforcement)
hooks/scripts/fellowship.sh  # Thin wrapper — ensures binary exists, then exec's it
hooks/scripts/ensure-binary.sh # Downloads CLI binary from GitHub releases
```

## Conventions

- **Skill names** must not collide with Claude Code built-in commands (e.g., don't name a skill `config`, `help`, `clear`).
- **YAML frontmatter** in SKILL.md files has two fields: `name` (matches directory name) and `description`. Command files use `description` only (no `name` field).
- **Skills vs commands:** Skills are for things Claude needs to know about and invoke automatically (quest phases, context compression). Commands are for user-invoked actions that don't need to consume base context (guide, settings, scribe). If only the user types it, make it a command.
- **Changelog** in README.md is append-only per version. Don't edit historical entries — they describe what shipped at that version.

## Releasing

1. Bump `version` in `.claude-plugin/plugin.json`
2. Add a changelog section in README.md under `## Changelog`
3. Commit, push to `main`
4. Tag with `git tag v<version>` and push the tag
5. Update `version` in the marketplace repo (`justinjdev/claude-plugins` → `.claude-plugin/marketplace.json`)
