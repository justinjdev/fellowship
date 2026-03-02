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

## Hook Architecture

The gate enforcement system uses Claude Code's [plugin hooks](https://docs.anthropic.com/en/docs/claude-code/hooks) to enforce a state machine during quest execution. Hooks fire on specific tool calls and either allow (exit 0) or block (exit 2) the action. Outside quest sessions (no state file), all hooks are no-ops.

### State file

Quest state lives in `tmp/quest-state.json` (resolved from the git/worktree root). Key fields:

| Field | Type | Purpose |
|---|---|---|
| `phase` | string | Current quest phase: `Onboard`, `Research`, `Plan`, `Implement`, `Review`, `Complete` |
| `gate_pending` | bool | Whether a gate is awaiting lead approval |
| `gate_id` | string | Identifier for the pending gate |
| `lembas_completed` | bool | Whether `/lembas` has been run this phase |
| `metadata_updated` | bool | Whether task metadata has been updated this phase |
| `auto_approve_gates` | string[] | Phases that auto-advance without lead approval |

### Shared utilities: `_common.sh`

Sourced by every hook script. Resolves the state file path from `git rev-parse --show-toplevel`, checks that `jq` is installed, reads and validates the state JSON. If no state file exists (not a quest session), exits 0 immediately — this is what makes hooks transparent to non-quest usage.

### PreToolUse hooks

These fire *before* a tool executes and can block it.

**`gate-guard.sh`** — Matcher: `Edit|Write|Bash|Agent|Skill|NotebookEdit`

Two responsibilities:
1. **Gate block:** When `gate_pending=true`, blocks all matched tools. The teammate must wait for lead approval.
2. **Phase-aware file guard:** During `Onboard`, `Research`, and `Plan` phases, blocks file writes (Edit, Write, NotebookEdit) to anything outside `tmp/`. This prevents production code changes before the Implement phase. Bash, Skill, and Agent calls pass through since they're needed for research and planning.

**`gate-submit.sh`** — Matcher: `SendMessage`

Processes gate submissions when a message starts with `[GATE]`:
1. Rejects multiple `[GATE]` markers in one message
2. Blocks if a gate is already pending
3. Checks prerequisites: both `lembas_completed` and `metadata_updated` must be true
4. If the next phase is in `auto_approve_gates`, advances the phase and resets prerequisites without setting `gate_pending`
5. Otherwise, sets `gate_pending=true` and generates a `gate_id`

Non-gate messages (no `[GATE]` prefix) pass through untouched.

**`completion-guard.sh`** — Matcher: `TaskUpdate`

Blocks `status: "completed"` updates unless the current phase is `Complete`. This prevents teammates from marking a quest done without progressing through all gates. Non-completion updates (status changes, metadata) pass through.

### PostToolUse hooks

These fire *after* a tool executes and track prerequisite completion.

**`gate-prereq.sh`** — Matcher: `Skill`

Sets `lembas_completed=true` when the invoked skill name contains "lembas" (matches any plugin namespace like `fellowship:lembas`). Other skill invocations are ignored.

**`metadata-track.sh`** — Matcher: `TaskUpdate`

Sets `metadata_updated=true` when the TaskUpdate includes `metadata.phase`. Other metadata or status-only updates are ignored.

### Data flow

```
Quest teammate takes action
        │
        ▼
  ┌─ PreToolUse ──────────────────────────────┐
  │                                            │
  │  Edit/Write/Bash/...  → gate-guard.sh      │
  │  SendMessage          → gate-submit.sh     │
  │  TaskUpdate           → completion-guard.sh │
  │                                            │
  │  exit 0 = allow    exit 2 = block          │
  └────────────────────────────────────────────┘
        │ (allowed)
        ▼
    Tool executes
        │
        ▼
  ┌─ PostToolUse ─────────────────────────────┐
  │                                            │
  │  Skill      → gate-prereq.sh              │
  │  TaskUpdate → metadata-track.sh            │
  │                                            │
  │  Updates tmp/quest-state.json              │
  └────────────────────────────────────────────┘
```

### Design principles

- **Fail closed.** Malformed input or state file errors produce exit 2 (block), not exit 0 (allow).
- **No-op without state.** If `tmp/quest-state.json` doesn't exist, all hooks exit 0. Non-quest sessions are never affected.
- **Atomic writes.** State updates write to `$STATE_FILE.tmp` then `mv` to avoid partial writes.
- **Single dependency.** All scripts require `jq` — no other external tools.

### Adding a new hook

1. Create the script in `hooks/scripts/`, source `_common.sh` at the top
2. Add the hook entry in `hooks/hooks.json` under the appropriate event (`PreToolUse` or `PostToolUse`) with a tool matcher
3. Add test cases to `hooks/test-hooks.sh`
4. Run `./hooks/test-hooks.sh` to verify

## Testing

The hook test suite is the primary automated test:

```bash
./hooks/test-hooks.sh
```

For end-to-end testing, run a fellowship or quest locally with `claude --plugin-dir .` and verify gate behavior manually.

## Questions?

Open an issue or start a discussion on the repo.
