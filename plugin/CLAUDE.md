# CLAUDE.md

Claude Code plugin. Skills, agents, and docs are pure markdown. Gate enforcement is a Go CLI binary distributed via GitHub releases (downloaded automatically on first use).

## Structure

```
.claude-plugin/plugin.json        # Plugin manifest at repo root (points to plugin/ paths)
plugin/skills/<name>/SKILL.md     # Skills — auto-invocable by Claude (quest, scout, council, etc.)
plugin/commands/<name>.md          # Commands — user-invoked only, no base context cost
plugin/agents/<name>.md           # Agent definitions
plugin/hooks/hooks.json           # Plugin hook definitions (gate enforcement)
plugin/hooks/scripts/fellowship.sh  # Thin wrapper — ensures binary exists, then exec's it
plugin/hooks/scripts/ensure-binary.sh # Downloads CLI binary from GitHub releases
```

## Conventions

- **Skill names** must not collide with Claude Code built-in commands (e.g., don't name a skill `config`, `help`, `clear`).
- **YAML frontmatter** in SKILL.md files has two fields: `name` (matches directory name) and `description`. Command files use `description` only (no `name` field).
- **Skills vs commands:** Skills are for things Claude needs to know about and invoke automatically (quest phases, context compression). Commands are for user-invoked actions that don't need to consume base context (guide, settings, scribe). If only the user types it, make it a command.
- **Changelog** in README.md is append-only per version. Don't edit historical entries — they describe what shipped at that version.

## Paired content (keep in sync)

Cross-file duplication that exists on purpose — agent files must be self-contained at runtime (they're injected as system prompts with no path context), and docs mirror the canonical schema. When editing one side, update the other:

- `plugin/agents/_protocol.md` (canonical) ↔ protocol blocks embedded in `plugin/agents/balrog.md`, `palantir.md`, `scout.md`
- `plugin/skills/gather-lore/SKILL.md` Step 2 pattern taxonomy ↔ `plugin/skills/warden/SKILL.md` Step 3 dimensions (and gather-lore Step 4 ↔ warden Step 6) ↔ the condensed taxonomy in `plugin/skills/quest/SKILL.md` Research step 4
- `plugin/skills/council/SKILL.md` Steps 1–5 ↔ `plugin/skills/quest/SKILL.md` Research step 2 (quest inlines the orientation rather than invoking council, so a quest carries no second skill's phase vocabulary)
- The quest lifecycle — Research → Plan → Implement → Review, three gates — is named in `plugin/skills/quest/SKILL.md`, `plugin/skills/fellowship/SKILL.md` and its `resources/`, `plugin/agents/palantir.md`, `plugin/commands/{settings,guide,scribe}.md`, README, and `site/src/routes/`. The enforcing list is `phaseOrder` in `cli/internal/state/state.go`
- Config schema: `plugin/commands/settings.md` (canonical) ↔ README Configuration section ↔ `site/src/routes/configuration/+page.svelte`

Do not encode these as HTML comments inside skill/agent prompt files — hidden directives in agent-facing content can be followed by agents as instructions.

## Releasing

1. Bump `version` in `.claude-plugin/plugin.json`
2. Add a changelog section in README.md under `## Changelog`
3. Commit, push to `main`
4. Tag with `git tag v<version>` and push the tag
5. Update `version` in the marketplace repo (`justinjdev/claude-plugins` → `.claude-plugin/marketplace.json`)
