---
paths:
  - "**/*.go"
---

# Go Conventions

**Logging vs terminal UI**: `slog` for application logging (structured, timestamped — debugging and the BDD judge); `internal/pkg/console` for CLI user output (prompts, `ALL CHECKS PASSED!`, separators, headers). `console` is excluded from `forbidigo` in `.golangci.yaml`.

**Single entity, single file**: each Go file holds one primary entity, filename = entity name in snake_case (`GitHubService` → `github_service.go` — "GitHub" is one word). Acceptable exceptions: function/constant-only files, re-export files, closely related types, data structures bundled with their primary entity. Enforced through review, not lint.
