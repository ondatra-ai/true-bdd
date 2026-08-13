// Sentinel nested module (NICE-4). `services/bdd-web/` is a Next.js app with NO
// first-party Go source; this go.mod exists ONLY to make the Go toolchain
// treat services/bdd-web/ as a separate module so the repo-root `go build ./...`,
// `go vet ./...`, `go test ./...`, and `golangci-lint run ./...` never
// descend into third-party Go packages that `npm ci` installs under
// services/bdd-web/node_modules (e.g. flatted/golang). Nothing in the repo imports
// this module; it carries no dependencies and needs no go.sum.
module github.com/ondatra-ai/true-bdd/services/bdd-web

go 1.25.0
