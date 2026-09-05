# Progress Tracking

Gandalf maintains awareness of quest progress through two mechanisms:

1. **Task metadata**: Each teammate updates their task's `phase` metadata field at phase transitions via `TaskUpdate`. Gandalf reads this via `TaskList` when reporting status.
2. **Gate messages**: Gate transition messages from teammates provide the most recent context for each quest.

## Status Report Format

When the user asks for "status" or Gandalf proactively reports progress:

```
## Fellowship Status

| Task | Type | Phase | Progress |
|------|------|-------|----------|
| quest-auth-bug | Quest | Implement | ██░ 2/3 |
| quest-rate-limit | Quest | Research (HELD) | ░░░ 0/3 |
| scout-auth-analysis | Scout | Validating | ██░ 2/3 |

**Quests:** 2 active (1 held) | **Scouts:** 1 active | **Completed:** 0
```

When a quest is held, append `(HELD)` to its phase and include the hold reason if present. Include held count in the summary line.

When groups are defined, group quests by group in the status report. Run `~/.claude/fellowship/bin/fellowship group show <name> --json` for a group's canonical membership and completed count rather than reconstructing it from task metadata:

```
## Group: API Work (1/2 quests in Implement+)

| Task | Type | Phase | Progress |
|------|------|-------|----------|
| quest-add-endpoint | Quest | Implement | ██░ 2/3 |
| quest-add-tests | Quest | Research | ░░░ 0/3 |
| scout-review-api | Scout | Investigating | █░░ 1/3 |

## Ungrouped

| Task | Type | Phase | Progress |
|------|------|-------|----------|
| quest-other-task | Quest | Plan | █░░ 1/3 |
```

## Phase-to-Progress Mapping

Quest phases:
- Gates passed, not phases entered: Research = 0/3, Plan = 1/3, Implement = 2/3, Review = 3/3. A plan-driven quest starts at Implement and has one gate: Implement = 0/1, Review = 1/1.
- A quest in Review has passed every gate; it is done when its entry status says `completed`, not when it reaches a phase.

Scout phases:
- Investigating = 1/3, Validating = 2/3, Done = 3/3

- Use filled/empty block characters for visual progress
- Pull phase from task metadata `phase` field via `TaskList`
- Pull last gate context from the most recent gate message or teammate update
