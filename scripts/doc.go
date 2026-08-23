// Package scripts is the root of the tooling that drives this repository: the
// conversation history hook, the ClickUp ticket interface, the pull-request
// merge loop, and the lint gates beside them in shell.
//
// One of three roots — services/, tests/, scripts/ — enforced by
// lint-layout.sh. These were Python scripts invoked directly from .claude/,
// and they are Go packages under a normal (non-dot) tree for one reason: the
// Go tool skips any directory whose name begins with a dot, so a package
// under .claude/ is invisible to `go build ./...`, `go test ./...` and
// golangci-lint. Two thousand lines of workflow logic outside every gate in
// the repository is not a trade worth making, so the source lives here and
// the entry points stay where their callers already look for them — thin
// `.sh` shims under .claude/ that `go run` the matching command in
// scripts/cmd/.
package scripts
