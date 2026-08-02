# Codex mechanics

Shared by `identify-task` and `implement-task`.

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

Wrapper: `./.claude/skills/implement-task/scripts/codex.sh <ro|auto> <prompt-file> [label]` (bakes the flags + tees trace).

## Playwright access for Codex

- **Run tests / hit the site (no setup):** `-s workspace-write` lets Codex run `npx playwright test` and shell to the dev server.
- **Open the site in Playwright MCP (one-time):**
  ```bash
  codex mcp add playwright -- npx @playwright/mcp@latest
  codex mcp list
  ```
  MCP tool calls aren't shell commands, so they work under `-s read-only`. Ensure the dev server is running so there's a site to open.

## Writing the Codex prompt

In one shot give it: the **goal** (+ non-goals/constraints); **context** (files, `CLAUDE.md`, the plan, prior `docs/context/requirements.md` requirements); the **artifact** to critique (plan, or diff + test results); what to **return** (a numbered list ranked by goal-relevance, each with a one-line rationale and a concrete fix; explicit RESOLVED/NOT-RESOLVED re-checks when verifying); and an instruction to **run commands to verify its own claims** rather than reasoning from memory.
