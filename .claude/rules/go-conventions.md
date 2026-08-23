---
paths:
  - "**/*.go"
---

# Go Conventions

**Logging vs terminal UI**: `slog` for application logging (structured, timestamped — debugging and the BDD judge); `internal/pkg/console` for CLI user output (prompts, `ALL CHECKS PASSED!`, separators, headers). `console` is excluded from `forbidigo` in `.golangci.yaml`. Gotcha: forbidigo's message on a raw `fmt.Print*` says "use slog for logging", which misdirects when the output is user-facing — the fix there is `console`, the one package the rule excludes. Nothing catches the reverse mistake (`console` where `slog` belongs).

**Single entity, single file**: each Go file holds one primary entity, filename = entity name in snake_case (`GitHubService` → `github_service.go` — "GitHub" is one word). Acceptable exceptions: function/constant-only files, re-export files, closely related types, data structures bundled with their primary entity. Enforced through review, not lint.
