# The architecture declares what a build-code writer may change in the project and which libraries it may read

A build-code writer's write roots already came from the architecture: each service's `path`. Its library reads did not. A sandbox rule in code denied the whole `github.com/ondatra-ai` organisation in the module cache and allowed everything else. In the release emulation, the writer could neither repair a root `go.sum` nor read the test library its host's tests required.

We decided that the architecture is enough to build the system, and that a writer finds its solution within what the architecture declares:

- **Project writes.** A writer changes its service's `path` and the exact paths in the service's `writable`, with no patterns. It changes the tests and step definitions never, even where a path or `writable` entry would reach them: build code only reads and runs them.
- **Library reads.** The service and the `testing` section each name their `libraries` with exact versions. Before the writer turn, the run downloads them and what they need. The writer then reads exactly those from the module cache, with no network.
- **Impossible work fails.** When the work is truly impossible within the declaration, the run fails at once with an error naming what it would need. A library that cannot be downloaded stops the run before the writer turn.
- **The engine's own scratch.** The host's tmp, the toolchain and the operating-system runtime stay the engine's operational grants, outside this rule.

No rule names TrueBDD's own module. The drill's isolation follows from this repository's architecture naming only third-party libraries.

## Considered Options

- **Grant language manifests implicitly** (`go.mod`, `package.json`). It is convenient, but it gives rights the Specification does not state.
- **A special grant and deny for TrueBDD's test library.** Rejected: whatever the file names is allowed, and a person controls the file.
- **Library names without versions, with proxy access for the writer.** A rebuild would then depend on what the proxy serves that day, and the writer would need network.

## Consequences

The Claude writer's replay command runs the service's and the tests' code unconfined, with network (ADR-0022), so it enforces this rule only through its file tools. The Owner kept it so, as a known limit: only the codex writer, the default, enforces the rule fully, and the Claude writer stays for model trials.

Grilling log: `docs/qna/20261026-175738-triage-215-216-221.yaml`, Q1, Q2, Q3, Q6, Q7, Q8, Q9, Q14, Q15, and the corrections entry.
