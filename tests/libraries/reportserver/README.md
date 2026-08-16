# BDD run report server

```bash
go run ./tests/bdd-cli/cmd/report-server        # http://127.0.0.1:7331
```

Flags: `-addr` (default `127.0.0.1:7331`), `-runs` (default `tmp/test_run`),
`-interval` (default `15s`).

Loopback only on purpose: responses carry whole prompt bodies and project source.

## What it replaces

A static generator that rendered **one** session to `tmp/test_report/*.html` after every
fixture. That could not show more than one run, could not compare two, and never showed what
a fixture *expected* next to what it got. This serves every session from memory instead.

## Views

| Route | Shows |
|---|---|
| `#/` | every run, newest first; pick two and Compare |
| `#/run/<id>` | that run's fixtures, roll-up tiles; `←`/`→` or `[`/`]` step between runs |
| `#/run/<id>/test/<name>` | expected vs actual, phase timeline, turns, files, evidence |
| `#/cmp/<a>/<b>` | test by test: verdict, wall, turns, cost, what changed |
| `#/cmp/<a>/<b>/test/<name>` | one test turn by turn, plus expected / judge / file diffs |

## Design

**The seal.** `bdd-cli-logs/harness.json` is the last byte ever written into a fixture
directory — `HarnessRecorder.Finish` is registered first in the subtest, so `t.Cleanup`'s LIFO
runs it last. Its presence therefore *proves* the fixture can never change, which is an exact
cache key rather than an mtime heuristic. Only the fixture currently in flight is re-parsed on
a rescan; the other 84 are handed back by pointer.

**Pull, not tick.** A read older than the interval does the scan itself. A background ticker
would burn CPU for the life of the process even with no browser open, and need shutdown
plumbing to stop it.

**Atomic publish.** Snapshots are immutable and swapped through `atomic.Pointer`. A reader
takes the pointer once and holds it for the whole request, so a refresh mid-request can never
tear a response — and no reader ever blocks on a lock.

**Version, not timestamp.** `/api/state` bumps `version` only when content actually changed.
A version that ticked on every scan would make the UI throw away scroll position and close
expanders for nothing.

**Real diffs.** `znkr.io/diff` (Myers) everywhere, differing only in the equality key:

| What | Key |
|---|---|
| tests across runs | fixture name |
| turns within a test | `(cell section, cell subject, role)` — **not** the turn number |
| changed files | path, with per-run scratch dirs normalised away |
| prompt / judge text | lines, via `textdiff.Hunks` + `IndentHeuristic` |

Turn alignment is the one that earns its keep. A cell is retried until it passes, so
`us-create-happy-path` is nine turns over four cells (`format`×2, `who`, `what`×2, `why`×4).
Keying on turn number *is* ordinal pairing: one extra retry then shifts every later turn and
reports all of them as changed. Keying on the cell yields a single insertion.

File paths get normalised because every artifact lives under `tmp/<timestamp-pid>/`. Compared
raw, no artifact ever matches across runs and the diff degenerates into "everything deleted,
everything added" — true and useless.

Diff output is asserted by **invariant, never golden**: the library documents its output as
unstable across minor versions.

## Layering

`reporter/` parses and nothing else — it owns `Fixture`/`Turn`/`Phase` and the phase-timeline
invariants. `reportserver/` owns the cache, the wire format, the diffs and the UI.

DTOs are an explicit layer (`wire.go`), not tags on the loaders, because `Phase` holds a
`*Turn` back-pointer that would emit every turn twice, `Artifact` holds whole 70KB bodies that
must never ride along in a list, and `time.Duration` marshals as nanoseconds. Tagging the
loaders would also weld their field names to the wire format in files pinned by invariant tests.

## Caveats

- Sessions recorded before harness schema 2 have no judge transcript and no manifest snapshot.
  The UI says so rather than rendering an empty diff, which would read as "identical".
- With a `repo`-sourced manifest, "expected" is today's `fixture.yaml`, not what that run was
  held to. Comparison marks itself not `comparable` in that case.
- Artifact bodies are fetched by opaque ref resolved through a map lookup — never by joining a
  client string onto a path, so there is no traversal surface.
