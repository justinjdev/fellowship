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
  - **No store, no data directory** — the repo has neither `<data-dir>/fellowship.db` nor the data directory itself. This is an ordinary repo with no fellowship, so hooks **allow** (exit 0). The binary must not create a store just by being run: only `fellowship init` and `fellowship state init` may bring one into existence.
  - **No store where one is expected** — the main worktree has a fellowship data directory but the store is missing, or is zero bytes. A fellowship was expected here and its state is gone (deleting the store was the cheapest way to switch enforcement off), so gate hooks **block** (exit 2); `worktree-guard` still fails open. A zero-byte store is a destroyed store, never a fresh one, and is never rebuilt from inside a hook. "Expected" excludes a data directory holding nothing but `config.json`: that file is the committable project config, so a fresh clone of any repo that uses one has the directory without ever having run a fellowship, and blocking there would refuse every tool call in it. An *empty* data directory still counts — git never checks one out, so only the CLI or a person made it.
  - **Broken store** — the file exists but cannot be opened or read. Enforcement state is unknown, so gate hooks **block** (exit 2).
  - **Out-of-date store** — the store was written by an older binary. Hooks never migrate: running the schema ladder from a hook would have every tool call racing to rewrite the store it is reading. Gate hooks **block** (exit 2, "run `fellowship init` to upgrade"); `init` and `state init` migrate. The two hooks that only decide (`gate-guard`, `worktree-guard`) open the store read-only.
  - **Missing quest row, no history** — the store is fine and this worktree is registered as a quest, but no `quest_state` row exists and the quest has no gates in its history and no events. This is the bootstrap window before the teammate runs `fellowship init`; blocking it would deadlock the quest before it starts, so hooks **allow** (exit 0) and log one line to stderr.
  - **Missing quest row, with history** — the same shape, but the quest has already submitted a gate or logged an event. It is past its bootstrap, so the row was deleted, not never-created: gate hooks **block** (exit 2, pointing at `fellowship init --quest <name>` from the lead).
  - **Unregistered worktree** — the session is in a git worktree that is not the main repo root while a fellowship is *running* (a non-completed quest whose worktree is still on disk — the same predicate `worktree-guard` arms itself with), and no quest matches it. Enforcement cannot be evaluated there, so gate hooks **block** (exit 2) instead of mistaking it for the lead session.
  - A pending gate is cleared only by the lead. `fellowship gate approve|reject` and `fellowship init` are *not* on the escape allowlist a blocked teammate may run; only read-only reporting (`status`, `gate status`, `history`, `events`, `health`) and side-channel bookkeeping (`failures`, `notes`, `todo`) are. A **held** quest reads the same allowlist, so a paused teammate can still see why it stopped and leave a note about it.
  - **Only the lead moves a phase.** `fellowship init --phase X` (and `--plan-skip`) on a quest row that already exists is a phase move, and a phase move is a gate decision — it can take a quest from Research to Implement with no gate ever submitted, and gate-guard waves it through because nothing is pending. On an existing row `init` may only reset the gate and prerequisite flags; `--phase`/`--plan-skip` are honored when the row is created for the first time, or when the caller is the recorded lead. `gate-guard` refuses the Bash form for a non-lead session.
  - **The store is not a coordination file.** The data directory is exempt from the early-phase write rule, but `fellowship.db` (and `-wal`/`-shm`/`-journal`) is not: `Edit`/`Write`/`NotebookEdit` aimed at it are refused in every phase, for every session, including in the main tree.
  - **Paths key off the main repo root.** Every hook resolves the data directory name from `gitutil.MainRepoRoot(cwd)` — exactly as `db.StorePath` does — because the project config lives in the main worktree. Resolving it from the session's own top-level made a teammate in a linked worktree enforce `.fellowship` for a fellowship configured otherwise.
