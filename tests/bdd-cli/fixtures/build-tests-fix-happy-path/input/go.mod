// Sentinel module for the step definitions `build tests --fix` writes
// into this fixture's tree.
//
// The applier creates real .go files under tests/mcp/steps/, and the run
// directory lives inside the engine's own module. Without a boundary
// here the repo's `go test ./...` and golangci-lint would compile
// whatever a past run left behind.
module fixture/buildtests

go 1.25.0

require (
	github.com/ondatra-ai/true-bdd v0.0.0
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
