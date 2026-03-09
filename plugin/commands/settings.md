---
description: View or edit fellowship configuration (~/.claude/fellowship.json). Run /settings to see current settings, change values, or reset to defaults.
---

# Config — Fellowship Settings Manager

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

| Key | Type | Default | Valid values |
|-----|------|---------|--------------|
| `branch.pattern` | string \| null | `null` | Template with `{slug}`, `{ticket}`, `{author}` placeholders. Default effective pattern: `"fellowship/{slug}"`. |
| `branch.author` | string \| null | `null` | String with no spaces or git-invalid characters |
| `branch.ticketPattern` | string | `"[A-Z]+-\\d+"` | Any valid regex |
| `worktree.enabled` | boolean | `true` | `true`, `false` |
| `worktree.directory` | string \| null | `null` | Absolute path to a directory |
| `gates.autoApprove` | string[] | `[]` | `"Research"`, `"Plan"`, `"Implement"`, `"Review"`, `"Complete"`. Names refer to the completed phase — e.g., `"Research"` auto-approves the Research→Plan transition. |
| `pr.draft` | boolean | `false` | `true`, `false` |
| `pr.template` | string \| null | `null` | Template with `{task}`, `{summary}`, `{changes}` placeholders |
| `palantir.enabled` | boolean | `true` | `true`, `false` |
| `palantir.minQuests` | number | `2` | Any positive integer |
| `issues.autoClose` | boolean | `true` | `true`, `false`. When true, `/missive` includes `Closes #N` in PR keywords. |

## Merge Semantics

| Type | Behavior |
|------|----------|
| Scalars (strings, booleans, numbers) | Later in chain wins: user overrides project, project overrides defaults |
| Arrays (e.g., `gates.autoApprove`) | Replace, not union — later in chain wins entirely |
| Nested objects | Deep merge per the rules above |