- The `worktree-guard` backstop is the exception to all of the above — it is defense-in-depth behind lead-provisioned isolation, so it fails *open* (allow) on any resolution failure, including a broken store, and blocks only on a positive main-tree mis-placement detection.
- **Who may write in the main tree.** The main working tree is the lead's own workspace, so "a source write in the main tree during an active fellowship" is not by itself a mis-placement — `worktree-guard` has to tell the lead apart from a teammate that was dropped into the main tree. Nothing in the git topology does that (both resolve to the same top-level), so the guard identifies the *session*:
  - `fellowship state init` records the lead in the **store** — the `lead` table (`session_id`, `root`, `created_at`), added by schema migration 4. The session id comes from `CLAUDE_CODE_SESSION_ID`, which Claude Code exports to the commands it runs and which is the same id it puts in hook payloads as `session_id`. It lives in SQLite rather than in a file under the data directory because that directory is exempt from the write guards: a marker there was writable by the very sessions it identified, so a teammate could name itself the lead. The legacy `<data-dir>/lead` file is still *read*, for one release, but only when the store names no lead at all — a store that names one is never overruled by a file.
  - Which write is judged: the **target file's** own working-tree root, resolved from its path, not the session's working directory. A teammate that stays in its own worktree and names an absolute path in the main tree is making the same mis-placed write as one dropped into the main tree.
  - At hook time the guard compares the payload's `session_id` with the recorded lead, in this order: **(1)** payload id == recorded id → the lead, allow; **(2)** the session's git top-level is a registered quest worktree → a teammate writing into the main tree, block (this one needs no session ids); **(3)** both ids are known and differ → a session that is not the lead, block; **(4)** otherwise the writer cannot be identified — no recorded lead, or no id in the payload — and the fail-open backstop allows.
  - The `.git`, `.claude` and data-directory exemption applies only to a session standing in the main tree; a teammate reaching into the main tree's data directory from its worktree is blocked like any other main-tree write.
  - Consequence to know: if the lead's session id changes mid-fellowship (a brand-new session in the main tree rather than a resumed one), rule 3 blocks it. `fellowship state init --claim-lead` re-records this session as the lead and changes nothing else; any `fellowship` command run in the main tree from a non-lead session prints one line saying so. `state init` with no `CLAUDE_CODE_SESSION_ID` in the environment records an anonymous lead and warns, since the guard then falls back to rule 2.
- The wrapper (`fellowship.sh`) applies that same posture to distribution itself: it never execs the `fellowship` binary directly from `hooks.json`. If the binary is missing or not executable, it runs `ensure-binary.sh` once to try to install it. If the binary is still unavailable afterwards, every gate hook (`gate-guard`, `gate-submit`, `gate-prereq`, `completion-guard`, `metadata-track`, `file-track`) blocks (exit 2, with a message on stderr) rather than silently letting the tool call through; `worktree-guard` alone exits 0 in that case, consistent with its fail-open backstop posture.

Current hooks: `gate-guard`, `gate-submit`, `gate-prereq`, `completion-guard`, `metadata-track`, `file-track`, `worktree-guard`. Run `fellowship` with no args for the full command reference.

What each of the gate hooks decides, against the four-phase lifecycle (Research → Plan → Implement → Review):

| Hook | Decides |
|---|---|
| `gate-guard` | Blocks a held quest and blocks everything but a read-only escape command while a gate is pending (both read the same allowlist), refuses a non-lead `fellowship init --phase/--plan-skip`, refuses any hand-edit of the store, and blocks source writes outside the data directory during Research and Plan (`state.IsEarlyPhase`) |
| `gate-submit` | Detects a `[GATE]` marker, checks the lembas and metadata prerequisites, and runs the phase transition — including the auto-approve path for a phase named in `gates.autoApprove` |
| `gate-prereq` | Records that `/lembas` ran for this phase |
| `metadata-track` | Records that the task's `phase` metadata was updated — only when it names a valid phase that is the quest's current one |
| `completion-guard` | Allows `TaskUpdate(status: "completed")` only in Review with no gate pending — Review is terminal, so this is the one place a quest may end |
| `file-track` | Records file touches in the quest history |

The lifecycle itself has exactly one definition: `phaseOrder` in `cli/internal/state/state.go`. `state.Phases()`, `state.GatePhases()` (the three phases a gate leaves, and the only valid `gates.autoApprove` entries) and `state.TerminalPhase` all derive from it — never write a phase name into a comparison when one of those will do.

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
cli/internal/                       # State, db (SQLite), dashboard, events, etc.
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
