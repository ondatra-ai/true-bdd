# Fix: {{findings, failing checks or merge conflict}}

Fix {{accepted findings, failed checks or conflict}} for {{issue #N}} in TrueBDD. Work alone to a checked local commit. Your final message is the report Claude reads.

## Read first

- Ticket and spec: {{paths or issue numbers, saved through `--issue`}}.
- Branch history and diff: `git log --oneline {{base}}..HEAD`, `git diff {{base}}...HEAD`.
- `CLAUDE.md`, `docs/agents/domain.md`, `GLOSSARY.md`, relevant `docs/adr/`, and the matching `.claude/rules/*.md`.
- `.claude/skills/mpc_tdd/SKILL.md` for behavior changes.

## What to fix

{{Keep the relevant form and delete the others.}}

- Accepted findings, each with `file:line`, what is wrong and what would settle it: {{findings}}.
- Failed checks: {{commands and output or log paths}}.
- Conflict: merge {{integration branch}} into this branch, preserving both sides' intent, resolve conflicts and re-check.

Change nothing beyond that scope. `## This run` names the code run's worktree, branch and the tip this fix must add a commit to. Stay there. Use plain local Git commits ending with `Refs #N`; never push, open or merge a PR, write to GitHub, amend, force-push or change lint configs. Leave vendored skills unchanged.

Run every command in the foreground. No `nohup`, `setsid`, `&` or detached drivers: all command work must finish before you report the run's end.

## Done when

- {{Criteria for each accepted finding or failed check.}}
- `go run ./scripts/cmd/alint {{every changed file}}`, `GOWORK=off go build ./...` and `GOWORK=off go test ./...` in each of `.`, `pkg/` and `services/true-bdd/` and `go run ./scripts/cmd/gates run --changed main` pass. Fix sibling findings too.
- If touching `services/`, `pkg/testkit/` or `tests/`, `GOWORK=off go test -tags bdd ./tests/true-bdd/ -mode=services:replay,tests:replay` passes.
- Run gates by hand, then commit with `git commit --no-verify`. The worktree is clean and your branch has a new commit beyond its starting tip. Single-command builds use `-o ./bin/{{name}}`.

## Report

Branch and tip SHA; what you fixed, finding by finding or check by check; each check command and result; decisions left open and what you chose; what you did not fix and why; any manual work. No preamble; at most {{400}} words.
