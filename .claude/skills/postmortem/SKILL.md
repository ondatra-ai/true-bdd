---
name: postmortem
description: Read one merge run back and file what it suggests, without merging anything. Use when the user asks for a postmortem of a PR, wants to know why a run took as long as it did, or asks why a merge skipped its own postmortem.
---

# Postmortem

Read-only. It reads a session history file and a timing record, asks for
proposals against the merge scaffolding, and files the ones above the triage
floor as ClickUp Tickets. It never edits the worktree, and stops if it finds
that anything did.

`pr-merge` runs this at its own tail, but only for a run that earned it — one
that did not finish automatically, or a task that took over 15 minutes. Every
other run skips it and says so, which is when this skill is the way to ask.

```bash
go run ./scripts/cmd/postmortem --pr <n> --history docs/history/<file>.md
```

Both flags are required. Name the history file rather than letting anything
infer it: `docs/history/hook-state` is one repo-global pointer, so with two
sessions live in one checkout it points at whichever wrote last — that is how
PR #99's postmortem came to read the other session's turns.

`--timings <path>` overrides the record the merge loop wrote to
`tmp/merge/timings.json`. An absent record is not a failure; the run is
reported to the model as unmeasured.
