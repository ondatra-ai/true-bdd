
# Step Coverage Validation

## Purpose
Evaluate whether one scenario from the scenario registry is executable:
whether every one of its steps binds to a step definition in the suite
that owns it.

## Instructions
1. Read the architectural spec listed under Reference Documentation and
   find the `architecture.testing.suites[]` entry whose `service:`
   equals the subject's **Service**. That entry's `path:` is the suite
   root; its step definitions live under `<path>/steps/`.
2. Glob and Read every file under `<suite path>/steps/` and collect the
   registered patterns — each is the first argument of a
   `suite.Step(`<regexp>`, …)` call.
3. Match each of the subject's steps against those patterns, using the
   step TEXT only: the Given/When/Then/And keyword is not part of what a
   definition binds.
4. Answer the validation question against what you found on disk.
5. Always explain your reasoning BEFORE the answer block.

Do not assume a definition exists because one plainly should. A pattern
you did not read is a pattern that is not there.
{{- if .Docs }}

## Reference Documentation
{{- range $key, $doc := .Docs }}
- Read(`{{ $doc.FilePath }}`) — {{ $key }}
{{- end }}
{{- end }}

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

## Validation Question

{{.Question}}{{- if .Rationale }} SO THAT WE ENSURE {{.Rationale}}{{- end }}
{{- if .FixTemplate }}

## If Validation Fails
If your answer is `fail`, copy the following fix template VERBATIM into
the `fix_prompt:` field of your result. Do not paraphrase.

{{ .FixTemplate }}
{{- end }}

## Answer Format
Output your answer using this exact format:

=== FILE_START: {{.ResultPath}} ===
answer: <pass | fail>
context:
  - "<context line per the question's context spec>"
{{- if .FixTemplate }}
fix_prompt: |
  <if answer is fail, paste the fix template above verbatim>
  <if answer is pass, omit this field entirely>
{{- end }}
=== FILE_END: {{.ResultPath}} ===
