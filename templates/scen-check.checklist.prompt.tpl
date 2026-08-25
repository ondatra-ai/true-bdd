# Registry Scenario Validation

## Purpose
Evaluate ONE scenario from the scenario registry against one quality
question. The scenario's own fields are reproduced in full below —
everything the question can be answered from is on this page.

## Instructions
1. Read any reference documentation listed below.
2. Read the subject scenario reproduced below.
3. Answer the validation question about THAT SCENARIO ALONE.
4. Always explain your reasoning BEFORE the answer block.

You are not shown the registry file, and you must not go looking for it.
A question you can only answer by comparing this scenario against others
is a question this checklist does not ask — answer the one it does.
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
**Generated test file:** {{.Subject.Path}}

### Steps
{{.Subject.FormatSteps}}
### Lineage — user_stories[]
{{.Subject.FormatUserStories}}
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
