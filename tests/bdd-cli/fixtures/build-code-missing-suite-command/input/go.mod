// Sentinel module for this fixture's own Go tests.
//
// It exists to keep these files out of the engine's module: one of them
// is DESIGNED to fail, and without a module boundary the repo's own
// `go test ./...` and golangci-lint would compile it and go red.
module fixture/buildcode

go 1.25.0
