# Contributing to Fellowship

Thanks for your interest in contributing to Fellowship — a Claude Code plugin for orchestrating multi-task workflows.

## What This Repo Is

Fellowship is a Claude Code plugin with two parts:

- **Prompt layer** — skills, agents, and commands are pure markdown under `plugin/`. No runtime code.
- **Enforcement layer** — gate enforcement, worktree isolation, and quest state live in a Go CLI (`cli/`). Claude Code hooks shell out to the `fellowship` binary, which reads the hook payload as JSON on stdin and signals allow/block via its exit code.

The binary is built from `cli/` and distributed via GitHub releases (goreleaser on tag push). End users never build it — `plugin/hooks/scripts/ensure-binary.sh` downloads the matching release on first use (wired as a `SessionStart` hook), installing it at `~/.claude/fellowship/bin/fellowship`.

## Getting Started

1. Clone the repo
2. Test the plugin locally: `claude --plugin-dir .`
3. Build and test the CLI (from `cli/`):

   ```bash
   go build ./...
   go test ./...
   ```

You need a recent Go toolchain (see the `go` directive in `cli/go.mod`).

## How Hooks Work

Hooks are subcommands of the CLI, not standalone scripts. `plugin/hooks/hooks.json` maps Claude Code hook events to invocations of `plugin/hooks/scripts/fellowship.sh hook <name>`; each handler lives in `cli/internal/hooks/` and is dispatched from `runHook` in `cli/cmd/fellowship/main.go`.

- Input: the hook payload is JSON on stdin, parsed by `hooks.ParseInput`.
- Output: **exit 0 allows** the tool call; **exit 2 blocks** it (the message on stderr is surfaced to Claude). Some hooks emit a JSON decision on stdout instead (e.g. gate-submit).
- Posture: gate hooks (`gate-guard`, `gate-submit`, `gate-prereq`, `completion-guard`, `metadata-track`, `file-track`) fail *closed* so enforcement can't be silently skipped, but only where blocking is actually the safe answer. Precisely:
  - **No store** — the repo has no `.fellowship/fellowship.db`. This is an ordinary repo with no fellowship, so hooks **allow** (exit 0). The binary must not create a store just by being run: only `fellowship init` and `fellowship state init` may bring one into existence.
  - **Broken store** — the file exists but cannot be opened or read. Enforcement state is unknown, so gate hooks **block** (exit 2).
  - **Missing quest row** — the store is fine and this worktree is registered as a quest, but no `quest_state` row exists yet. This is the bootstrap window before the teammate runs `fellowship init`; blocking it would deadlock the quest before it starts, so hooks **allow** (exit 0) and log one line to stderr.
  - **Unregistered worktree** — the session is in a git worktree that is not the main repo root while a fellowship is initialized, and no quest matches it. Enforcement cannot be evaluated there, so gate hooks **block** (exit 2) instead of mistaking it for the lead session.
  - A pending gate is cleared only by the lead. `fellowship gate approve|reject` and `fellowship init` are *not* on the escape allowlist a blocked teammate may run; only read-only reporting (`status`, `gate status`, `tome`, `herald`, `eagles`) and side-channel bookkeeping (`autopsy`, `bulletin`, `errand`) are.
- The `worktree-guard` backstop is the exception to all of the above — it is defense-in-depth behind lead-provisioned isolation, so it fails *open* (allow) on any resolution failure, including a broken store, and blocks only on a positive main-tree mis-placement detection.
- The wrapper (`fellowship.sh`) applies that same posture to distribution itself: it never execs the `fellowship` binary directly from `hooks.json`. If the binary is missing or not executable, it runs `ensure-binary.sh` once to try to install it. If the binary is still unavailable afterwards, every gate hook (`gate-guard`, `gate-submit`, `gate-prereq`, `completion-guard`, `metadata-track`, `file-track`) blocks (exit 2, with a message on stderr) rather than silently letting the tool call through; `worktree-guard` alone exits 0 in that case, consistent with its fail-open backstop posture.

Current hooks: `gate-guard`, `gate-submit`, `gate-prereq`, `completion-guard`, `metadata-track`, `file-track`, `worktree-guard`. Run `fellowship` with no args for the full command reference.

## How to Contribute

1. Open an issue first for non-trivial changes so we can discuss the approach
2. Fork the repo and create a branch from `main`
3. Make your changes
4. For CLI changes, run the checks below and confirm they pass
5. Open a PR with a clear description of what and why

## Conventions

- **Commits**: use conventional commits (`feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `chore:`)
- **Go**: idiomatic Go, standard library where practical, clear error handling. Keep hook decision logic pure and table-test it (see `cli/internal/hooks/*_test.go`).
- **Skills**: `SKILL.md` files with YAML frontmatter (`name` and `description`). Commands use `description` only.
- **Skill names**: must not collide with Claude Code built-in commands (`help`, `clear`, `config`, etc.)
- **Changelog**: the README `## Changelog` is append-only per version and updated by the maintainer at release time. Don't edit historical entries.

## Repo Structure

```
.claude-plugin/plugin.json          # Plugin manifest (repo root; points to plugin/ paths)
plugin/skills/<name>/SKILL.md       # Skills — auto-invocable by Claude
plugin/commands/<name>.md           # Commands — user-invoked only
plugin/agents/<name>.md             # Agent definitions
plugin/hooks/hooks.json             # Maps hook events to `fellowship hook <name>`
plugin/hooks/scripts/ensure-binary.sh  # Downloads the CLI binary from GitHub releases
plugin/hooks/scripts/fellowship.sh  # Thin wrapper — ensures binary, then exec's it
cli/cmd/fellowship/main.go          # CLI entrypoint and subcommand dispatch
cli/internal/hooks/                 # Hook decision logic (pure, table-tested)
cli/internal/                       # State, db (SQLite), dashboard, herald, etc.
README.md                           # User-facing docs and changelog
CLAUDE.md                           # AI assistant conventions
```

## Schema changes

`cli/internal/db` tracks the SQLite schema version with `PRAGMA user_version` and upgrades a store through an ordered ladder of steps (`cli/internal/db/migrations.go`). To ship a schema change:

- Bump the version: add a new entry to the `migrations` slice, one version higher than the current last entry.
- Add a step, don't inline it elsewhere: the step's `up` func is the only place that DDL or data transformation runs for stores upgrading from an older version.
- Never edit an existing step. A store that already recorded that version in `user_version` will never run it again — an edit only reaches stores that haven't upgraded yet, and silently diverges from ones that already did. This applies to purely additive changes too (a new column, a new index): there's no safe edit to a step that may already be applied somewhere. Ship a corrective follow-up migration instead.
- Keep the fresh-install schema (`schema` in `cli/internal/db/schema.go`) in sync with the ladder, so a brand-new store and one upgraded through every migration always end up with identical schema objects (`TestFreshSchemaMatchesMigratedSchema` in `migrations_test.go` enforces this).

## Testing

For CLI changes, run from `cli/`:

```bash
gofmt -l .        # formatting — must report no files
go vet ./...      # static checks
go test ./...     # unit tests
```

For plugin (prompt-layer) changes, run a fellowship or quest locally with `claude --plugin-dir .` and verify behavior manually.

## Questions?

Open an issue or start a discussion on the repo.
