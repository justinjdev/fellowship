---
name: lorebook
description: Load phase-specific guidance from a quest template. Invoke at the start of each quest phase when a template has been assigned.
---

# Lorebook — Quest Template Guidance

Load and apply phase-specific guidance from your assigned quest template.

## When to Use

Invoke at the start of each quest phase when your spawn prompt includes a `TEMPLATE:` assignment.

## Process

1. **Resolve the template file** from three directories (highest priority first):
   - Project: `.claude/fellowship-templates/{name}.md`
   - User: `~/.claude/fellowship-templates/{name}.md`
   - Built-in: `<plugin>/plugin/skills/lorebook/templates/{name}.md`, where
     `<plugin>` is the most recently installed version directory under
     `~/.claude/plugins/cache/justinjdev/fellowship/`

   Use the first match found. If no file exists, skip silently.

2. **Read the section** matching your current quest phase. The lifecycle has
   four phases and a template has a section for each:

   | Phase | Section | Covers |
   |-------|---------|--------|
   | Research | `## Research Guidance` | What to read and understand first, and what the project's prior art is |
   | Plan | `## Plan Guidance` | What a plan for this kind of work must name before it is complete |
   | Implement | `## Implement Guidance` | Project-specific rules, generators, and commands to use while writing |
   | Review | `## Review Guidance` | What adversarial review, convention review, and verification must confirm before the PR |

   A template missing a section is not an error — that phase simply has no
   extra guidance. Review is the last phase, so its section is also the last
   guidance a quest loads: put anything that must be true before the PR opens
   there, not in a fifth section.

3. **Apply the guidance** as advisory context for your current phase. Template
   guidance supplements the quest skill's phase requirements; it never
   overrides or waives them, and it never removes a gate.

## The built-in example

`example` is the one template fellowship ships. It has no keywords, so it never
auto-suggests — assign it explicitly (`template: example`) to see the shape, or
copy it into `.claude/fellowship-templates/` and rewrite it for your project.
`/scribe` writes a real one by interviewing you about the work.
