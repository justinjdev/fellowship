---
name: feature
description: New feature with requirements analysis and integration testing
keywords: [add, feature, new, implement, create, build, support]
---

## Research Guidance
- Clarify requirements — what exactly should this do?
- Identify existing patterns in the codebase to follow
- Find integration points — where does this feature connect?
- Check for prior art or related features

## Plan Guidance
- Design the API surface or user-facing interface first
- Plan for backwards compatibility if modifying existing interfaces
- Define test strategy: unit tests for logic, integration tests for wiring

## Implement Guidance
- Follow existing patterns in the codebase
- Build the smallest working version first, then iterate
- Write tests alongside implementation, not after

## Review Guidance
- Verify the feature meets the stated requirements
- Check that new code follows existing conventions
- Ensure test coverage for happy path and key edge cases
