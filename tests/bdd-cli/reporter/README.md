# BDD run reporter

Renders a self-contained HTML report for a BDD fixture session: where a
run's wall clock went, what each AI turn cost, and which slices are the
engine's own code versus a model deciding how long to take.

```bash
# 1. run the suite (or a single fixture) and keep the verbose output
go test -tags bdd ./tests/bdd-cli/... -v -timeout 30m > tmp/bdd-run.log 2>&1

# 2. render the newest session
go run ./tests/bdd-cli/reporter

# -> tmp/bdd-report.html                  the index
# -> tmp/bdd-report-<fixture>.html        one detail page per fixture
```

The **index** answers *where the time went*. Each fixture links to its
own **detail page**, which answers *what happened in each slice*: open a
row for the command that ran, the checklist cell, the prompt sent, the
tool permissions, the response, the tool calls the model made, and the
token/cost breakdown. Expanders are native `<details>` — no JavaScript,
so the pages work offline and print.

No dependencies beyond the Go toolchain already needed to build the
engine. Paths hang off the repo root (found by walking up for `.git`),
so the working directory does not matter.

| Flag | Default | |
|---|---|---|
| `-session` | newest dir under `tmp/test_run/` | the run to report on |
| `-gotest` | `tmp/bdd-run.log` | verbose `go test` output |
| `-out` | `tmp/bdd-report.html` | where to write |

## What it reads

1. **`tmp/test_run/<session>/<fixture>/tmp/true-bdd.log.json`** — the
   engine's own JSON log. Each AI turn leaves `Dispatching AI turn`
   (turn / role / cli / model), `AI turn usage` (claude only: cost and
   token counters) and `AI turn returned` / `AI turn failed`
   (`duration_ms`). The spans *between* those records are the engine's
   deterministic work.
2. **The `go test -v` output** — two things live only here: the
   per-fixture wall clock and verdict, and the harness judge's own slog
   records. The judge runs in the test process, so its cost never
   reaches the engine log.
3. **The fixture's prompt artifacts** under `tmp/<run-id>/` — the system
   and user prompts, responses, result YAML and CLI transcripts. Matched
   to turns positionally: a prompt artifact written before a dispatch is
   that turn's input, anything written after a completion is its output.
   Their filenames also carry the checklist cell.
4. **The fixture manifest**, via `runner.LoadFixture` — the `cmd:`,
   `prep:`, `teardown:`, `answers:` and expectations. Read through the
   runner rather than re-parsed, so the reporter cannot drift from the
   harness's own view of a manifest.

## Commands

Every subprocess the run spawned is shown as a copyable command line:

| Command | Source |
|---|---|
| harness → CLI | the fixture manifest's `cmd:`, in the run's tmpdir |
| engine → test runner | `Spawning test runner` (`testrunner/spawn_log.go`) |
| engine → crush / codex | `Spawning agent CLI` (`adapters/ai/cli_invocation.go`) |
| engine → claude | `Spawning agent CLI` (`claudecode/internal/subprocess`) |

The claude record redacts `--system-prompt` to `<N bytes>` — the prompt
itself is archived as an artifact and shown in full on the page.

Logs written before those records existed still get a claude command,
**reconstructed** from the model, tool lists and permission mode the
engine did log, and labelled as such wherever it appears. The
reconstruction mirrors `BuildCommand` in
`src/claudecode/internal/cli/discovery.go`; `invocation_test.go` pins the
flag order against it.

## The accounting contract

The report's spine is a gap-free timeline: every slice is tagged
deterministic or non-deterministic, and the slices must sum to the wall
clock `go test` measured. The command prints the drift per fixture — if
that number is not near zero, the phase model has a hole and the report
is lying:

```
build-code-playwright-nextjs: wall 205.88s, phases 205.88s, drift 0.003s
```

`TestPhasesSumToWallClock` and `TestPhasesAreContiguous` pin that
invariant against a synthetic run, so a change to the phase model that
loses or double-counts time fails in `go test ./...` rather than in a
report someone is reading.

Exactly one slice is a residual rather than a measurement: *fixture
prep*, because `go test` reports a subtest's total but never stamps when
it began. It is labelled as such in the output.

## Layout

| File | |
|---|---|
| `main.go` | flags, repo-root anchoring, drift self-check, page writing |
| `engine_log.go` | JSON log parsing, folding records into turns |
| `turn.go` | one AI turn, its token counters and time split |
| `artifact.go` | artifact ↔ turn matching, checklist-cell parsing |
| `invocation.go` | subprocess commands, logged or reconstructed |
| `toolcall.go` | the tool calls a turn made |
| `run_metadata.go` | checklist/architecture load, fix-loop decisions |
| `manifest.go` | the fixture manifest, via `runner.LoadFixture` |
| `gotest_log.go` | wall clocks, verdicts, and the judge's usage |
| `fixture.go` | assembling one fixture from every source |
| `phase.go` | the timeline: contiguous slices, tagged and owned |
| `renderer.go` | index assembly, palette, shared helpers |
| `section_*.go` | one file per index section |
| `detail_page.go`, `detail_blocks.go`, `section_detail_turn.go` | the per-fixture page |
| `head.html`, `caveats.html` | embedded page shell and prose |

## Dependency

Needs the per-turn telemetry that `src/adapters/ai/router.go` and
`src/adapters/ai/claude_provider.go` emit (`duration_ms`, `role`, and the
`AI turn usage` record). Against an engine log without those fields the
report still renders, but durations and costs come out empty.
