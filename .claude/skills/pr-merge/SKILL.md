---
name: pr-merge
description: Merge the current branch's PR with one command — three bounded CodeRabbit review rounds, triage, fixes, ClickUp deferrals, approval and merge. Use whenever the user wants to merge a PR, finish a branch, land changes, or says "merge this", "land it", "ship it", "we're done with this branch".
---

# PR Merge

**Only on the user's explicit command to merge** (CLAUDE.md, CRITICAL).

```bash
./.claude/skills/pr-merge/run.sh [pr]
```

That is the whole skill — it orchestrates the reviews, triage, fixes,
tickets, `pr-commit`, approval, merge and post-mortem. Run it in the
**background**; a full loop takes close to an hour. Then report what it
printed: the per-round band table, what was fixed, what was ticketed, any
anomalies, and the merge result.

## Exit codes

- **0 / 1** — merged (1 = `merge.sh` also reported a problem).
- **2** — a required tool is missing.
- **3** — a round did not finish. **State is saved; re-run to resume.**
  Usually the review never arrived — **do not post `@coderabbitai review`
  yourself.** If the answer was `Review rate limited`, the hourly quota
  (~4) is spent and asking again cannot help. Report and wait.

## Rules

- Never merge without an explicit user command, whatever the checks say.
- The command prints the `fix-now` queue and stops. Working it is
  `fix-queue`, on a new command — landing this PR authorises nothing more.
- To diagnose a non-zero exit: `threads.sh status|list|preflight [pr]`, or
  `orchestrate.py --pr N --only land`. A `✗ MISMATCH` from `list` means a
  finding is unaccounted for — fix `render_review.py` before trusting any
  count.
