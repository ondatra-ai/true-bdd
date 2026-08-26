---
name: task-handle
description: Take one ClickUp Ticket from TO DO to merged and DONE, unattended — check it is complete, start, do the work, scope-check, commit, review, merge, close. Halts untouched if any field is missing. Pass the ticket id. Use when the user names one ticket to work, or says "handle this ticket". For the whole queue use task-loop.
disable-model-invocation: true
disallowed-tools: AskUserQuestion
argument-hint: "<ticket-id>"
allowed-tools: Bash(${CLAUDE_PROJECT_DIR}/.claude/hooks/history.sh *) Bash(git *) Bash(gh *) Bash(go *) mcp__claude_ai_ClickUP__getTask
---

# Task Handle

Ticket: `$ARGUMENTS`. One Ticket, one branch, one PR, one outcome.

```!
git -C "${CLAUDE_PROJECT_DIR}" rev-parse --abbrev-ref HEAD
git -C "${CLAUDE_PROJECT_DIR}" status --short
```

**Invoking this skill is the mandate to merge without asking** — the one place
that overrides CLAUDE.md's "NEVER merge a pull request without an explicit
command".

Refuse to start if the checkout is not on `main`, or the tree is dirty.

## 1. Check

`getTask` the Ticket and hold it against `ticket-schema.yaml` in this
directory. Every field it lists must be filled, and the body must carry all
four headings.

**Anything missing, or `Scope: PROJECT` → halt.** Report `not started` and
exactly which field or heading is empty. Write nothing: no `updateTask`, no
`setCustomFieldValue`, no comment, no status. The Ticket stays in `TO DO`
untouched, so it is not a `FAILED` — a human fills it in.

## 2. Start

```text
/task-start <ticket-id>
```

```bash
"${CLAUDE_PROJECT_DIR}/.claude/hooks/history.sh" mandate <ticket-id>
```

The mandate tells `scripts/merge` to use the automatic triage row (1–8 skip,
9–10 ticket). It is honoured only while this Ticket is bound.

## 3. Work

Only what the Ticket describes. Growing past it is a decline. If the change is
already in the code, or the Ticket is wrong about it, decline with the evidence.

## 4. Scope check

`git diff --stat main` against the Ticket's `Expected Changes` globs. One
chance: narrow to scope, or decline.

## 5. Commit

Invoke `pr-commit` with `auto --changed main`. Put the ClickUp URL in the PR
body.

Gates red → fix and retry. **Five retries across the whole Ticket**; the sixth
is a decline.

## 6. Review

Invoke `code-review` with `main` as the fixed point and the Ticket body as the
spec, both as arguments. Spec axis only; Standards findings are logged and
block nothing.

- missing or wrong → fix and retry, same cap of five;
- present but not asked for → decline.

## 7. Merge

Has the user said anything since step 2? Then the mandate for this Ticket is
cancelled:

```bash
"${CLAUDE_PROJECT_DIR}/.claude/hooks/history.sh" unmandate
```

Leave the PR open, report `awaiting merge`, stop. The user merges and runs
`/task-done`.

Otherwise invoke `pr-merge`. It ends in `git checkout main`.

## 8. Close

```text
/task-done
```

## Declining

Refusing to merge on the merits: unasked-for work, a scope mismatch that would
not narrow, gates red after five retries, or any point needing a question.

```text
/task-fail <why, one or two sentences>
```

Leave the branch and the PR as the failure left them — no close, no delete, no
revert.

## Report

One line: id, title, outcome (`DONE` · `FAILED` · `awaiting merge` ·
`not started`), PR URL. For `not started`, name the missing field.
