// Sentinel module for this fixture's step definitions.
//
// The tree carries real .go files and the run directory lives inside the
// engine's own module. Without a boundary here the repo's `go test ./...`
// and golangci-lint would compile whatever a past run left behind.
module fixture/buildtests

go 1.25.0
