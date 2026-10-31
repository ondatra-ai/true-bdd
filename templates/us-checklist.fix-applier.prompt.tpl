{{ if eq .Judgement "words" -}}
The Product Owner chose apply at the menu. Apply this approved word repair:
{{.FixPrompt}}
Edit ONLY the product working copy below, adding the glossary entry AND the
concept's terms (a string list), or rewording its means. Glossary entries
have term and means fields. Preserve unrelated product content.
Do not edit the Story, Scenario or architecture; the Word mapping comes later.
{{ range .Subject.TermDocuments }}{{ if eq .Key "product" }}
Edit `{{.Scratch}}`, the working copy of `{{.Path}}`.
{{ end }}{{ end }}
Return confirmation inside these markers:
=== FILE_START: {{.ResultPath}} ===
applied: true
target: "product"
summary: "<approved word repair>"
=== FILE_END: {{.ResultPath}} ===
{{ else -}}
{{ if .Judgement -}}
The Product Owner chose apply at the menu for Story {{.Subject.ID}}.
Approved fix:
{{.FixPrompt}}
Product working copy:
{{ range .Subject.TermDocuments }}{{ if eq .Key "product" }}
- `{{.Key}}`: edit `{{.Scratch}}`, the working copy of `{{.Path}}`
{{ end }}{{ end }}
If the approved fix changes a concept, edit only that product working copy;
return applied: true, target and summary inside the markers below.
If it rewrites a step, return the COMPLETE updated Story body inside those
markers, preserving every other field and criterion. Do not edit concepts
for a step rewrite. Current Story:
{{.Subject | toYaml}}
=== FILE_START: {{.ResultPath}} ===
<confirmation YAML or complete Story body YAML>
=== FILE_END: {{.ResultPath}} ===
{{ else -}}

## Current Story

**ID:** {{.Subject.ID}}
**Title:** {{.Subject.Title}}
**As a:** {{.Subject.AsA}}
**I want:** {{.Subject.IWant}}
**So that:** {{.Subject.SoThat}}
**Status:** {{.Subject.Status}}

### Current Acceptance Criteria
{{range .Subject.AcceptanceCriteria}}
**{{.ID}}:**
{{.Description}}
{{end}}

---

## Fix Prompt to Apply

{{.FixPrompt}}

---

## Instructions

Apply the changes described in the "Fix Prompt to Apply" section above.
The fix may target any field of the story — top-level (`title`,
`as_a`, `i_want`, `so_that`, `status`) or `acceptance_criteria` —
so output the COMPLETE updated story body. Preserve every field that
the fix prompt does not touch.

Output the complete updated story body using FILE_START/FILE_END
markers:

=== FILE_START: {{.ResultPath}} ===
(Complete story body YAML here — every top-level field plus
`acceptance_criteria`)
=== FILE_END: {{.ResultPath}} ===
{{ end -}}
{{ end -}}
