# Codex mechanics

Shared by the `test-author` and `test-fixer` agents (the review-loop procedure + scoring
is the "## The review loop + scoring" section below).

## Non-interactive invocation

`codex exec` blocks silently on approval prompts and hangs headlessly forever UNLESS you pass a sandbox flag. Always pass one:

| Mode | Flag | Use for |
|---|---|---|
| Read-only | `-s read-only` | risk discovery, plan critique, audit (default) |
| Workspace-write | `-s workspace-write` | run tests / build / mutate files |

```bash
mkdir -p ./tmp
codex exec -s read-only --ephemeral -C "$PWD" --color never \
  -c model_reasoning_effort=low \
  -o ./tmp/codex-review.md \
  - < ./tmp/codex-prompt.md
```

- `-s read-only` — sandbox policy → autonomous exit. **Prevents the hang.**
- `--ephemeral` — no persisted session. `-C "$PWD"` — repo root. `--color never` — clean text.
- `-c model_reasoning_effort=low` — stops mechanical verification crawling (a `max` run once timed out echoing 5,000 `node_modules` files). Raise only for hard reasoning.
- `-o ./tmp/codex-review.md` — final answer to a file (trace can't bury it). Add `--output-schema <schema.json>` for structured JSON.
- `- < ./tmp/codex-prompt.md` — prompt via stdin (no quoting hell).

`codex exec` prints nothing until exit — launch as a **background** task and arm a Monitor on exit; cover success AND timeout/hang in the filter (silence ≠ "still thinking").

Wrapper: `./.claude/scripts/codex.sh <ro|auto> <prompt-file> [label]` (bakes the flags + tees trace).

## Playwright access for Codex

- **Run tests / hit the site (no setup):** `-s workspace-write` lets Codex run `npx playwright test` and shell to the dev server.
- **Open the site in Playwright MCP (one-time):**
  ```bash
  codex mcp add playwright -- npx @playwright/mcp@latest
  codex mcp list
  ```
  MCP tool calls aren't shell commands, so they work under `-s read-only`. Ensure the dev server is running so there's a site to open.

## Writing the Codex prompt

In one shot give it: the **task** (the requirements list for the test-author, or the reproduce block + specs for the task-blind fixer); **context** (the relevant files, `CLAUDE.md`); the **artifact** to critique (the current diff + test results); what to **return** (findings only — each with location, evidence, and a concrete fix; explicit verify/challenge re-checks from round 2 on — NO scores, the driver scores); and an instruction to **run commands to verify its own claims** rather than reasoning from memory. The `codex_prompts` templates bake this in.

## The review loop + scoring

The `test-author` and `fixer` agents run a bounded codex↔crush loop. **The caller passes `codex_cap` ∈ {0, 1, 3, 5}** — the number of cycles (`0` = no review). codex reviews (read-only, findings only); the AGENT scores; crush applies the keeps and re-runs ALL tests.

**Full context every round.** Each codex prompt carries, in full: (1) the task — test-author: requirements + the reconcile/expected-RED plan; fixer (task-blind): the reproduce block + the e2e specs being greened, read from disk (never a brief); (2) the complete current diff under review; (3) all prior findings (rounds 1…N−1); (4) the disposition of each prior finding (applied where/how, or skipped + the reason). The `codex_prompts` templates bake these slots in.

**codex's jobs.** Round 1: fresh findings only. Round 2+: (a) verify each APPLIED finding is correctly and completely implemented; (b) challenge each SKIP; (c) fresh findings. No scores, no ranking — codex runs commands to verify its own claims.

**The AGENT scores** each finding: one composite 1–10 plus four pass/fail gates; keep only if **composite ≥7 AND every gate passes** — **Correctness** (right, not merely asserted), **Evidence** (grounded in commands codex ran, not memory), **Scope fit** (inside the task's intent), **Regression risk** (doesn't break existing behaviour or the only-expected-red invariant). Record each kept item's composite + gates + one-line reason in `codex_ledger`.

**The cycle** (repeat ≤ `codex_cap`): fill the `codex_prompts` review template → write it to a prompt file under `codex_artifacts` (DISTINCT from the wrapper's `-o` answer file — a shared path overwrites the prompt) → run `<codex_wrapper> ro <prompt-file> <label>` FOREGROUND/blocking → score → fill the matching `crush_prompts.*_apply_review` template with the keeps and pipe it to `<crush_wrapper> <role> - <label> --continue` (SAME crush session) → crush applies, re-runs ALL tests (only-expected-red for the author / fully green for the fixer), refreshes `result.json` → record the round. Stop at the cap, or earlier when a round is DRY (every prior application verified clean, no skip-challenge survives, no fresh finding passes the gates).

**Impossible-in-code (fixer).** The fixer never edits a driving e2e/BDD test; if a spec genuinely cannot be satisfied in code, STOP and escalate with evidence rather than weaken it.
