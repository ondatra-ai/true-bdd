
# Apply Checklist Validation

## Purpose
Evaluate whether one acceptance criterion from a refined story is
correctly reflected in the requirements registry (the scratch copy of
`docs/requirements.yaml` for this run).

## Instructions
1. Read the registry file at the path below.
2. Read any reference documentation listed.
3. Answer the validation question against the registry's current state.
4. Always explain your reasoning BEFORE the answer block.

## Registry (read this file first)
- Read(`{{.Subject.RequirementsScratchPath}}`) — current registry state
{{- if .Docs }}

## Reference Documentation
{{- range $key, $doc := .Docs }}
- Read(`{{ $doc.FilePath }}`) — {{ $key }}
{{- end }}
{{- end }}

## Subject — Acceptance Criterion

**Story ID:** {{.Subject.StoryID}}
**Story Path:** {{.Subject.StoryPath}}
**AC ID:** {{.Subject.ACID}}
**Lineage Scenario ID:** {{.Subject.LineageScenarioID}}
**Description:** {{.Subject.Description}}

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
