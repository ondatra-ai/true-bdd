---
paths:
  - "**/*.go"
---

# Go Conventions

**Logging vs terminal UI**: `slog` for application logging (structured, timestamped — debugging and the BDD judge); `internal/pkg/console` for CLI user output (prompts, `ALL CHECKS PASSED!`, separators, headers). `console` is excluded from `forbidigo` in `.golangci.yaml`. Gotcha: forbidigo's message on a raw `fmt.Print*` says "use slog for logging", which misdirects when the output is user-facing — the fix there is `console`, the one package the rule excludes. Nothing catches the reverse mistake (`console` where `slog` belongs).

**Single entity, single file**: each Go file holds one primary entity, filename = entity name in snake_case (`GitHubService` → `github_service.go` — "GitHub" is one word). Acceptable exceptions: function/constant-only files, re-export files, closely related types, data structures bundled with their primary entity. Enforced through review, not lint.

**Comment budget** (`scripts/lint-comments.sh`): ≤3 lines of prose, ≤15 of indented block scheme. Prose is argument and compresses; a comment earns length only by carrying a fact learned by observation — one a simplifier would delete with no gate going red — and that fits in a line plus its provenance (PR #, comment id). State each fact once, at the line it guards; the narrative goes to `docs/for_further/`. Exempt from the prose cap only: the doc comment preceding `package X`, which `go doc`/gopls surface on every lookup.
