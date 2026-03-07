---
name: bugfix
description: Bug fix with reproduction and regression testing
keywords: [fix, bug, broken, crash, error, regression, issue, failing]
---

## Research Guidance
- Reproduce the bug with a minimal test case
- Identify root cause (not just symptoms)
- Check for related issues or similar bugs elsewhere
- Note the expected vs actual behavior

## Plan Guidance
- Write failing test that captures the bug
- Fix root cause, not symptoms
- Verify no regressions in adjacent functionality

## Implement Guidance
- Start with the failing test before touching production code
- Keep the fix minimal — don't refactor surrounding code
- Verify the fix doesn't break other tests

## Review Guidance
- Confirm fix addresses the reported issue
- Check edge cases around the fix
- Verify test covers the specific failure mode
