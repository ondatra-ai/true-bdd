# Checklists mix deterministic and model Prompts

A checklist Prompt is either `Q:`, answered by a model, or `D: <check>`, answered by one of the engine's built-in checks, with check parameters under `with:`. Within an Item every `D:` Prompt runs before any `Q:` Prompt, and a failed `D:` Prompt skips that Item's `Q:` Prompts until it is fixed. A model-only walk over the Registry costs hours and gives unstable verdicts where a structural answer exists (does a step match exactly one catalogue template, is every declared term defined), while judging meaning still needs a model; declaring the deterministic part as a Prompt keeps it visible in the checklist and the report, and lets `--fix` treat its findings like any other.

## Considered Options

- A deterministic item filter hardwired in the command, as `build tests` narrows its walk with step coverage today: cheap, but invisible in the checklist and absent from the report.
- An external command per Cell, its exit code deciding: any host could write checks, at the price of a process per Cell and an argument protocol to design.
- Model-only checks: no parsing, tolerant of paraphrase, but a gate that varies between runs and a re-walk that costs a model turn per Cell.

## Consequences

- A Prompt with both `D:` and `Q:`, or neither, and a `D:` naming no built-in check are startup refusals; `model:` is refused on a `D:` Prompt, which has no validation turn.
- A `D:` Prompt returns the same verdict shape as a model (pass or fail plus context lines), so its `F:` drives the existing fix and clarification loop.
- The engine reads the documents a `D:` Prompt cites as data, the product document included, which until now reached checklists only as prompt text.

Decided in `docs/qna/20261021-112516-self-spec-and-doc-validation-chain.yaml`, Q11, Q12 and the syntax entry Q12 delegated.
