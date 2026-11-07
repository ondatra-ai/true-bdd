# TrueBDD's generation architecture

TrueBDD tests nothing. From a Host project's Specification it generates the Steps package, then the tests that import it, then the code those tests demand. The tests define what the code must be; the code has no authority of its own, and only the Steps package is checked. This is the living picture behind ADR-0040 and ADR-0041; `.claude/rules/generation.md` states the same rules for work on the repository, and [the published page](https://claude.ai/artifact/SYhGb7WK9jaN6Bu2Lex6HH) draws it.

## The direction is reversed

```mermaid
flowchart LR
  subgraph USUAL["Usual"]
    UT["Tests<br/>written afterwards"] -->|check| UC["Code<br/>written first, by people"]
  end
  subgraph TRUEBDD["TrueBDD"]
    SP["Specification<br/>the only thing people write"] -->|build steps| ST["Steps package<br/>generated and checked"]
    ST -->|build tests| TE["Tests<br/>generated, define the code"]
    TE -->|build code| CO["Code<br/>generated to pass the tests"]
  end
```

Tests are not written to check code that already exists. The steps come first, the tests are rendered from the Scenarios on top of them, and the code is generated until those tests pass.

## What generates what, and what is checked

```mermaid
flowchart LR
  subgraph SPEC["Specification: written by people"]
    PD["Product document<br/>concepts, glossary"]
    STO["Stories"]
    REG["Scenario Registry<br/>docs/scenarios.yaml"]
    ARCH["Architecture<br/>services, Public interfaces,<br/>dependencies and what their requests carry,<br/>testing: commands, testkit"]
  end
  subgraph ENGINE["TrueBDD engine: generates, tests nothing"]
    US["us refine, us apply"]
    BS["1. build steps<br/>AI, reads go doc of the runtime"]
    BT["2. build tests<br/>deterministic, one pass"]
    BC["3. build code<br/>red check first when no code,<br/>then AI until green"]
  end
  subgraph GEN["Generated: regenerate at will"]
    STEPS["Steps package<br/>steps/, helpers/<br/>checked"]
    TESTS["Tests<br/>*_test.go, main_test.go<br/>never checked"]
    CODE["Code<br/>services/#lt;service#gt;/"]
  end
  subgraph RT["TrueBDD runtime: the same for every host"]
    TK["pkg/testkit"]
    SDK["pkg/proxy/cli"]
    CP["utilities/cliproxy"]
  end
  REAL["Real CLI or its recorded cassette"]
  STO --> US -->|writes Scenarios| REG
  SPEC -.->|every command reads it| ENGINE
  BS -->|writes| STEPS
  BT -->|writes| TESTS
  BC -->|writes| CODE
  TESTS -->|import| STEPS
  TESTS -->|run and check| CODE
  STEPS -->|import| TK
  STEPS -->|import| SDK
  TESTS -->|the dependency's contract| SDK
  SDK -->|builds, puts on PATH| CP
  CODE -->|calls the CLI| CP
  CP -->|forwards or replays| REAL
  BS -.->|go doc -all| TK
  BS -.->|go doc -all| SDK
```

- `build steps` writes the Steps package, the one generated thing that is checked: Step coverage, compilation, one binding per step, written bodies and their verification, which also reads the helpers.
- `build tests` renders one test per Scenario and the tests' main file, `main_test.go` (`TestMain`, the suite, `New`), in one deterministic pass; the tests are bound to the steps by construction and never checked. Without the Steps package it refuses to start.
- `build code` writes the code. When a service has no code yet, it first runs that service's tests: every one must be red, because the code does not exist; a green test stops it, naming its Scenario. With code present, it goes on to the failing tests.

## Every command is a checklist

