# The BDD harness is a `pkg/` package, and the only one allowed to cross roots

ADR 0003 admitted `pkg/` as the fourth root and said what it may hold: the IO
channels, and nothing parked there for want of a better home. `.alint.yml`'s
`pkg-is-the-declared-channels` turns that sentence into a gate whose allowlist
is the decision record, and whose message demands this document before a name
is added to it.

`pkg/testkit/` is added. It is the BDD harness — the eight packages that build
the fixture trees, install the record/replay shim, run the registry's scenarios,
grade them and report on them. Until now it lived at `tests/libraries/`, beside
the scenarios it runs.

## Why it moved

`tests/` is what the engine asks of a host project: scenarios and step
definitions, and for this repository the fixture trees those scenarios name. A
host writes those. A host does not write a scenario runner, a cassette proxy or
a report server — it gets them. Keeping both under one root made `tests/`
describe two unrelated things, and made `./tests/...` a package pattern that
matched a suite and its own machinery indiscriminately.

## Why `pkg/` rather than a fifth root

A fifth root was the obvious answer and was wrong. The harness is shared
machinery that several roots reach: `scripts/` runs its report server, the
suites import its runner, and the engine's own golden tests sit beside it.
That is the definition `pkg/` already carries.

It was also, until recently, impossible. `runner` imported
`services/bdd-cli/claudecode` for the judge's model call, and `root-pkg-floor`
forbids `pkg/` from importing any root. Rewriting the judge onto
`pkg/cli/claude` — one headless schema-constrained turn instead of a
bidirectional streaming session — removed that import, and with it the reason
the harness could not live under `pkg/`. The simplification came first; the
move is what it bought.

## The exemption, stated plainly

`root-pkg-floor` negates `**/pkg/testkit/**`. Every other package under `pkg/`
is a floor: it is imported by the roots and imports none of them. The harness
is the opposite kind of thing — it exists to reach the roots it exercises, and
a harness that cannot import the code under test is not a harness. The negation
is therefore not an exception grudgingly made for one package's convenience; it
marks the one package under `pkg/` whose direction of dependency is inverted.

The cost is real and worth naming: the floor rule no longer holds uniformly
across `pkg/`, so a reader cannot infer a package's direction from its root
alone. `pkg/testkit/` is the single place to check, and it is named in the deny
list itself.

## What this does not change

The four roots stand. `pkg/` did not become a parking lot: the allowlist still
enumerates every package, and a ninth entry needs its own ADR. The harness's
own conventions are unchanged — the sentinel `go.mod`s, the shim's env
contract, the cassette layout — because none of them depended on where the
packages lived.
