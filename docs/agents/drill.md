# Rebuild drill

Run `go run ./scripts/cmd/drill start --base <sha>` from the checkout. It logs a private iteration directory outside the checkout. Keep that path: every following command takes `--iteration <dir>`. The iteration's `record.json` records the immutable base and trimmed binary's SHA-256, allowed audit roots, cuts, runs, questions, oracle results and audit history. It is replaced atomically. Cut directories are unrelated private temporary directories with anonymous names; they contain one new Git commit and no base history.

```text
go run ./scripts/cmd/drill cut a --iteration <dir>
go run ./scripts/cmd/drill run a a-1 --iteration <dir> -- build code --fix
go run ./scripts/cmd/drill stop a-1 --iteration <dir>
go run ./scripts/cmd/drill oracle a --iteration <dir>
go run ./scripts/cmd/drill audit a --iteration <dir>
```

Cuts B, C and D use this pipeline, one command at a time, with an audit and an operator's judgment before continuing:

```text
go run ./scripts/cmd/drill cut b --iteration <dir>
go run ./scripts/cmd/drill run b b-tests --iteration <dir> -- build tests --fix
go run ./scripts/cmd/drill run b b-steps --iteration <dir> -- build steps --fix
go run ./scripts/cmd/drill run b b-code --iteration <dir> -- build code --fix
```

`build tests --fix` renders test files deterministically, without AI or a test run. `build steps --fix` writes and verifies the step definitions, including Red without the target; `build code --fix` builds the service to turn the tests green (ADR-0031). The old engine built at the base drives all three commands, but is no test target. The prebuilt oracle flag remains available only for the independent oracle below.

Every cut keeps `docs/product/`, `docs/architecture/architecture.yaml`, `docs/scenarios.yaml`, `true-bdd/`, `templates/`, `pkg/`, `scripts/`, `go.mod`, `go.sum`, `.gitignore` and the portions of `tests/true-bdd/` its layers do not delete. It also keeps each exact interface document named by a service's `public_interface.document` in the base's architecture, relative to the Host project root, wherever the document lives, including inside a deleted layer. Naming one file keeps neither its siblings nor its directory. UI declarations have inline elements and add no document. The architecture is read from the base's archive, so later checkout changes cannot change a cut's kept documents. So every cut keeps what concepts need: the product document's `concepts:` and glossary, the architecture's concept bindings and Word mappings, and the Public interface documents the bindings reference.

`run` accepts exactly `-- build tests --fix`, `-- build steps --fix` or `-- build code --fix`, optionally followed by repeated `--model <tier>=<cli>:<model>` pairs; every other command line is refused. Each maps to its own typed engine operation. The engine validates model overrides; deterministic `build tests` does not support them. `run` and `oracle` return after launching a detached worker. Read the run's log and state under `record.json`; the worker keeps the three-hour limit, including clarification pauses, without a terminal or launcher. Only one unfinished run is permitted per iteration. Tags cannot be reused. Each rerun remains an operator decision. The fix menu receives `1`; a clarification receives nothing automatically.

The drill recognizes the fix menu by its three option lines in order at the end of the engine output so far, and a clarification by suggested answers numbered from 1, each starting a line, then a line starting with `0) Other` at the end of the output so far. Only whitespace or the original engine's optional `Your answer:` prompt may follow the clarification. Both forms wait for 50 milliseconds without output before recognition, so a quoted form followed by more output across chunks does not trigger an answer or pause. The original engine's line-starting `Your choice (1/2/3):` prompt is also accepted; a prompt following an already recognized form does not trigger a second response. A clarification does not require the original engine's `[current/total]` question marker.

A paused run records the printed question, options and a private answer filename. If the engine exits while waiting, the wait ends immediately, the question stays unanswered in the record, and terminal cleanup runs. Publish a JSON object at that filename with one line of answer and the exact Specification citation:

```json
{
  "answer": "The answer in your own words, or the chosen option number",
  "document": "docs/product/product.yaml",
  "passage": "An exact passage from that document supporting the answer"
}
```

Write the file through a temporary file and rename to avoid presenting incomplete JSON. Sources are confined to `docs/product/`, `docs/architecture/architecture.yaml`, `docs/scenarios.yaml` and the kept interface documents named by the architecture; the passage must occur in the named document. Symlinks resolving outside these Specification documents are refused. The operator remains responsible for whether it supports the answer. When the Specification cannot answer, record the question as the blocker and use `stop`; never restore or copy deleted code. Follow issue #212's stop-at-the-first-blocker rule; the tool does not decide whether progress justifies another command.

Every terminal path stops the engine's process group and discovers and stops every process with the exact `TRUE_BDD_DRILL_RUN` environment entry, including detached children. Argument text cannot claim ownership. An end timestamp and exit code are written only after cleanup verification. A discovery or cleanup failure produces `cleanup_failed`, retaining an unfinished record for a later `stop`. The worker has a separate identity so it can finish the record after killing the engine's descendants.

The oracle builds the engine the cut produced, exports a fresh cut-A tree from the base, and runs its original suite with `go test -tags bdd -json -count=1 -timeout=40m ./tests/true-bdd/ -mode=services:replay,tests:replay -engine-bin=<absolute path>`. The shared external deadline covers preparation, build and replay. The record counts Registry Scenarios and lists the first ten failures; suite guards do not increase the Scenario count.

The audit reads the cut's `tmp/true-bdd.log.json` and `tmp/**/*-codex-events.jsonl`, the writer's `--json` events. It names every evidence file in its log and saves those paths in `record.json`. Engine tool-call records retain explicit allowed/denied decisions; Codex command and file-change events retain their raw details with an unknown access decision, since an exit code or completion does not establish a successful read. It groups references inside the cut, in Go's installation/caches or CLI configuration roots, outside those roots, or on the network. Allowed roots classify evidence; they grant no reads. Network tool references remain visible. Unreadable or malformed logs and ambiguous paths are reported as limitations. Oracle replay is explicitly excluded: its logs carry recording-time paths and it runs no live AI turns. A report is evidence for the operator, never a contamination verdict.

Issue #187 is the product prerequisite for local-read isolation; this tool supplies no substitute sandbox. Do not begin a real iteration until that prerequisite holds. No live AI CLI or real engine run is part of the tool's offline test suite.
