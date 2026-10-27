## Subject — Registry Scenario

**Scenario ID:** {{.Subject.ID}}
**Description:** {{.Subject.Description}}
**Service:** {{.Subject.Service}}
{{- if .Subject.Requirement }}
**Requirement:** {{.Subject.Requirement}}
{{- end }}

### Steps
{{.Subject.FormatSteps}}

---

## Fix Prompt to Apply

{{.FixPrompt}}

---

## Instructions

1. Use the Read, Glob, and Grep tools to inspect the current state of
   the file(s) the fix prompt targets, and the sibling step definitions
   beside them.
2. Apply the changes described in the "Fix Prompt to Apply" section
   above:
   - Use Write to create a new file when the fix prompt says so.
   - Use Edit to extend an existing file when the fix prompt says so.
3. You MAY only modify files under the `steps/` package of the test
   suite the fix prompt names. That path came from the architectural
   spec; do not write anywhere else, and do not invent a sibling
   directory when the named one does not exist — report the failure
   instead.
4. You MUST NOT touch the scenario registry.
5. Each new definition MUST be registered in the suite's existing
   `Register` function, so a single call site still lists everything the
   suite binds.
6. Do not duplicate a definition whose pattern already matches the same
   step, and do not loosen an existing pattern to make it match.
7. After the changes succeed, output the confirmation block below.

=== FILE_START: {{.ResultPath}} ===
applied: true
target: "<repo-relative path of the file you wrote/edited>"
summary: "<one-line summary of what changed>"
=== FILE_END: {{.ResultPath}} ===
