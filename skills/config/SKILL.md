---
name: config
description: View or edit fellowship configuration (~/.claude/fellowship.json). Run /config to see current settings, change values, or reset to defaults.
---

# Config — Fellowship Settings Manager

## Steps

### Step 1: Read Current Config

Read `~/.claude/fellowship.json`. If it does not exist, report "No config file found — all defaults active."

If it exists, show the current settings as a table comparing each key to its default, highlighting only non-default values.

### Step 2: Show Settings

Present the user's current config (or defaults if no file) in this format:

```
Fellowship Config (~/.claude/fellowship.json)

  branch.pattern      null               (default)
  branch.author       null               (default)
  branch.ticketPattern [A-Z]+-\d+        (default)
  worktree.enabled    true               (default)
  worktree.directory  null               (default)
  gates.autoApprove   []                 (default)
  pr.draft            false              (default)
  pr.template         null               (default)
  palantir.enabled    true               (default)
  palantir.minQuests  2                  (default)
```

Mark non-default values with `(custom)` instead of `(default)`.

### Step 3: Ask What to Change

Ask the user what they'd like to change. Use `AskUserQuestion` with these options:

1. **Change settings** — modify specific values
2. **Reset to defaults** — delete the config file
3. **Done** — exit without changes

If the user picks "Change settings", ask which settings to modify. Present each setting with its current value and valid options. Use the schema below for validation.

### Step 4: Write Config

Write only non-default values to `~/.claude/fellowship.json`. If all values match defaults, delete the file instead (no point keeping it). Validate each value against the Schema Reference below before writing.

### Step 5: Confirm

Read back the file and show the updated settings table from Step 2.

## Schema Reference

This is the canonical schema for `~/.claude/fellowship.json`. Other skills reference this table.

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
