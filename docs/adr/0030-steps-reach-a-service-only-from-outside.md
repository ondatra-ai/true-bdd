# Step definitions reach a service only from outside, through the Public interface the architecture declares

In #192's live run on 28 October, a new behaviour was lost in `build tests --fix`. The step definition imported the calculator's Go package and called `calc.Halve`, which did not exist yet, so the steps package did not compile. The fix plan then made the definition halve the number itself, because "the spec names no production function for halving". The tests went green, and `build code` had nothing to build.

We decided:

- **Three kinds of interface, each with an owner.**
  - A service's **Public interface** is declared in the architecture document, per service, in the standard form of its kind: it names an OpenAPI file for HTTP and an OpenCLI-like file for a CLI. The files it names are part of the Specification. A build never chooses it, and every rebuild keeps it.
  - The **Persistent data model**, the shape of what a service stores between runs, is declared in the architecture and kept by every rebuild, including any code that defines it.
  - Everything else is **Internal code**, which a build chooses freely.
- **Step definitions look from outside.** A step definition sets up and observes a service only through its Public interface, across a process boundary. The tests and the target share no code and no package, and a stub the tests need lives in the tests' own code.
- **Stored data is an open exception.** A step definition may also set up or check the Persistent data model. Where code defines that model, the Owner wants that code fixed and checked as it is, probably by another approach. How a step reaches it, through the store or through that code, is not decided; it waits for a host with stored data.
- **A library is not a BDD target.** Every service the Registry tests offers its Public interface across a process boundary. So a steps package never compiles against its target, and a behaviour that is not built yet shows only at run time.
- **The test comes first.** The test and its step definitions are written against the declared interface before `build code` builds the behaviour, and they may stay red until then (ADR-0031). The engine generates nothing into the target.

## Considered Options

- **The test chooses the API (classic TDD).** The contract would not be in the Specification, so a rebuild could choose another one.
- **`build code` first.** No failing test would drive the new behaviour.
- **The engine generates a "not built" stub of the interface into the service.** It keeps a compiled suite building, but puts generated code into the target.
- **A library as an exception, or behind a driver the tests own.** Either way the tests share a package with the target, and missing symbols become compile errors again.

## Consequences

- Every fixture whose Registry-rendered step definitions import a service's package must take a process-boundary shape: on 28 October `build-code-spec-scenario`, `build-code-spec-missing-product`, `build-code-spec-missing-registry` and `fix-loop-menu-eof`. The Jest and Playwright fixtures keep theirs: their tests are written by hand for `build code`, not rendered from a Registry, and `build code` repairs failing tests however they appeared.
- TrueBDD's own Public interface is scattered today: its commands and outputs are in the product glossary, its template inputs in `architecture.prompt_data`, its calls to the AI CLIs under its dependencies. It is to be gathered into this form.
- How a test that is red because its behaviour is not built is told from a wrong definition is ADR-0031's.

Grilling log: `docs/qna/20261028-094259-build-tests-new-behaviour.yaml`, Q1, Q2, Q3, Q4, Q5, the correction entry after Q16 and the correction entry after the Owner's confirmation, which withdraws the forced entry after Q5.
