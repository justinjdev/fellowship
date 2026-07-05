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

Hooks are subcommands of the CLI, not standalone scripts. `plugin/hooks/hooks.json` maps Claude Code hook events to `fellowship hook <name>` invocations; each handler lives in `cli/internal/hooks/` and is dispatched from `runHook` in `cli/cmd/fellowship/main.go`.

- Input: the hook payload is JSON on stdin, parsed by `hooks.ParseInput`.
- Output: **exit 0 allows** the tool call; **exit 2 blocks** it (the message on stderr is surfaced to Claude). Some hooks emit a JSON decision on stdout instead (e.g. gate-submit).
- Posture: gate hooks fail *closed* (block on internal error) so enforcement can't be silently skipped. The `worktree-guard` backstop is the exception — it is defense-in-depth behind lead-provisioned isolation, so it fails *open* (allow) on any resolution failure and blocks only on a positive main-tree mis-placement detection.

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
