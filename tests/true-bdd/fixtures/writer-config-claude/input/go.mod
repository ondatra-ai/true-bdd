// Sentinel module for this fixture's own Go sources.
//
// Run dirs live under tmp/test_run INSIDE the repo module, so without
// this boundary the repo's own `go test ./...` and golangci-lint would
// compile the deliberately-broken production file below and go red for
// a reason nobody wrote.
module fixture/buildcodefix

go 1.25.0
