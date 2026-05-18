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
   the file(s) the fix prompt targets.
2. Apply the changes described in the "Fix Prompt to Apply" section
   above:
   - Use Write to create a new test file when the fix prompt says so.
   - Use Edit to extend an existing test file when the fix prompt says
     so.
3. You MAY only modify files under these roots:
   - `tests/integration/`
   - `tests/e2e/`
   - `services/backend/`
   - `services/frontend/`
4. You MUST NOT touch `docs/requirements.yaml` or any path outside the
   four roots above.
5. The test you add MUST be named `test('{{.Subject.ID}}: <short
   description>', ...)` (or the equivalent in the chosen framework) so
   the next validation walk finds it by scenario id.
6. Do not duplicate an existing test for this scenario id.
7. After the changes succeed, output the confirmation block below.

=== FILE_START: {{.ResultPath}} ===
applied: true
target: <repo-relative path of the file you wrote/edited>
summary: "<one-line summary of what changed>"
=== FILE_END: {{.ResultPath}} ===
