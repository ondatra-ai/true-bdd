// Sentinel nested module (mirrors services/bdd-web/go.mod). This is the
// PARKED Playwright suite: a self-contained npm package with its OWN
// `node_modules` and no first-party Go source. The go.mod exists ONLY to
// make the Go toolchain treat this directory as a separate module, so the
// repo-root `go build ./...`, `go vet ./...`, `go test ./...` and
// `golangci-lint run ./...` never descend into the third-party Go packages
// npm installs under node_modules (e.g. flatted/golang).
//
// Its replacement is the Go suite at tests/bdd-web/, which runs the same
// scenarios through tests/libraries/bddgo and drives the browser with
// playwright-go. This tree stays until that one covers it; nothing imports
// this module, it carries no dependencies and needs no go.sum.
module github.com/ondatra-ai/true-bdd/tests/legacy/bdd-web-playwright

go 1.25.0
