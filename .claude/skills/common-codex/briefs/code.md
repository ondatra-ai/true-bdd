# Code: {{ticket or change}}

Implement {{issue #N or described change}} in TrueBDD, alone and to the end. Nobody answers questions during the run. Your final message is the report Claude reads.

## Read first

- The ticket, spec and prior work: {{paths, issue numbers, commits}}. Issues supplied through `--issue` are saved at the paths in `## This run`.
- `CLAUDE.md`, `docs/agents/domain.md`, `GLOSSARY.md`, relevant `docs/adr/` and every `.claude/rules/*.md` matching a file you touch.
- `.claude/skills/mpc_tdd/SKILL.md`: build behavior test-first at the seams this task names.

## The change

{{What must change, the acceptance criteria, what is out of scope, and any decisions already made.}}

## Workspace and rules

`## This run` names your worktree, branch and starting commit. Stay inside that worktree. Use plain local Git commits, ending each commit message with `Refs #N`. Never push, open a PR, merge a PR, or write to GitHub. Never amend commits, force-push or change `.alint.yml` or `.golangci.yaml`. Leave vendored skills unchanged.

The engine's code belongs in `services/true-bdd/`, tooling in `scripts/`, tests in `tests/true-bdd/`; `pkg/` holds declared shared channels and typed CLI wrappers. Scripts do not import the engine. Read `CLAUDE.md` for the complete contracts.

Run every command in the foreground. No `nohup`, `setsid`, `&` or detached drivers: all command work must finish before you report the run's end.

## Done when

- {{Task-specific acceptance criteria.}}
- After edits, `go run ./scripts/cmd/alint {{every changed file}}` passes, including sibling findings it reports.
- `GOWORK=off go build ./...` and `GOWORK=off go test ./...` in each of `.`, `pkg/` and `services/true-bdd/` and `go run ./scripts/cmd/gates run --changed main` pass. Every single-command build has `-o ./bin/{{name}}`.
- If you touch `services/`, `pkg/testkit/` or `tests/`, also run `GOWORK=off go test -tags bdd ./tests/true-bdd/ -mode=services:replay,tests:replay`.
- Run the gates by hand before `git commit --no-verify`. Every change is committed on your recorded branch and the worktree is clean. Before reporting, merge {{base branch}} into your branch when the task requires integration, resolve conflicts there and repeat the affected checks.

## Report

Branch and tip SHA; what you built, file by file; each acceptance criterion met or not and why; check commands and results; decisions you made; omissions and their reasons; open questions and anything that needs manual work. No preamble; at most {{500}} words.
