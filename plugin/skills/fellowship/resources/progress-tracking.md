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
| quest-auth-bug | Quest | Implement | ███░░░ 3/6 |
| quest-rate-limit | Quest | Research (HELD) | █░░░░░ 1/6 |
| scout-auth-analysis | Scout | Validating | ██░░ 2/3 |

**Quests:** 2 active (1 held) | **Scouts:** 1 active | **Completed:** 0
```

When a quest is held, append `(HELD)` to its phase and include the hold reason if present. Include held count in the summary line.

When companies are defined, group quests by company in the status report:

```
## Company: API Work (1/2 quests in Implement+)

| Task | Type | Phase | Progress |
|------|------|-------|----------|
| quest-add-endpoint | Quest | Implement | ███░░░ 3/6 |
| quest-add-tests | Quest | Research | █░░░░░ 1/6 |
| scout-review-api | Scout | Investigating | █░░ 1/3 |

## Ungrouped

| Task | Type | Phase | Progress |
|------|------|-------|----------|
| quest-other-task | Quest | Plan | ██░░░░ 2/6 |
```

## Phase-to-Progress Mapping

Quest phases:
- Onboard = 0/6, Research = 1/6, Plan = 2/6, Implement = 3/6, Adversarial = 4/6, Review = 5/6, Complete = 6/6

Scout phases:
- Investigating = 1/3, Validating = 2/3, Done = 3/3

- Use filled/empty block characters for visual progress
- Pull phase from task metadata `phase` field via `TaskList`
- Pull last gate context from the most recent gate message or teammate update
