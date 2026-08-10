# BDD run reporter

Renders a self-contained HTML report for a BDD fixture session: where a
run's wall clock went, what each AI turn cost, and which slices are the
engine's own code versus a model deciding how long to take.

The BDD suite renders it **in-process, after every fixture** — no redirect,
no second command, and a report exists while a long run is still going:

```bash
go test -tags bdd -timeout=180m ./tests/bdd-cli/...

# -> tmp/test_report/index.html        the index
# -> tmp/test_report/<fixture>.html    one detail page per fixture
```

To re-render a session by hand — an older one, or one whose suite was
killed:

```bash
go run ./tests/bdd-cli/cmd/reporter
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
| `-gotest` | none | legacy `go test -v` log, for sessions with no harness record |
| `-out-dir` | `tmp/test_report` | directory for `index.html` and the per-fixture pages |

## What it reads

1. **`tmp/test_run/<session>/<fixture>/tmp/true-bdd.log.json`** — the
   engine's own JSON log. Each AI turn leaves `Dispatching AI turn`
   (turn / role / cli / model), `AI turn usage` (claude only: cost and
   token counters) and `AI turn returned` / `AI turn failed`
   (`duration_ms`). The spans *between* those records are the engine's
   deterministic work.
2. **`tmp/test_run/<session>/<fixture>/bdd-cli-logs/harness.json`** — the
   harness's own record of the run. Four things live only here, because
   the engine cannot see them from inside its own process: the fixture's
   wall clock, its verdict, the file diff, and what the harness judge
   spent. Written from a `t.Cleanup`, so it survives a `t.Fatalf` — and
   strictly after `Execute` took both snapshots, so it can never enter
   the diff the judge grades.

   The judge's cost is billed by **time window**: `runner.Verdict`
   stamps the model call, and the sink in `runner/harness_logging.go`
   claims the `AI turn usage` records that fall inside it. A fixture
   that died before reaching its judge therefore claims nothing, instead
   of inheriting the next fixture's cost.

   A session recorded before this file existed can still be rendered by
   passing `-gotest` the old redirected log. Opt-in on purpose: the
   harness now configures `slog`, which changes the format those judge
   records were scraped from, so a legacy log left active by default
   would quietly drop the judge's cost while still producing a
   complete-looking timeline.
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
clock the harness measured. The command prints the drift per fixture — if
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
prep*, because the harness records a fixture's total but the engine
never stamps when the prep before it began. It is labelled as such in the output.

Framework runs get their own slices wherever they happened, not just at
discovery. In a `--fix` run the engine's `PostFix` hook re-executes the
test after every applied fix — that is what decides whether the fix
worked — and for a webServer-startup subject it is a whole
docker-build-and-run suite. Those runs are placed from the
`Test runner returned` record's timestamp and its reported wall clock,
so the gap between two turns splits into the engine's own bookkeeping
plus a `Test run (<framework> · <phase>)` slice owned by the tests. A
run the log cannot place — no exit timestamp — leaves its gap
undivided rather than being given an invented position.

## Layout

| File | |
|---|---|
| `cmd/reporter/main.go` | flags and printing, for the manual re-render |
| `engine_log.go` | JSON log parsing, folding records into turns |
| `turn.go` | one AI turn, its token counters and time split |
| `artifact.go` | artifact ↔ turn matching, checklist-cell parsing |
| `invocation.go` | subprocess commands, logged or reconstructed |
| `toolcall.go` | the tool calls a turn made |
| `run_metadata.go` | checklist/architecture load, fix-loop decisions |
| `manifest.go` | the fixture manifest, via `runner.LoadFixture` |
| `report.go` | `Render`, its options and its summary — the only way in |
| `pages.go` | the flat page set, and pruning what a previous session left |
| `harness_record.go` | the harness's record of a run, and the legacy fallback |
| `gotest_log.go` | the legacy `go test -v` scrape, kept for old sessions |
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
