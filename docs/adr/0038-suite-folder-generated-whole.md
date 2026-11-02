# A suite's folder is generated whole: its harness lives in the test library, `build steps` writes its wiring, and `build tests` checks it with its fixtures

Issue #249 found that cut B of #276's drill cannot compile TrueBDD's suite: the cut deletes `tests/true-bdd/steps/` and keeps `tests/true-bdd/scenarios/scenarios.go`, which needs `steps.Harness`, `Options`, `FixtureName` and `NewState`, and no command writes them. Looking closer, about 1,800 lines of the suite's folder were never written by TrueBDD: the wiring in `scenarios.go` (463 lines), the harness and utilities in `steps/` outside its definitions (about 1,000), the guard checks in `coverage_test.go` (385) and `main_test.go`. On 2 November a cut B that deleted the whole folder but its fixtures ran `build tests --fix` (33 test files, under a second, no AI) and `build steps --fix` (42 model turns: grouping, 19 Items, the binding judgements and the pattern proposals passed, 66 skeleton files) and stopped at compilation, because no command writes the `scenarios` package the generated tests import.

We decided:

- **A suite's folder is generated whole.** Everything in `tests/<suite>/` but its fixtures is generated from the Specification, and whatever TrueBDD did not write there may be overwritten. Anything else the tests use is a library the architecture declares in `testing.libraries` (GLOSSARY, Library).
- **The harness lives in the test library.** Every harness mechanism (modes, record and replay of the AI CLIs, cassettes, golden comparison, the judge, run records, bring-up) moves into `pkg/testkit` behind an API. The architecture declares that library and states the suite's own choices; the generated suite only wires it.
- **`build steps` writes the wiring, with no AI.** Its engine writes the suite's `scenarios` package and the file holding `TestMain` together with the skeletons, `State` and `Register`, which stays its own (one line per skeleton). `build tests` renders only the Scenarios' test files and still reads no step definition.
- **`State` always holds the library's per-Scenario world** in one field the skeleton writes and the wiring fills; the writer adds the definitions' own typed fields beside it.
- **The writer also writes the utilities the definitions share.** The shape check admits them, and the critique and the red-without-target check cover them as they cover bodies, since a utility can do the product's work as a body can.
- **`build tests` checks the suite's folder.** Without `--fix` it refuses when the test files disagree with the Registry, a Scenario names a fixture that does not exist, a fixture is named by no Scenario, a fixture's copy of a shipped template departs from it, or a comment in a fixture's documents names a Scenario beside an outcome. With `--fix` it rewrites stale test files and still refuses a fixture problem, which no rendering fixes. A gate runs it in place of the hand-written guard tests.

Why the four checks are in `build tests` (the Owner asked to keep this):

- they guard the folder `build tests` already owns; the fixtures are test-bench data, not the Specification `scen check` holds the Scenarios to;
- all four are deterministic and need neither a model nor a test run, as `build tests`;
- the gate already runs `build tests`, so CI keeps them with no new command and no new gate, and the check of the generated tests is the generator checking its own output;
- a check of the whole folder, such as a fixture no Scenario names, has no Scenario to answer for in `scen check`'s per-Scenario cells.

This amends ADR-0035, whose writer changed only bodies, imports and `State`, and ADR-0031, whose `build tests` only rendered. The drill's cut B now deletes the whole suite folder but its fixtures.

## Considered Options

- **Move the glue into a hand-written harness kept beside the steps** (the first answer, withdrawn): it leaves code in the suite that the Specification does not produce.
- **Let the writer add any declaration the kept tests reference:** the model would invent the harness in a cut.
- **State the harness's behaviour in the Specification:** the architecture would turn into a description of the test bench, which no Story concerns.
- **`build tests` renders the wiring:** it runs before `build steps` and reads no definitions, while the wiring names `State` and `Register`, which `build steps` writes.
- **The checks as library calls from the wiring, as repository gates, or as `D:` checks in `scen check`:** gates keep them outside the engine, and `scen check` would need a deterministic-only walk for CI and checks the Specification, not test-bench data.
- **A separate command for the checks:** a new Public interface item and gate for a folder `build tests` already owns.

## Consequences

- A Scenario naming a fixture that does not exist yet stops `build tests`, so a fixture is made before `build tests` runs; in cut D, which deletes every fixture, `build tests` refuses at once.
- The template and comment checks become product behaviour for every host, so their rules are stated in general terms, not as this repository's list of exceptions.
- Still open in the grilling: which packages of `pkg/` the tests may use, whether `true-bdd tests run` belongs to this change, the type of `State`'s library field, and how the work is cut into tickets.

Grilling log: `docs/qna/20261101-215343-steps-package-harness-glue.yaml`, Q3, Q4, Q6, Q7, Q8 and its continuation, Q10, Q11, Q13, and the entries Claude settled as forced consequences (the utilities, the `TestMain` file).
