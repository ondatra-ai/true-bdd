# The engine is its own Go module and uses no code of this repository

The engine imported this repository's `pkg/`: about 3,200 lines of process, file, git and CLI code. A rebuild from the Specification never wrote that layer, and #181 accepted this as a leak. The engine also shared the root `go.mod` with the tests and the tooling.

These are two separate problems, and we decided both:

- **Code.** The engine imports nothing of this repository outside its folder. It builds only with libraries the architecture names with exact versions. A rebuild writes the whole engine, that layer included.
- **Manifest.** The repository becomes three Go modules:
  - the engine, `services/bdd-cli`;
  - `pkg/`, the shared libraries and the test library;
  - the root, holding the tests and the scripts.

  The engine's `go.mod` lives in its folder. A cut deletes it, and the rebuild writes it from the named libraries.

The code change lands first, so the engine module requires nothing of this repository.

## Considered Options

- **Keep one module.** It is cheapest, but the engine's manifest stays shared with the tooling.
- **Four modules, one per Go root.** A module for the tests adds no benefit.
- **Keep the engine on `pkg/`, listed as a library.** It is simpler, but a rebuild never shows that the Specification alone yields the engine.
- **Hide the test library from the writer.** Go builds tests from source, and the writer must run the tests, so that code cannot be hidden. The engine's own code is written separately instead.

## Consequences

- **Scenarios first.** The behaviour the engine took from `pkg/` must be pinned by Scenarios first, or a rebuild drops it.
- **The log contract.** Log message names are no longer shared code. The Specification and the Scenarios hold that contract.
- **Recordings.** In an experiment, 40 of 256 Scenarios failed as stale cassettes, because recorded requests embed module identities. Three more failed on workspace module selection. Re-record what still changes after the workspace is handled.
- **Tags.** Go requires the nested module's tags to carry its directory, `pkg/vX.Y.Z`. Whether releases tag both together is decided separately.

Grilling log: `docs/qna/20261026-175738-triage-215-216-221.yaml`, Q3, Q4, Q5, both Q6 entries, Q13 and Q14.
