---
name: lorebook
description: Load phase-specific guidance from a quest template. Invoke at the start of each quest phase when a template has been assigned.
---

# Lorebook — Quest Template Guidance

Load and apply phase-specific guidance from your assigned quest template.

## When to Use

Invoke at the start of each quest phase when your spawn prompt includes a `TEMPLATE:` assignment.

## Process

1. **Resolve the template file** from two directories (highest priority first):
   - Project: `.claude/fellowship-templates/{name}.md`
   - User: `~/.claude/fellowship-templates/{name}.md`

   Use the first match found. If no file exists, skip silently.

2. **Read the section** matching your current quest phase:
   | Phase | Section |
   |-------|---------|
   | Research | `## Research Guidance` |
   | Plan | `## Plan Guidance` |
   | Implement | `## Implement Guidance` |
   | Review | `## Review Guidance` |

3. **Apply the guidance** as advisory context for your current phase. Template guidance supplements but does not override the quest skill's phase requirements.
