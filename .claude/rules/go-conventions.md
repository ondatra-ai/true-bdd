---
paths:
  - "**/*.go"
---

# Go Conventions

**Logging vs terminal UI**: `slog` for application logging (structured, timestamped — debugging and the BDD judge); `internal/pkg/console` for CLI user output (prompts, `ALL CHECKS PASSED!`, separators, headers). `console` is excluded from `forbidigo` in `.golangci.yaml`. Gotcha: forbidigo's message on a raw `fmt.Print*` says "use slog for logging", which misdirects when the output is user-facing — the fix there is `console`, the one package the rule excludes. Nothing catches the reverse mistake (`console` where `slog` belongs).

**Single entity, single file**: each Go file holds one primary entity, filename = entity name in snake_case (`GitHubService` → `github_service.go` — "GitHub" is one word). Acceptable exceptions: function/constant-only files, re-export files, closely related types, data structures bundled with their primary entity. The casing is gated (`.alint.yml`, `go-filename-snake-case`); the one-entity rule is still review.

**The ceilings are numbers, not taste** (`.golangci.yaml`, ~100 linters on `default: standard`): a function is ≤80 lines and ≤40 statements (`funlen`), cyclomatic complexity ≤10 (`cyclop`; `gocyclo` is on but unconfigured, so its default 30 never binds first). Split at the ceiling rather than arguing with it. Also on and worth knowing before you write: `gochecknoglobals` and `gochecknoinits` (no package-level `var`, no `init()` — a global needs a `//nolint:gochecknoglobals` with a reason), `mnd` (name the magic number), `varnamelen` (no `x` living twelve lines), `nonamedreturns`, `testpackage` (tests go in `package foo_test`), `exhaustive` (a switch over an enum names every case).

**Errors have three rules that interlock**: `errors.New` for a static message, `fmt.Errorf` with `%w` to wrap — `forbidigo` forbids the middle case, a `fmt.Errorf` with no argument to interpolate. `wrapcheck` wants every error crossing a package boundary wrapped, and exempts `internal/pkg/errors` because that package already wraps. `err113` (no dynamic errors) is lifted in `_test.go`, where fixtures make them freely.

**Struct tags are snake_case both ways** (`tagliatelle`: `json: snake`, `yaml: snake`) — except where the casing is somebody else's wire contract, which is what the three carve-outs in `.golangci.yaml` mark: `adapters/github/`, `models/thread.go` (GitHub's camelCase API) and `infrastructure/testrunner/dto/` (`go test -json` PascalCase, `jest --json` camelCase). Match the wire there, not us.

**Whitespace is linted, and fixable**: `wsl_v5` (cuddling rules — first statement in a block may cuddle, a branch body up to 2 lines may) plus `nlreturn` (blank line before `return`). Never hand-fix these — `./scripts/lints.sh <file>.go` runs `golangci-lint --fix` over the whole package and rewrites them, which is also what the PostToolUse hook does on every edit.

**Comment budget** (`scripts/lint-comments.sh`): ≤3 lines of prose, ≤15 of indented block scheme. Prose is argument and compresses; a comment earns length only by carrying a fact learned by observation — one a simplifier would delete with no gate going red — and that fits in a line plus its provenance (PR #, comment id). State each fact once, at the line it guards; the narrative goes to `docs/for_further/`. Exempt from the prose cap only: the doc comment preceding `package X`, which `go doc`/gopls surface on every lookup.
