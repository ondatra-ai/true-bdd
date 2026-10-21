// Sentinel module for the step definitions `build tests --fix` would
// write into this fixture's tree.
//
// The run is refused before any is written, but a refusal that failed to
// happen would leave real .go files under tests/mcp/ inside the engine's
// own module. Without a boundary here the repo's `go test ./...` and
// golangci-lint would compile whatever a past run left behind.
module fixture/buildtests

go 1.25.0
