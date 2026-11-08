---
paths:
  - "services/true-bdd/**"
  - "pkg/**"
  - "utilities/**"
  - "tests/**"
  - "scripts/**"
  - "docs/**"
  - "templates/**"
  - "true-bdd/**"
---

# Generation

TrueBDD **generates**; it tests nothing. From a Host project's Specification it writes the steps, then the tests, then the code, and the tests define what the code must be. Hold every change to this picture, drawn in `docs/adr/generation-architecture.md` and decided in ADR-0040, ADR-0041 and ADR-0042. Where the code has not caught up yet, move it toward the picture.

1. **People write only the Specification:** the product document, Stories, the Registry and the architecture. A suite's folder but its fixtures, and every service's code, is generated and regenerated at will.
2. **The order is `build steps`, `build tests`, `build code`.** `build steps` writes the Steps package (`steps/`, `helpers/`); `build tests` renders, in one deterministic pass, one test per Scenario and the tests' main file (`main_test.go`: `TestMain`, the suite, `New`), which import it; `build code` writes the code.
3. **The Steps package is the one generated thing that is checked:** Step coverage, compilation, one binding per step, written bodies and their verification. A test is its Scenario's steps stated literally over that package, bound by construction; `build tests` checks only its folder (ADR-0038).
4. **Every test is red until the code is built.** When `build code` starts with no code built, it checks Red without the target first; a green test stops it with a distinct error naming its Scenario, and nothing goes back to `build steps`.
5. **The runtime knows no product.** `pkg/testkit`, `pkg/proxy/cli` and `utilities/cliproxy` hold what is the same for every Host project. A product's facts flow one way, from its Specification into the runtime: the architecture declares a third-party dependency and what its requests carry, the tests' main file passes that on, the SDK hands it to the stand-in.
6. **Steps see the runtime through its declared surface.** Step bodies import the `pkg/testkit` root and `pkg/proxy/cli`, and the AI learns them from `go doc`; helpers live in `tests/<suite>/helpers/`, declared nowhere.
7. **Every command is a checklist.** Each check a command makes about what it walks is a checklist item (always in full: an Item alone is what a command walks): a `Q`, which a model answers from a prompt or the engine's code answers, never both, and its `F`, the fix. One checklist item answers one question, items run in the written order, and the command decides what each is asked about. Work goes checklist first: what a checklist item can do is written as one, not as engine logic; a ticket adding such logic to the engine says why a checklist item cannot do it. The engine's own contracts (start checks, the rules of one AI turn, the walk's limits) stay outside checklists. `Q.code` replaces retired `D:`, and the checks still made outside a checklist move into one, each through its own ticket (`docs/adr/generation-architecture.md`, Every command is a checklist; ADR-0042).
