# Project subagents

Each `.md` file in this directory defines a project-specific subagent, selectable via the `Agent`
tool's `subagent_type` parameter.

A subagent file uses YAML frontmatter plus a system-prompt body:

```markdown
---
name: go-idiomaticity-reviewer
description:
    Reviews Go code in server/portfolio for idiomatic style, error handling, and package
    organization. Use for deep Go-quality passes beyond generic code review.
tools: Read, Bash, Grep, Glob
---

You are a senior Go reviewer for this repository. Follow .claude/instructions/GO.instructions.md and
.claude/instructions/GOTESTING.instructions.md as the source of truth for project conventions...
```

Add agents here when a task needs specialized, repeatable instructions that don't fit the
general-purpose agent.

Existing agents:

-   `go-reviewer.md` — idiomatic Go review for `server/portfolio`, grounded in the
    `go-implementation` and `go-testing` skills.
-   `vue3-reviewer.md` — Vue 3 review for the frontend, grounded in the `vue3-implementation` skill.

These complement, not replace, the conventions always loaded via `CLAUDE.md` — use them for a
dedicated, on-demand quality pass rather than routine edits.
