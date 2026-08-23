# The `main` protection ruleset — provenance and adaptation

**Status**: in force since the repo was set up; this file is the rationale, recorded
2026-08-23. Not a plan — nothing here is pending.
**Kind**: infrastructure note. A GitHub ruleset lives on GitHub, so no file in this
repository can carry it as a comment; that is why it is written down here.

## Where it came from

The "Main Protection" ruleset (id `20972312`) is modelled on the one in
`speedandfunction/website`. Read the live rules with:

```bash
gh api repos/ondatra-ai/true-bdd/rules/branches/main
```

Classic branch protection was **deleted** when the ruleset landed — the two stack if
both exist. Consequence: `/branches/main/protection` now 404s, and that 404 does not
mean the branch is unprotected.

## What was dropped in the adaptation, and why

The source ruleset required **code scanning** and **Copilot** review. Both were removed
here: this repo runs neither, and a required check that never reports blocks every PR
forever. Re-importing the source ruleset wholesale would reintroduce exactly that
deadlock — if you ever re-sync from `speedandfunction/website`, drop those two rules
again.

## What it requires

Two green checks (`gates`, `CodeRabbit`), one approving review, every review thread
resolved, squash-only merges, linear history; deletion and force-push forbidden.

## Three properties worth knowing

- **The admin role bypasses all of it on a PR merge** (`bypass_actors`, `pull_request`
  scope), and `bypass_actors` does not appear in the `rules/branches/main` response. So
  a merge succeeding proves nothing about the preconditions. What actually enforces them
  is `merge.py`: it resolves every thread at the end of every round, and escalates to
  `--admin` only after the plain merge is refused, printing the refusal it escalated past.
- **`dismiss_stale_reviews_on_push` and `require_last_push_approval` are on**, so every
  push voids the approval — get the re-review *after* the last commit. Only `killev` has
  write access and GitHub forbids self-approval, so the approval comes from CodeRabbit's
  `APPROVED` review; the admin bypass is what keeps that from being a deadlock when the
  bot does not deliver.
- **`require_code_owner_review` is on but inert**: there is no `CODEOWNERS` file, so no
  path has an owner. Adding one makes the rule bite immediately, across every PR.
