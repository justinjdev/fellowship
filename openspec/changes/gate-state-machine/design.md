## Context

Fellowship is a pure-markdown Claude Code plugin that orchestrates parallel quest teammates. Each teammate runs `/quest` (a 6-phase lifecycle: Onboard → Research → Plan → Implement → Review → Complete) in an isolated git worktree. The lead (Gandalf) coordinates via `SendMessage` and `TaskUpdate`.

Gate enforcement is currently prompt-only: the spawn prompt tells teammates to "STOP" after sending gate messages. Observed failure rate is ~67%. Teammates continue working after gates, self-approve, combine gates, and skip lembas/metadata prerequisites.

Claude Code plugins can ship hooks via `hooks/hooks.json` that execute shell commands on tool events. Hooks can block tool calls (exit code 2), inject context, and read/write files. This is the structural enforcement mechanism available to us.

## Goals / Non-Goals

**Goals:**
- Structurally prevent teammates from working after submitting a gate (PreToolUse blocking)
- Structurally prevent gate submission without lembas + metadata prerequisites
- Make self-approval irrelevant — only Gandalf can clear `gate_pending` by writing to the state file
- Keep the state machine simple: one JSON file per worktree, read/written by bash scripts with `jq`
- Maintain backward compatibility for non-fellowship usage (hooks are no-ops when no state file exists)

**Non-Goals:**
- Enforcing gate message *content* quality (still prompt-level)
- Preventing agents from *saying* "approved" in text output (irrelevant if they can't work)
- Building a general-purpose agent state machine framework
- Supporting concurrent state file writers (one teammate per worktree)
- Removing prompt-based gate instructions (hooks supplement, not replace)

## Decisions

### 1. State file location: `tmp/quest-state.json`

The quest skill already uses `tmp/` for `checkpoint.md` (lembas). Placing the state file here keeps ephemeral quest artifacts together and ensures cleanup with the worktree.

**Alternative considered:** `.claude/quest-state.json` — rejected because `.claude/` is for configuration, not runtime state.

### 2. Teammate detection: state file existence

Hooks check for `tmp/quest-state.json` at the start of every invocation. If absent, exit 0 immediately (no-op). This means:
- Normal user sessions: unaffected
- Gandalf's session: unaffected (no state file in main worktree)
- Quest teammates: enforced (state file created at Phase 0)

**Alternative considered:** Marker file or environment variable — rejected as unnecessary indirection when the state file itself is the marker.

### 3. Gate approval: Gandalf writes to teammate's state file

When Gandalf approves a gate, the fellowship skill instructs Gandalf to write to the teammate's `tmp/quest-state.json` using a bash command. Worktrees share the filesystem, so Gandalf can reach `<worktree-path>/tmp/quest-state.json` directly.

The teammate's worktree path is stored in task metadata (`metadata.worktree_path`) set during Phase 0. Gandalf reads this from `TaskGet` when routing approvals.

**Flow:**
1. Teammate submits gate → hook sets `gate_pending=true` in state file
2. Teammate's subsequent tool calls are blocked by PreToolUse hook
3. Gandalf receives gate message, routes approval
4. Gandalf runs: `jq '.gate_pending=false | .phase="<next>" | .lembas_completed=false | .metadata_updated=false' <path>/tmp/quest-state.json > tmp && mv tmp <path>/tmp/quest-state.json`
5. Teammate's next tool call succeeds (hook sees `gate_pending=false`)

**Alternative considered:** Teammate reads approval from a SendMessage and updates its own state — rejected because this reintroduces the self-approval problem. The whole point is that only an external actor can advance the state.

### 4. Hook strategy: minimal hooks, maximum coverage

Four hooks total:

| Hook | Event | Matcher | Purpose |
|------|-------|---------|---------|
| `gate-guard` | PreToolUse | `Edit\|Write\|Bash\|Agent\|Skill\|NotebookEdit` | Block work when `gate_pending=true` |
| `gate-prereq` | PostToolUse | `Skill` | Track `/lembas` completion → set `lembas_completed=true` |
| `metadata-track` | PostToolUse | `TaskUpdate` | Track phase metadata update → set `metadata_updated=true` |
| `gate-submit` | PreToolUse | `SendMessage` | When gate message detected and `gate_pending=false`, verify prerequisites met, then set `gate_pending=true` |

**Alternative considered:** A `Stop` hook to validate turn-end state — deferred. The PreToolUse blocking is the primary enforcement; a Stop hook would be defense-in-depth but adds complexity. Can add later if needed.

**Alternative considered:** Separate hooks for each phase transition — rejected. A single state file with boolean flags is simpler and handles all phases uniformly.

### 5. Gate message detection: content pattern matching

The `gate-submit` hook on SendMessage needs to distinguish gate messages from normal teammate messages (e.g., "I'm stuck" or "question about the API"). Strategy: match on the gate checklist format that the quest skill produces.

Gate messages contain structured checklists with `- [x]` items and phase names. The hook checks for the presence of a phase keyword (`Research|Plan|Implement|Review|Complete`) combined with checklist markers. This is a heuristic — false positives are low-risk (the hook just sets `gate_pending`, which Gandalf can clear), and false negatives mean the gate isn't enforced for that message (fallback to prompt behavior).

**Alternative considered:** Require a structured `GATE:` prefix in gate messages — rejected. This requires modifying the quest skill's gate output format AND relying on the agent to use it (another prompt-level instruction). Pattern matching on existing output is more robust.

### 6. `jq` as the only dependency

All scripts use `jq` for JSON manipulation. `jq` is pre-installed on macOS (via Xcode CLT) and available on virtually all Linux distributions. No other dependencies.

**Alternative considered:** Pure bash JSON parsing with `sed`/`awk` — rejected. Brittle, hard to maintain, and `jq` is a reasonable expectation.

### 7. Auto-approved gates skip the state machine block

When `config.gates.autoApprove` includes a gate name, the teammate's spawn prompt already tells it to proceed without waiting. The state machine should not contradict this. For auto-approved gates:
- The `gate-submit` hook still validates prerequisites (lembas + metadata)
- But it does NOT set `gate_pending=true`
- Instead it sets `phase` to the next phase and resets prerequisites

This requires the state file to include the auto-approve config, written at init time.

## Risks / Trade-offs

**[`jq` not installed] → Mitigation:** Scripts check for `jq` at the top and exit 0 (no-op, non-blocking) with a stderr warning if missing. Hooks degrade gracefully to prompt-only enforcement rather than breaking the session.

**[Stale state file after crash] → Mitigation:** If a teammate crashes with `gate_pending=true`, the state file persists in the worktree. On respawn, the quest skill detects an existing state file and either resets it or resumes from the recorded phase. The respawn procedure in fellowship SKILL.md already points at the existing worktree.

**[Gate message false negatives] → Mitigation:** If the pattern match misses a gate message, enforcement falls back to prompt behavior (same as today). This is strictly better than current state, not worse.

**[Cross-worktree path changes] → Mitigation:** Worktree paths are stable for the lifetime of the worktree. The path is written to task metadata once at Phase 0 and doesn't change.

**[Hook latency] → Trade-off:** Every tool call reads `tmp/quest-state.json`. This is a local file read of ~200 bytes — negligible latency. `jq` invocation adds ~10-20ms per hook. Acceptable.

**[Complexity budget] → Trade-off:** The plugin goes from pure markdown to markdown + ~150 lines of bash. This is a meaningful shift. The scripts are simple (read JSON, check field, exit 0 or 2) but they need to be maintained and tested. The compliance improvement from ~33% to ~95% justifies the cost.
