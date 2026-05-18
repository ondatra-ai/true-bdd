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
