# Isolation (the lead's job)

Every quest gets its own git worktree, and provisioning one is the **lead's**
job — never a flag to trust, never something a teammate is left to do. This is
the full protocol behind the summary in [SKILL.md](../SKILL.md).

## Why the harness flag is not enough

The Agent tool's `isolation: "worktree"` parameter has been observed to
silently no-op for background quest teammates: no worktree is created, the
teammate lands in the main repo root, and nothing errors. A teammate cannot be
relied on to provision its own either — that instruction is advisory and fails
just as quietly when skipped, dropping the quest into the shared main tree.

## Gate hook propagation

Plugin hooks fire in the session that loaded the plugin, and that covers the
tool calls of the background agents it spawns. A worktree can still be opened
by a session of its own — a rekindled quest, a terminal — and that session
only enforces `worktree-guard` if a `.claude/settings.local.json` at its root
registers it.

`fellowship state init` writes that file: it merges the `worktree-guard`
PreToolUse hook into the project's `.claude/settings.local.json`, preserving
any existing settings, idempotently. That file is **git-ignored**, so this
touches no history and leaves no untracked file. `--skip-hook-install` opts out
if you manage settings yourself.

That alone catches the primary failure: a teammate that lands in the main repo
root reads the main tree's `settings.local.json`, so the guard fires and
blocks. Arming a *correctly placed* teammate takes one more step — the lead
copies `.claude/settings.local.json` into each new worktree right after
`git worktree add`. No commit, ever.

## Pre-flight, before spawning any quest

Gandalf confirms all three:

1. `state init` registered the hook — its output reads "Registered
   worktree-guard hook in .claude/settings.local.json", or the file already
   had it.
2. The binary is present — `~/.claude/fellowship/bin/fellowship version`
   succeeds.
3. A fellowship has been initialized, so the state store exists.

The guard is **inert unless a fellowship is active**, so installing it is
always safe; it never blocks work outside a fellowship.

## Provisioning, for each quest

1. **Create the worktree yourself.** Run
   `git worktree add -b <branch> <path> <base>` with `<path>` a distinct git
   worktree root (any path; the default is `.claude/worktrees` under the
   repo). Passing the harness `isolation` flag as well MAY work, but it must
   be verified, never assumed.
2. **Make it usable.** Provision dependencies so the teammate's tests run (for
   example symlink or install `node_modules`), and copy the settings file in
   so the guard is armed there:
   `mkdir -p <path>/.claude && cp .claude/settings.local.json <path>/.claude/`.
3. **Verify before the teammate writes.** `git worktree list` must show the
   worktree, and its path must not be the main root. Never tell a teammate it
   is "already isolated" without having checked.
4. **Publish the verified path before spawning.** Register it in the store —
   `fellowship state update-quest --dir <repo_root> --name <quest_name>
   --worktree <path>` (or `add-quest --worktree` when registering a new
   quest) — so `fellowship init --dir <path>` resolves the quest and the
   teammate's own check (`state show --json`) finds it and does not create a
   second worktree on the wrong branch.

## The two backstops

None of the above is what finally prevents a mis-placed write. These are, and
they hold regardless of how isolation was provisioned:

1. The teammate's mandatory isolation self-check before its first write — its
   git top-level must differ from the main repo root, else it stops and
   messages the lead (see [spawn-prompts.md](spawn-prompts.md)).
2. The fail-closed `worktree-guard` PreToolUse hook, which blocks source
   writes from the main tree during an active fellowship. A block from it is
   **proof** the teammate is mis-placed, not an obstacle to route around.
