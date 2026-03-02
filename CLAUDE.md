# CLAUDE.md

Claude Code plugin — no build system. Skills, agents, and docs are pure markdown. Gate enforcement hooks are bash scripts (require `jq`).

## Structure

```
.claude-plugin/plugin.json   # Plugin manifest (name, version, repo URL)
skills/<name>/SKILL.md       # Each skill is a single SKILL.md with YAML frontmatter
agents/<name>.md             # Agent definitions
hooks/hooks.json             # Plugin hook definitions (gate enforcement)
hooks/scripts/*.sh           # Bash scripts for gate state machine (require jq)
hooks/test-hooks.sh          # Hook test suite
README.md                    # User-facing docs, install instructions, changelog
```

## Conventions

- **Skill names** must not collide with Claude Code built-in commands (e.g., don't name a skill `config`, `help`, `clear`).
- **YAML frontmatter** in SKILL.md files has two fields: `name` (matches directory name) and `description`.
- **Changelog** in README.md is append-only per version. Don't edit historical entries — they describe what shipped at that version.

## Releasing

1. Bump `version` in `.claude-plugin/plugin.json`
2. Add a changelog section in README.md under `## Changelog`
3. Commit, push to `main`
4. Tag with `git tag v<version>` and push the tag
5. Update `version` in the marketplace repo (`justinjdev/claude-plugins` → `.claude-plugin/marketplace.json`)