```mermaid
flowchart TD
  CMD["Command<br/>decides what each checklist item is asked about:<br/>one Item, an array of Items, or text"] --> NEXT
  NEXT{"Next checklist item,<br/>in the written order"}
  NEXT --> Q{"Q"}
  Q -->|prompt| QP["A model answers"]
  Q -->|code| QC["The engine's code answers"]
  QP --> V{"Verdict"}
  QC --> V
  V -->|pass| NEXT
  V -->|"fail, naming what failed"| FX{"Run with --fix?"}
  FX -->|no| REC["Recorded as failed"] --> NEXT
  FX -->|yes| F["F: fixes only what its Q found,<br/>the person's choice at the fix menu"]
  F -->|refine| F
  F -->|exit| STOP["Run stops"]
  F -->|"cannot fix"| CANT["Run stops with the reason<br/>and what a person must correct"]
  F -->|apply| AP["Fix applied"]
  AP --> SAME{"The same Q runs again"}
  SAME -->|"fail, within max_apply_attempts"| F
  SAME -->|"fail, past max_apply_attempts"| ERR["Run stops with a named error"]
  SAME -->|pass| RESTART["The checklist starts again<br/>from its first checklist item"]
  RESTART --> NEXT
  NEXT -->|"everything walked"| DONE{"Every Q passed?"}
  DONE -->|yes| PASS["Command passes"]
  DONE -->|no| FAIL["Command fails, naming what failed"]
```

Each check a command makes about what it walks is a checklist item: a `Q`, the verification, and its `F`, the fix (ADR-0042). A `Q` is a prompt, which a model answers, or code, which the engine runs, never both; one checklist item answers one question, and a check needing a prompt and code is two checklist items. The engine reorders nothing: where code checks must come first, the checklist is written that way. Without `--fix` the walk goes on to the end and reports every failure. With `--fix` an applied fix is followed by its own `Q` and then the whole checklist again, since an apply may break an item that passed; an `F` fixes only what its Q found, receiving the question, answer, context lines and item's F text. A problem its Q did not find is neither its work nor a reason for cannot-fix. An `F` that cannot fix stops the run with its reason and what a person must correct, without asking for an Earlier step. A person who has not decided is a legitimate cannot-fix. The engine's own contracts (the checks of the host's files at start, the rules of one AI turn, the walk's limits) are not checklist items: a checklist cannot check itself. Work a checklist item can do is written as one, not as engine logic. The YAML form is `Q: {prompt: …}` or `Q: {code: …}`, with parameters under `with:` and optional `F:`; legacy `D:`, scalar `Q:`, both options and neither refuse at startup. A cannot-fix outcome is one optional `cannot_fix` mapping with `reason` and `correction`, shared by proposal, clarification reply and apply. Existing `interface_cannot_serve` and `impossible` wire names remain instances with their established logs and reruns. Some of `build steps`' checks still sit in the engine; they move into checklists ticket by ticket.

An Earlier step is a previous command of the pipeline `us refine`, `us apply`, `build steps`, `build tests`, `build code`; it is a term, not an F instruction.

## The runtime knows no product

```mermaid
flowchart LR
  ARCH["Architecture<br/>dependencies: protocol, scope,<br/>what their requests carry<br/>testing.testkit: repo_layer, fixture_step"]
  MAIN["Tests' main file<br/>build tests"]
  STEPS["Steps package"]
  TK["pkg/testkit<br/>root API"]
  TKI["pkg/testkit/internal<br/>runner, reporter"]
  SDK["pkg/proxy/cli"]
  CP["utilities/cliproxy"]
  CODE["Generated code"]
  REAL["Real CLI or cassette"]
  ARCH -->|the engine copies| MAIN
  MAIN -->|NewSuite options| TK
  MAIN -->|Substitute with the dependency's rules| SDK
  STEPS -->|import| TK
  STEPS -->|import| SDK
  TK -->|uses| TKI
  SDK -->|builds, puts on PATH| CP
  TK -->|builds and runs| CODE
  CODE -->|calls the CLI| CP
  CP -->|record, live, replay| REAL
```

A product's facts flow one way, from its Specification into the runtime. The architecture declares each third-party dependency and what its requests carry; the tests' main file passes that to the SDK, which hands it to `cliproxy`. `pkg/testkit`, `pkg/proxy/cli` and `utilities/cliproxy` hold only what is the same for every Host project; step bodies see the `pkg/testkit` root and the SDK alone, and the AI learns them from `go doc -all`.

Grilling log for the F scope, handover, Earlier step term and pending-decision outcome: `docs/qna/20261105-154500-cannot-fix-at-recording.yaml`, Q3 to Q6.
