
# Test Coverage Validation

## Purpose
Evaluate whether one scenario from `docs/requirements.yaml` is covered
by an executable test in the codebase.

## Instructions
1. Search the project's test trees for any file that references the
   subject scenario id literally.
2. Read any reference documentation listed.
3. Answer the validation question against what you find on disk.
4. Always explain your reasoning BEFORE the answer block.

## Search Roots
Look under these paths only:
- `tests/integration/`
- `tests/e2e/`
- `services/backend/`
- `services/frontend/`

Use Glob and Grep to find files that mention the scenario id
(`{{.Subject.ID}}`) — typically inside a `test('<id>: ...')` name,
a tag, or a leading comment. Use Read to confirm the match in context.
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
