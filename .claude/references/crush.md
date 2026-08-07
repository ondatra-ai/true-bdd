# Crush mechanics

Shared by the `test-author` and `fixer` agents. **Crush (GLM-5.2, 1M context, via
zhipu-coding) is the ONLY sanctioned writer of repo files** — the driver agents have no
Write/Edit tools and drive crush through the wrapper. Take the wrapper path, artifact
dir, roles, and prompt templates from `docs/context/paths.yaml` (`crush_wrapper`,
`crush_artifacts`, `crush_prompts`); never hardcode them.

## Invocation (always via the wrapper — a bare `crush run` is read-only)

`<crush_wrapper> <author|fixer> <prompt-file|-> [label] [--continue]`

- **role** — exported as `CRUSH_GUARD_ROLE`, selects the write sandbox (below). No role
  = fail-closed (read-only).
- **prompt** — `-` reads a QUOTED heredoc from stdin (the drivers have no Write tool, so
  they pipe a filled `crush_prompts` template); or a prompt-file path.
- **`--continue`** — resume crush's most recent session for follow-up turns (SAME
  session). Drivers run strictly sequentially, so "most recent" is safe.
- **artifacts** — prompt at `tmp/crush/<label>.prompt.md`, transcript at `<label>.out`,
  guard log at `hook.log`.
- **stall** — chatty child output pipe-deadlocks crush's shell, so a hung run is
  TREE-KILLED after `CRUSH_TIMEOUT` (default 1800s; a healthy author round ~20–25 min at
  xhigh) with **exit 124**. Run the wrapper foreground/blocking; on 124 relaunch once,
  then return a blocker.

## Sandbox roles (hook-enforced by `.crush/hooks/guard.py` — the ONLY enforcement)

`crush run` has NO permission gate; the PreToolUse write-guard is the sole gate.

| Role | file writes under | bash it may run |
|---|---|---|
| `author` | `tests/harness/` (+ `tmp/crush/`) | `playwright test` + `npm run` in `tests/harness`, `tsc --noEmit` in `tests/harness`, read-only cmds (ls/cat/grep/rg/find/head/tail/git status/git diff) |
| `fixer` | `harness/` (+ `tmp/crush/`) | all of the above **plus** `tsc` / `next build` / `npm run` / `npm install` in `harness/` |

- Unknown role, or any tool not on the allowlist → **DENIED** (default-deny — unknown
  tools are treated as writers). A write outside the sandbox, arbitrary bash, or
  compound `; | &` / redirects (except the one sanctioned `> tmp/crush/*.log 2>&1` on a
  test run) is denied.
- **Consequence:** the `author` role CANNOT run the unit suite (`cd harness && npm run
  …`) — only `fixer` can. So the test-author's "ALL tests" is the e2e suite only.

## Gotchas (non-negotiable)

- **`--reporter=dot` ALWAYS.** Chatty reporters deadlock crush's shell — pass
  `--reporter=dot`, or redirect `> tmp/crush/<name>.log 2>&1` and read the log.
- **Crush knows NOTHING about this repo.** Inline every concrete path + convention into
  the prompt (that is what the `crush_prompts` templates are for) — crush never reads
  `paths.yaml`, so nothing may reach it as a bare paths.yaml key or an unfilled
  `{{placeholder}}`. For UI work, resolve `design_system` yourself and inline its paths
  (tokens, SPEC, prototype) — crush does not know the design system exists.
- **Model-pin trap.** `glm-5.2[1m]` is a DISPLAY suffix, not a config id — pinning it
  silently falls back to global state. Correct pin (already in `.crush.json`):
  `{"model": "glm-5.2", "provider": "zhipu-coding"}` (context_window 1,000,000). Verify
  which model actually answered via `.crush/crush.db`
  (`SELECT provider, model FROM messages ORDER BY rowid DESC`; `created_at` is SECONDS).
- **Agent-definition caching.** Subagent definitions cache at session start — edits to
  `.claude/agents/*` need a FRESH session to take effect.
- **Verify, don't trust.** Crush's claims are not evidence. The driver runs `git status`
  after every crush call (sandbox guardrail) and reports from crush's `result.json` /
  transcript, never from crush's prose.

## MCP (`crush.json`)

Crush has its own MCP servers (context7, an interactive terminal, playwright), but the
guard default-denies any tool not on its allowlist, so under a role crush effectively
works through file tools + the bash whitelist + read-only tools. Do not rely on crush's
MCP for any enforcement-sensitive step.
