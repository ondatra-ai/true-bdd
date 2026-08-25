# Issue tracker

How an agent resolves a reference to the work item behind a change. Required
by `.claude/skills/code-review` and `.claude/skills/triage`, both vendored
from `mattpocock/skills`, whose preambles assume this file exists and tell the
user to run `/setup-matt-pocock-skills` when it does not.

## Two trackers, and they are not interchangeable

**ClickUp is where work is planned.** List `901523097822` holds every Ticket:
what to change, why, how to verify it, and the fields
`.claude/skills/task-handle/ticket-schema.yaml` requires of anything an agent
may take unattended. A Ticket is the **spec** — when a review asks "what did
this change set out to do", the answer is a Ticket body.

**GitHub issues are not used for planning.** The repository is
`ondatra-ai/true-bdd`; it has pull requests and CI, and the occasional issue,
but the backlog does not live there. A bare `#42` in a commit message is a
**pull request**, not an issue, unless the surrounding text says otherwise.

## Resolving a reference

| what you have | how to resolve it |
| --- | --- |
| a ClickUp id (`86cb8hjf7`) or an `app.clickup.com/t/...` URL | `mcp__claude_ai_ClickUP__getTask` — returns the untruncated description |
| `#42` in a commit message or PR body | `gh pr view 42` first; `gh issue view 42` only if that misses |
| a branch name | `gh pr list --head <branch> --json number,title,body` |
| nothing at all | the spec was passed as an argument, or there is none — see below |

The PR body is the bridge: `pr-update` and `task-handle` both put the ClickUp
URL there, so a PR resolves to its Ticket without guessing.

## When a skill is run unattended

`task-handle` passes the fixed point (`main`) and the spec (the Ticket body)
to `code-review` as arguments, precisely so neither lookup above is reached
and neither of that skill's two "ask the user" branches can fire. If you are
reading this during an automatic run and still do not have a spec, that is a
**decline**, not a question — the run has `AskUserQuestion` removed.

## What is not a spec

`docs/scenarios.yaml` is the BDD registry and `docs/product/`,
`docs/architecture/` are the engine's specification documents. They describe
the product, not the change under review. Do not substitute one for a missing
Ticket: a review measured against the wrong spec is worse than one that
reports having none.
