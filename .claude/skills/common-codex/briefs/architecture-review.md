# Review, architecture axis: {{change}}

Review one TrueBDD change for one thing only: whether each part of it fits the generation architecture. Change nothing; your final message is your report.

## Change

- Diff: `git diff {{fixed point}}...HEAD {{path filters}}`, from {{checkout or worktree path}}. This is the whole change under review.
- Commits: `git log --oneline {{fixed point}}..HEAD`.
- The ticket it carries out: {{issue numbers, saved through `--issue`}}.

## The architecture

`docs/adr/generation-architecture.md`, its invariants in `.claude/rules/generation.md`, and the decisions behind them, ADR-0040, ADR-0041 and ADR-0042. When a change's place is in doubt, the Owner's decisions: {{the `docs/qna/` log the ticket carries out}}, read oldest to newest, a newer entry overriding an older one.

## Question

For every file the diff changes, answer three things:

1. Its place: the command that writes it (`build steps`, `build tests`, `build code`, or none when people write it) and what holds it (a checklist, the Steps package, the tests, the code, `pkg/testkit`, `pkg/proxy/cli`, `utilities/cliproxy`, a Specification document, the engine, or this repository's tooling).
2. Each invariant it keeps or breaks.
3. Whether that place matches the one the ticket names for it.

A broken invariant is a finding unless the ticket names the ADR it amends. Report nothing else: standards and the ticket's spec belong to the review's other two runs.

## Report

A table: file or group of files, place, invariants broken (or none), matches the ticket (yes or no). Then the findings, most serious first, each with `file:line`, the invariant, evidence and confidence. If the change fits, say so in one line. No preamble; at most {{600}} words.
