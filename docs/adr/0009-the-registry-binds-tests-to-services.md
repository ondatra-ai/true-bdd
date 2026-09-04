# The registry binds a test to its service; `architecture.yaml` declares no suites

`architecture.testing.suites[]` is gone. `testing:` now holds two keys —
`framework:` and `commands:` — and says how this repository runs its tests, once.

A suite used to carry six fields. Every one of them was either already stated
elsewhere or was not read:

- `service:` and `path:` are on the scenario, and always were
  (`docs/scenarios.yaml`, `service:` and `path:` per entry). The suite repeated
  them at a coarser grain.
- `framework:` is a property of the repository, not of a suite. Two suites
  declaring different frameworks was expressible and meaningless: the engine
  renders one generator.
- `helpers:` was never decoded. It reached prompts as prose and nothing else.
- `pattern:` was decoded and passed to a runner that ignores it.
- `config:` was the one field that carried real weight, and it survives —
  moved to `testing:`, one per repository. See below.

## What replaces the lookups

Everything the engine did by finding a suite it now does from the scenario:

- The owning service is `scenario.Service`, checked against `services[]`.
- The file is `scenario.Path`. All of a service's scenarios must share one
  directory — whichever the host chose; `tests/mcp/` for a service named
  `mcp-service` is a naming decision, not a defect. That is precisely what the
  suite's `path:` used to pin. The generator derives the package clause and the
  scenarios import from that directory exactly as it derived them from the
  suite, so regeneration is byte-identical.
- The test binary asks `LoadSuiteSpec` for a *service* name and gets its tree.

## What this cost

Two refusals changed hands rather than disappearing. A scenario naming an
undeclared service was refused by the loader against a suite's `service:`; it is
now refused by the generator against the scenario's own. That is a better place
for it — the loader never knew which scenarios existed.

One refusal is gone outright, and deliberately: two scenarios sharing a file
while naming different services. A file has one directory and a directory has
one service, so it is unreachable, and it was deleted rather than kept as
decoration. The scenario that pinned it (E2E-013) and its fixture went with it;
the generator's unit tests cover the surviving refusals.

## What a host loses, and what it nearly lost

Per-suite `helpers:` and `pattern:`, both already inert.

`config:` was nearly lost with them. The first draft removed it on the reasoning
that a host could encode its working directory in the command string. The
`build-code-playwright-nextjs` fixture refuted that within one recording run:
playwright resolves `testDir` relative to its config file, so a command started
from the repository root discovered nothing and wrote a zero-byte report. The
field is restored on `testing:` — one working directory per repository rather
than one per suite — and the fixture is the reason it is documented here as
load-bearing rather than as legacy.

## Why not one suite instead of none

A single suite entry would have kept the shape while removing the plural. It
would also have kept `service:` — one name for a document that describes two
services' tests — and that is precisely the field that made the shape wrong. A
suite naming one service cannot own the other's tests, and fix prompts aimed at
it would point at the wrong root while the write roots permitted the edit. The
honest options were one suite per service, which is what existed, or none.
