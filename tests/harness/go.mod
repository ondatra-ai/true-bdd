// Sentinel nested module (mirrors harness/go.mod). `tests/harness/` is a
// self-contained Playwright suite with its OWN `node_modules` (installed by
// `npm ci` here) and NO first-party Go source. This go.mod exists ONLY to make
// the Go toolchain treat tests/harness/ as a separate module so the repo-root
// `go build ./...`, `go vet ./...`, `go test ./...`, and `golangci-lint run ./...`
// never descend into third-party Go packages that npm installs under
// tests/harness/node_modules (e.g. flatted/golang). Nothing imports this module;
// it carries no dependencies and needs no go.sum. The Go fixture materializer
// this suite builds lives in the root module at tests/materializer/.
module github.com/ondatra-ai/true-bdd/tests/harness

go 1.25.0
