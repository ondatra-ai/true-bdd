---
name: pr-merge
description: Merge the current branch's PR using squash merge, delete the remote branch, switch to main, pull latest, and clean up the local branch. Resolves blocking review threads first — validating each comment, fixing the valid ones and rejecting the invalid ones with a reason. Use this skill whenever the user wants to merge a PR, finish a branch, land changes, or says things like "merge this", "land it", "ship it", "merge the PR", or "we're done with this branch".
---

# PR Merge

## 1. Check what is blocking

```bash
./.claude/skills/pr-merge/threads.sh status
```

`main` requires the `gates` check **and** no changes-requested review. If
`state` is `BLOCKED`, work through step 2 before merging. If `gates` is
`IN_PROGRESS`, wait for it — do not merge with `--admin` to skip it.

## 2. Clear the review threads

```bash
./.claude/skills/pr-merge/threads.sh list
```

Then, **one thread at a time**:

1. **Validate it against the current code.** Read the file the comment
   points at. A finding can be stale (already fixed), misplaced (the
   line moved — the helper flags `[OUTDATED]`), or simply wrong about
   this codebase. Do not assume the reviewer is right, and do not assume
   it is wrong.
2. **Act:**
   - *Valid* → fix it. Fix at the root: if three comments share one
     cause, fix the cause and say so on the other two. Add a test when
     the finding is a real defect rather than a wording change.
   - *Invalid* → leave it. A reason the author can check beats a
     dismissal.
3. **Reply**, saying which it was and why:
   ```bash
   ./.claude/skills/pr-merge/threads.sh reply <comment-id> "Valid — fixed …"
   ./.claude/skills/pr-merge/threads.sh reply <comment-id> "Rejected — …"
   ```
   For a rejection, give the evidence: the API's documented behaviour,
   the constraint the suggestion would break, the reason the code is
   already correct.
4. **Resolve that thread, and only that thread:**
   ```bash
   ./.claude/skills/pr-merge/threads.sh resolve <thread-id>
   ```

Then re-run gates (`./.claude/skills/pr-commit/gates.sh`) and commit the
fixes.

### Rules

- **Never resolve a thread you have not read and replied to.** Resolving
  claims the comment was answered. There is no bulk-resolve verb for
  exactly this reason: a reviewer bot posts a fresh review on every push,
  so `list` grows while you work, and a blind loop will sweep up comments
  that arrived seconds ago and were never looked at.
- **Re-run `list` after pushing.** A new push means a new review.
- **A rejection is a technical argument, not a dismissal.** If it cannot
  be defended in two sentences against the actual code, it is a fix.
- **Never dismiss the review itself** (`gh pr review --dismiss`) to get
  past the block. Answer the comments; the bot re-reviews and clears its
  own verdict.
- **Ask before merging with `--admin`.** That overrides branch
  protection, which is the user's call and not the default path.

## 3. Merge

```bash
./.claude/skills/pr-merge/merge.sh
```

Squash-merges, deletes the remote branch, switches to main, pulls, and
removes the local branch.
