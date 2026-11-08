# Architecture read: {{ticket draft}}

Claude wrote {{a ticket draft}} in TrueBDD and will publish it next. You check one thing only: whether it fits the generation architecture. Change nothing; your final message is your report.

## Read first

- Document: {{path or issue #N saved through `--issue`}}.
- The architecture: `docs/adr/generation-architecture.md`, its invariants in `.claude/rules/generation.md`, and the decisions behind them, ADR-0040, ADR-0041 and ADR-0042. The rule for tickets: `docs/agents/issue-tracker.md`, section Architecture fit.
- The Owner's decisions, when a change's place is in doubt: {{the `docs/qna/` log this ticket carries out}}. Read `docs/qna/` as `docs/agents/domain.md` says: oldest to newest, a newer entry overriding an older one.

## Question

For every change the ticket asks for, answer four things:

1. Its place in the architecture: the command that writes it (`build steps`, `build tests`, `build code`, or none when people write it) and what holds it (a checklist, the engine, the Steps package, the tests, the code, `pkg/testkit`, `pkg/proxy/cli`, `utilities/cliproxy`, or a Specification document).
2. Each invariant it keeps or breaks.
3. Whether the ticket names that place, as the Architecture fit section asks.
4. Checklist first: when it adds or changes engine logic that decides whether something passes, the checklist item that could do the same work (what it is asked about, its `Q` by prompt or by code, its `F`), or why none can, as the architecture's section Every command is a checklist draws it.

A broken invariant is a finding unless the ticket names the ADR it amends. Engine logic a checklist item could carry is a finding that names that checklist item. Report nothing else: correctness, completeness, feasibility and wording outside the architecture belong to the ticket's separate plan-read.

## Report

A table: change, place, invariants broken (or none), place named in the ticket (yes or no), checklist item that could carry it (or none, or not engine logic). Then the findings, most serious first, each with the quoted passage, the invariant, evidence (`file:line` or the architecture's own words) and confidence. If the ticket fits, say so in one line. No preamble; at most {{500}} words.
