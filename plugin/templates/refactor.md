---
name: refactor
description: Code refactoring with behavior preservation
keywords: [refactor, restructure, reorganize, simplify, clean, extract, rename, move]
---

## Research Guidance
- Identify the full scope of code to refactor
- Understand current behavior thoroughly before changing structure
- Map all callers and dependents of the code being changed
- Document the current behavior as a baseline

## Plan Guidance
- Plan incremental changes — each step should preserve behavior
- Ensure existing tests cover the behavior before refactoring
- If test coverage is insufficient, plan to add characterization tests first

## Implement Guidance
- Make one structural change at a time
- Run tests after every change — refactoring should never break tests
- If tests break, the change wasn't behavior-preserving — revert and rethink

## Review Guidance
- Verify no behavior changes — diff should show structural changes only
- Confirm all existing tests still pass without modification
- Check that the refactored code is actually simpler/clearer
