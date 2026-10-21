// Package enginecoverage measures which of the engine's code the replay suite
// exercises, so behaviour no Scenario specifies is visible before a rebuild
// from the spec drops it (spec #127, Q20 of
// docs/qna/20261021-112516-self-spec-and-doc-validation-chain.yaml).
//
// It is measurement, not product, and so is tested by `go test` rather than by
// a Registry Scenario. The engine runs as a subprocess of the harness, which is
// why this is Go's binary coverage and not `go test -cover`:
//
//	build    go build -cover -coverpkg=<the engine's packages>
//	exercise anything that runs that binary with GOCOVERDIR set (the replay suite)
//	merge    go tool covdata textfmt, one text profile from every run's counters
//	report   per package, then per function, every run of blocks that never ran
package enginecoverage
