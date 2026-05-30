## Subject — Failing Test

**Test ID:** {{.Subject.ID}}
**Service:** {{.Subject.Service}}
**Layer:** {{.Subject.Layer}}
**Framework:** {{.Subject.Framework}}
**Test Name:** {{.Subject.TestName}}
**Source File:** {{.Subject.FilePath}}

### Last Failure Output
```
{{.Subject.FailureOutput}}
```

---

## Fix Prompt to Apply

{{.FixPrompt}}

---

## Instructions

1. Use the Read, Glob, and Grep tools to inspect the current state of
   the production source the fix prompt targets.
2. Apply the changes described in the "Fix Prompt to Apply" section
   above:
   - Use Write to create a new file under `services/*` when the fix
     prompt says so.
   - Use Edit to modify an existing file under `services/*` when the
     fix prompt says so.
3. You MAY only modify files under `services/*`.
4. You MUST NOT touch any of the following:
   - Anything under `tests/`
   - Any file matching `*_test.go`
   - Anything under `services/*/__tests__/`
   - Anything under `docs/`
5. Do not weaken, skip, or remove any assertion in the failing test.
   The test is the contract; the production code must conform to it.
6. After the changes succeed, output the confirmation block below.

=== FILE_START: {{.ResultPath}} ===
applied: true
target: <repo-relative path of the file you wrote/edited>
summary: "<one-line summary of what changed>"
=== FILE_END: {{.ResultPath}} ===
