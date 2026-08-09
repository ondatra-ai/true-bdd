# BDD run report

`bdd_report.py` renders a self-contained HTML report for a BDD fixture
session: where a run's wall clock went, what each AI turn cost, and which
slices are the engine's own code versus a model deciding how long to take.

```bash
# 1. run the suite (or a single fixture) and keep the verbose output
go test -tags bdd ./tests/bdd-cli/... -v -timeout 30m > tmp/bdd-run.log 2>&1

# 2. render the newest session
python3 tests/bdd-cli/report/bdd_report.py

# -> tmp/bdd-report.html
```

Python 3.9+, no third-party packages. Every default path hangs off the
repo root resolved from the script's own location, so the working
directory does not matter.

| Flag | Default | |
|---|---|---|
| `--session` | newest dir under `tmp/test_run/` | the run to report on |
| `--gotest` | `tmp/bdd-run.log` | verbose `go test` output |
| `--out` | `tmp/bdd-report.html` | where to write |

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
3. **The fixture's prompt artifacts** under `tmp/<run-id>/` — used to
   detect a subject carrying the runner's `<startup>` marker, and fix
   prompts whose failure block came through empty.

## The accounting contract

The report's spine is a gap-free timeline: every slice is tagged
deterministic or non-deterministic, and the slices must sum to the wall
clock `go test` measured. The script prints the drift per fixture on
stderr — if that number is not near zero, the phase model has a hole and
the report is lying:

```
build-code-playwright-nextjs: wall 205.88s, phases 205.88s, drift 0.003s
```

Exactly one slice is a residual rather than a measurement: *fixture prep*,
because `go test` reports a subtest's total but never stamps when it
began. It is labelled as such in the output.

## Dependency

Needs the per-turn telemetry that `src/adapters/ai/router.go` and
`src/adapters/ai/claude_provider.go` emit (`duration_ms`, `role`, and the
`AI turn usage` record). Against an engine log without those fields the
report still renders, but durations and costs come out empty.
