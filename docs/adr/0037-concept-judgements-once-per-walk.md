# Each binding and each linked concept's words are judged once per walk, in `scen check` and `build steps` alike

ADR-0034 had `scen check` judge a Scenario's bindings afresh for every Scenario, trusting no earlier result, and ADR-0036 had it judge the words of a Scenario's linked concepts per Scenario. #264 measured what that costs on TrueBDD's own Registry: one pass made 2,198 binding and 2,198 word judgements for 19 distinct bindings and 19 concepts, bindings alone 59% of the turn time. Writing the answer is the time, so fewer judgements are what make a walk faster, not more workers. The experiments on a 25-Scenario sample kept the verdicts: judging each binding once cut its turn time by 60%, and its stricter shared turn failed the `AI CLI observations` binding that 16 per-Scenario turns had passed, a failure the check confirmed as real; judging each concept's words once cut theirs by 65% with no verdict changed. Linking each distinct step once left a borderline step unlinked in 3 of 4 shared runs, and was dropped. The closing review of PR #263 then found `build steps` still judging all of a suite's bindings in one turn, so one binding could pass one gate and fail the other.

We decided:

- **`scen check` judges each binding once per walk.** A binding is a concept, its block and its service. Its turn starts once every Scenario of the walk has answered the Prompts before it, and reads the product document, that concept and binding, the service's complete Public interface, and every distinct step of the walk's passing links that reached the concept for that service. Every Scenario whose link reached it answers from that one verdict, with its own context lines.
- **`scen check` judges each linked concept's words once per walk,** in a turn of its own over the product document and that concept. `us refine` still judges a Story's words in its own turn.
- **The link stays per Scenario.** Each Scenario's steps are linked in its own turn, afresh, as ADR-0034 decided.
- **`build steps` judges each binding the same way,** before any skeleton: one turn per binding over the steps of every Item, grouped or resumed, that reaches its concept for the service, through `scen check`'s bindings Prompt and the same evaluator path. There is one binding judgement, not two.
- **No verdict outlives its walk.** A `--fix` round that applies a fix starts a new walk, in which every binding and every concept's words are judged again.
- **`D: step-definitions` passes a step no definition binds yet.** `scen check` runs before `build steps` writes a Scenario's step definitions, so an unbound step is listed as `build steps`' gap, not failed; a step two definitions bind, or a definition recording no declared concept of the step's block, still fails.

This supersedes ADR-0034's "afresh for every Scenario, trusting no earlier result" for bindings and words, and ADR-0036's words judged "in `scen check` per Scenario". ADR-0035's "as `scen check` judges it" now holds in both commands.

## Considered Options

- **Every judgement per Scenario,** as ADR-0034 and ADR-0036 decided: 2,198 binding and 2,198 word judgements a pass where 19 of each answer the same questions.
- **More workers, fast mode, or less thinking.** The Owner dropped them: they do the same work faster and leave its complexity as it was.
- **The link once per distinct step.** It lost a borderline step that every per-Scenario run linked, even with the descriptions of every Scenario using it.
- **`build steps` keeping a suite's bindings in one turn,** with the difference recorded here. One binding could then pass `scen check` and stop `build steps`, or the reverse.

## Consequences

- A binding's verdict is shared: when it flips, every Scenario linking it flips with it.
- `build steps` makes one binding turn per binding where it made one per suite: `build-steps-writing-002` dispatches 12 validation turns, not 10.
- A binding judged outside a started walk is an engine error, not a per-Scenario turn.

Grilling log: `docs/qna/20261031-110300-scen-check-whole-registry-cost.yaml`, Q1, Q3, Q4, Q5 and Q6.
