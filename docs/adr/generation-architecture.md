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

- `build steps` writes the Steps package, the one generated thing that is checked: Step coverage, compilation, one binding per step, written bodies and the critique, which also reads the helpers.
- `build tests` renders one test per Scenario and the tests' main file, `main_test.go` (`TestMain`, the suite, `New`), in one deterministic pass; the tests are bound to the steps by construction and never checked. Without the Steps package it refuses to start.
- `build code` writes the code. When a service has no code yet, it first runs that service's tests: every one must be red, because the code does not exist; a green test stops it, naming its Scenario. With code present, it goes on to the failing tests.

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
