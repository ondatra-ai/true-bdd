// Sentinel module boundary for the whole fixture tree.
//
// Fixtures carry Go sources in two places, and only one of them was ever
// guarded:
//
//   - `<fixture>/input/` — the designed host project. Each such tree
//     ships its own go.mod, because it has to be a real module once the
//     runner materialises it into a run dir.
//   - `<fixture>/cassettes/*/fsdiff/after/` — the RECORDING of files an
//     AI turn wrote. Nothing puts a go.mod there: the fsdiff captures
//     only the files that changed, and the input tree's go.mod did not
//     change. So a recorded `.go` file landed directly in this repo's
//     module.
//
// That is the same trap the per-fixture sentinels exist to close, just
// reached from the other side. Left open, the repo's own `go test ./...`
// and golangci-lint compile whatever an applier happened to write — and
// since fixtures deliberately plant broken and failing code, the repo's
// gates would go red for reasons nobody wrote.
//
// One boundary here covers both cases for every fixture, present and
// future. Nothing under this directory is ever built as part of the
// engine; the harness reads these files as data and copies them, so
// excluding them from the module costs nothing.
//
// This file is never materialised into a run dir — the runner overlays
// `<fixture>/input/`, not the fixture directory itself.
module fixture/tree

go 1.25.0
