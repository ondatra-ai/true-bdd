# Architecture read: {{ticket draft}}

Claude wrote {{a ticket draft}} in TrueBDD and will publish it next. You check one thing only: whether it fits the generation architecture. Change nothing; your final message is your report.

## Read first

- Document: {{path or issue #N saved through `--issue`}}.
- The architecture: `docs/adr/generation-architecture.md`, its six invariants in `.claude/rules/generation.md`, and the decisions behind them, ADR-0040 and ADR-0041. The rule for tickets: `docs/agents/issue-tracker.md`, section Architecture fit.
- The Owner's decisions, when a change's place is in doubt: {{the `docs/qna/` log this ticket carries out}}. Read `docs/qna/` as `docs/agents/domain.md` says: oldest to newest, a newer entry overriding an older one.

## Question

For every change the ticket asks for, answer three things:

1. Its place in the architecture: the command that writes it (`build steps`, `build tests`, `build code`, or none when people write it) and what holds it (the Steps package, the tests, the code, `pkg/testkit`, `pkg/proxy/cli`, `utilities/cliproxy`, or a Specification document).
2. Each of the six invariants it keeps or breaks.
3. Whether the ticket names that place, as the Architecture fit section asks.

A broken invariant is a finding unless the ticket names the ADR it amends. Report nothing else: correctness, completeness, feasibility and wording outside the architecture belong to the ticket's separate plan-read.

## Report

A table: change, place, invariants broken (or none), place named in the ticket (yes or no). Then the findings, most serious first, each with the quoted passage, the invariant, evidence (`file:line` or the architecture's own words) and confidence. If the ticket fits, say so in one line. No preamble; at most {{500}} words.
