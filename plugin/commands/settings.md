---
description: View or edit fellowship configuration (~/.claude/fellowship.json). Run /settings to see current settings, change values, or reset to defaults.
---

# Settings — Fellowship Settings Manager

## Steps

### Step 1: Read Config Layers

Read both config files (neither is required to exist):

1. **Project config:** `.fellowship/config.json` in the git repository root (find with `git rev-parse --show-toplevel`)
2. **User config:** `~/.claude/fellowship.json`

Note which keys are present in each file. The effective value for each key follows this precedence:
**defaults → project → user** (user always wins).

### Step 2: Show Settings

Present the merged config as a table with a **Source** column showing `[default]`, `[project]`, or `[user]` for each key:

```
Fellowship Config

  Setting                Value                    Source
  ─────────────────────────────────────────────────────────
  dataDir                .fellowship              [default]
  branch.pattern         null                     [default]
  branch.author          null                     [default]
  branch.ticketPattern   [A-Z]+-\d+               [default]
  worktree.enabled       true                     [default]
  worktree.directory     null                     [default]
  gates.autoApprove      []                       [default]
  pr.draft               false                    [default]
  pr.template            null                     [default]
  palantir.enabled       true                     [default]
  palantir.minQuests     2                        [default]
  issues.autoClose       true                     [default]
  failures.expiryDays    90                       [default]
  models.quest           null (inherit)           [default]
  models.scout           null (sonnet)            [default]
  models.palantir        null (haiku)             [default]
  models.balrog          null (inherit)           [default]
  models.explore         null (haiku)             [default]
  models.validator       null (sonnet)            [default]

  User config:    ~/.claude/fellowship.json
  Project config: .fellowship/config.json (none found)
```

Show both file paths at the bottom, noting "(none found)" if a file doesn't exist.

### Step 3: Ask What to Change

Ask the user what they'd like to change. Use `AskUserQuestion` with these options:

1. **Change user settings** — modify values in `~/.claude/fellowship.json`
2. **Change project settings** — modify values in `.fellowship/config.json` (committable)
3. **Reset user config to defaults** — delete `~/.claude/fellowship.json`
4. **Done** — exit without changes

If the user picks "Change user settings" or "Change project settings", ask which settings to modify. Present each setting with its current effective value and valid options. Use the schema below for validation.

### Step 4: Write Config

For user settings: write only values that differ from the effective merged value of `defaults → project`. This means you may need to write an explicit default value when the user wants to override a non-default project config value back to its default. If no keys remain after this minimization, delete `~/.claude/fellowship.json` instead of writing an empty object.

For project settings: write only non-default values to `.fellowship/config.json`. Create `.fellowship/` directory if needed. If all values match defaults, delete the file instead of writing an empty object.

Validate each value against the Schema Reference below before writing.

### Step 5: Confirm

Read back both files and show the updated settings table from Step 2.

## Schema Reference

This is the canonical schema for fellowship config files. Both `~/.claude/fellowship.json` and `.fellowship/config.json` support the same keys.

**Enforced by** marks how a setting takes effect: "Binary" settings are read and applied by the `fellowship` Go CLI itself; "Prompt" settings only take effect because the agent reads the merged config and follows it — no CLI code enforces them structurally.

| Key | Type | Default | Enforced by | Valid values |
|-----|------|---------|--------------|--------------|
| `dataDir` | string | `".fellowship"` | Binary | Directory name for fellowship working files (state, checkpoints, todos, history). Created inside each worktree and the main repo root. |
| `branch.pattern` | string \| null | `null` | Prompt | Template with `{slug}`, `{ticket}`, `{author}` placeholders. Default effective pattern: `"fellowship/{slug}"`. |
| `branch.author` | string \| null | `null` | Prompt | String with no spaces or git-invalid characters |
| `branch.ticketPattern` | string | `"[A-Z]+-\\d+"` | Prompt | Any valid regex |
| `worktree.enabled` | boolean | `true` | Prompt | `true`, `false` |
| `worktree.directory` | string \| null | `null` | Prompt | Absolute path to a directory |
| `gates.autoApprove` | string[] | `[]` | Binary | `"Research"`, `"Plan"`, `"Implement"`. Names refer to the phase being left — e.g., `"Research"` auto-approves the Research→Plan transition. `"Review"` is not a valid value: it is the last phase and no gate leaves it. |
| `pr.draft` | boolean | `false` | Prompt | `true`, `false` |
| `pr.template` | string \| null | `null` | Prompt | Template with `{task}`, `{summary}`, `{changes}` placeholders |
| `palantir.enabled` | boolean | `true` | Prompt | `true`, `false` |
| `palantir.minQuests` | number | `2` | Prompt | Any positive integer |
| `issues.autoClose` | boolean | `true` | Prompt | `true`, `false`. When true, `/missive` includes `Closes #N` in PR keywords. |
| `failures.expiryDays` | number | `90` | Binary | Any positive integer. Days before a quest failure record expires and is eligible for cleanup. |
| `models.quest` | string \| null | `null` | Prompt | Model for quest teammates. Valid values: the aliases `"haiku"`, `"sonnet"`, `"opus"` — these are passed verbatim as the Agent tool's `model` parameter, which does not accept `"inherit"` or full model IDs. To inherit the session model, leave the key `null` (the parameter is then omitted). `null` = built-in default: inherit the session model. |
| `models.scout` | string \| null | `null` | Prompt | Model for scout teammates. Same valid values. `null` = built-in default: `sonnet` (from the scout agent definition). |
| `models.palantir` | string \| null | `null` | Prompt | Model for the palantir monitor. Same valid values. `null` = built-in default: `haiku` (from the palantir agent definition). |
| `models.balrog` | string \| null | `null` | Prompt | Model for balrog adversarial review. Same valid values. `null` = built-in default: inherit the session model. |
| `models.explore` | string \| null | `null` | Prompt | Model for Explore scan subagents spawned by quest, scout, council, and guide. Same valid values. `null` = built-in default: `haiku`. |
| `models.validator` | string \| null | `null` | Prompt | Model for scout's validation subagent. Same valid values. `null` = built-in default: `sonnet` (from the validator agent definition). |

## Merge Semantics

| Type | Behavior |
|------|----------|
| Scalars (strings, booleans, numbers) | Later in chain wins: user overrides project, project overrides defaults |
| Arrays (e.g., `gates.autoApprove`) | Replace, not union — later in chain wins entirely |
| Nested objects | Deep merge per the rules above |
