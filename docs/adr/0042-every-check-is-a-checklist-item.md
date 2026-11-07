# Every check of a command is a checklist item, and verification reads only what it references

Round 8 of #249 was expected to stop at `build steps`' first critique of a step definition: 769,185 bytes for `given:project tree` on cut B against `engine.max_prompt_bytes` 150,000, 593,900 of them fixture files. In the grilling of 4 and 5 November the Owner moved from the consequence (a prompt over the limit) to its cause (a turn that receives far more than its question reads), and from there to what that turn is: one check among several `build steps` makes outside its checklist. Experiments on TrueBDD's own suite measured each option ([report](../research/2026-11-04-verification-experiments.md)).

We decided:

- **Every check a command makes about what it walks is a checklist item:** a **Q**, the verification, and its **F**, the fix. A Q is either a `prompt`, which a model answers, or `code`, which the engine runs, never both; a check needing both is two checklist items. `D:` becomes a Q with the `code` option, amending [ADR-0015](0015-deterministic-checklist-prompts.md).
- **One checklist item answers one question.** A checklist has as many items as it takes to verify and update what the command walks.
- **The command decides what each checklist item is asked about:** one Item, an array of Items, or text.
- **Checklist items run in the order they are written.** The engine reorders nothing; where code checks must come first, the checklist is written that way. This replaces ADR-0015's rule that every `D:` runs before any `Q:` and skips the Item's model questions when it fails.
- **Without `--fix`** the walk goes on to the end and reports every failure, with no special rule after a failed code Q.
- **With `--fix`** a failed Q gets its F: the fix prompt, the person's choice at the fix menu, the apply. Then the same Q runs again, and then the whole checklist from its start, since an apply may turn an item that passed into a failure. Any F may answer that it cannot fix; the run then stops with its reason and what a person must correct, without asking for an Earlier step. An F fixes only what its Q found; a problem it did not find is neither its work nor a reason for cannot-fix. The Q hands its question, answer, context lines and the item's F text to the fix. A person who has not decided is a legitimate cannot-fix. `build steps`' `interface_cannot_serve` and `build code`'s `impossible` become instances of this rule.
- **The engine's own contracts are not checklist items:** the checks of the host's files at start, the rules of one AI turn (the answer's schema, the prompt byte limit) and the walk's limits. A checklist cannot check itself, and a run whose checklist is broken never reaches it.
- **Checklist first:** work a checklist item can do is written as one, not as engine logic.
- **Verification of step definitions is one checklist item** (the step called the critique until now; the word is dropped). Its question: does the generated step definition match what its steps mean under its concept and binding? Its one F rewrites the step definition by its findings, its pattern too when needed. Pattern capture and scope, the binding, each step's meaning and observing rather than computing are what the question checks, not five routes. This amends [ADR-0035](0035-build-steps-writes-definitions-per-concept.md).
- **Verification reads only what is referenced**, still tool-less: the definition's code and what that code calls; its concept, binding, glossary entries and Word mappings; only the Public interface entries the binding's `references:` name; only the runtime `go doc` of what the code calls; the bound steps as distinct forms with a few example values; no fixture files. The templates say exactly what they supply. This amends [ADR-0039](0039-prompt-evidence-paths-and-byte-limit.md)'s whole suite source and fixture contracts and narrows [ADR-0041](0041-test-runtime-knows-no-product.md)'s embedded `go doc` to what the code calls.
- **A host-specific fact a step body must meet is stated in the host's Specification,** in its architecture document, never only in a fixture file; fixture files stay test data.
- **How a step's arguments are read is a section of the architecture document:** a quoted literal, a regex running to the end of the step, a glob, a literal path, a token such as `{{CWD}}`. Today only this repository's agents' rules say so.

Measured on TrueBDD's 19 concept Items and 11 controls whose answers are known, every turn one after another:

| Shape | Linear time | Largest prompt | Cost | Controls right |
|---|---|---|---|---|
| Today's evidence, one turn per Item (17 Items) | 1,606 s | 1,927,468 bytes | $46.30 | 11 of 11 |
| By reference, one item for the question | 1,594 s | 51,879 bytes | $7.35 | 10 of 11 |
| By reference, three items | 2,480 s | 49,069 bytes | $16.33 | 9 of 11 |
| By reference, five items, one per aspect | 3,223 s | 48,687 bytes | $24.14 | 10 of 11 |

The misses trace to runtime doc comments that do not say what the code does, and to `CALC_REAL_GO`, a fixture-only fact this ADR moves into the host's Specification.

## Considered Options

- **A larger prompt limit:** it treats the size, not why the turn receives what it never reads.
- **Portions of one definition per turn, or minimal embedding with fixture files:** both still exceeded 150,000 bytes on a few Items.
- **Verification reading its evidence by path:** kept in reserve; selection by reference made it unnecessary.
- **Three or five checklist items for the verification of step definitions:** 1.6 and 2 times the linear time, 2.2 and 3.3 times the cost, at equal or lower accuracy; three of the five aspects read the same evidence.
- **A table of step-form meanings:** the template form [ADR-0034](0034-concepts-replace-catalogue-templates.md) retired comes back. A handful of capacious templates, each serving about fifty steps, stays an open idea: five patterns bind 70% of TrueBDD's bound steps.
- **Engine contracts as checklist items:** a checklist cannot check itself.

## Consequences

- The shipped checklists are rewritten in the order they run, their `D:` prompts as code Qs. The checks `build steps` makes outside its checklist (the binding judgement, the engine's pattern check, the checks after an apply, the verification of step definitions) move into checklists, each through its own ticket.
- Scenarios that pin the old order and evidence change: `checklist-checks-001-checks-run-before-questions` (every `D:` before any `Q:`, whatever the file order) and `concept-bodies-003-fixture-contract` (a contract only a fixture states).
- The runtime's doc comments must say what the code does: `CheckStdout`, `Mode`, `RunCommand`, `HostRoot`, `PrepCmds` and `ExpectedExitCode` do not.
- `GLOSSARY.md`'s Prompt, Built-in check and Cell entries change with the code that merges `D:` into Q.

Grilling log: `docs/qna/20261104-081811-critique-size.yaml`, Q1 to Q27, with the research entries and the entry Claude was delegated in Q26. The F scope, handover, Earlier step term and pending-decision outcome were settled in `docs/qna/20261105-154500-cannot-fix-at-recording.yaml`, Q3 to Q6.
