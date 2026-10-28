# Tests are written red, without the target: `build tests` renders them, `build steps` writes the step definitions

Since #173, `build tests --fix` ran every step definition it wrote against the target, and counted every failure as a wrong definition. #192 showed that this loses a new behaviour: a test is red because its behaviour is not built yet, and the Fix loop bends the definition until it passes. In the live run of 28 October the definition halved the number itself, and `build code` had nothing to build.

We decided that **tests turn green only in `build code`**. `build code` builds the code so that it passes every test and nothing more. "From scratch" is the rule for its result, not the order of its work: it may start from existing code and repair only the failing tests. The goal is that a Host project keeps no code in its repository, so no build may count on finding code from the one before.

So **writing the tests never relies on the target's code**, and its result is the same on an empty tree and on a built one. Every test is red until `build code` runs. The work splits into two commands, run by the System Architect in the order `us refine`, `us apply`, `build tests`, `build steps`, `build code`:

- **`build tests` is deterministic.** It renders the test files from the Registry, and starts no test runner and no AI turn: when the Scenario and the step definitions are right, the rendered test cannot be wrong.
- **`build steps` writes the step definitions**, the hard part, which carries each Scenario's logic. It checks:
  - **Covered:** every step binds to exactly one step definition.
  - **Listed:** the test runner lists every Scenario's generated test. This needs the step definitions to compile, so it is `build steps`' check and not `build tests'`.
  - **Red without the target:** one run of the suite with the target absent, where every Scenario must be red. A Scenario that is green without its target has a definition doing the product's work, and goes to the Fix loop.
  - **Critique:** a separate AI turn reads each new definition against the declared Public interface, the catalogue phrase's meaning and the Word mapping. It checks that the definition reaches the service from outside, through the declared interface, and asserts what the phrase means.

**What they may change.** Both write only the tests: the paths the architecture protects from `build code` (ADR-0028). Neither writes a service's code or the Specification. When the declared Public interface cannot set up or observe what a phrase means, `build steps` stops and names the phrase and what the interface lacks.

No marker says whether a Scenario's behaviour is built: no Registry field, no Story status, no build's VCS stamp. The verdict depends only on the Specification and the tests.

## Considered Options

- **Accept a red at a `Then` for a Scenario marked "not built yet"** (#192): with a baseline run, a Registry field, the Registry the running engine was built from, or the Story's status. Each needs an extra parameter, or a history the next build may not have.
- **Judge each verification afresh against the target's code.** A build cannot count on that code being there.
- **No run at all, only the list and coverage.** The live run's bent definition passes both.
- **One command for rendering and step definitions, as before, or `build steps` as a stage inside `build tests`.** The deterministic part needs no AI and cannot be wrong, so all the uncertainty sits in one command of its own.

## Consequences

- #173's run against the target, #192's verdict rule and `testing.prebuilt_engine_arg` (#189), whose only user was that run, are replaced.
- A wrong assertion that only a run against built code would catch is now the critique pass's job, and the drill's oracle stays the final check.
- The harness must report each Scenario on its own when the target is absent, not one failure of the whole suite's bring-up.

Grilling log: `docs/qna/20261028-094259-build-tests-new-behaviour.yaml`, Q6, Q7, Q8, Q9, Q10, Q11, Q18, the correction entry after Q8, and the forced entries after Q8 and Q9.
