## Current Story

**ID:** {{.Subject.ID}}
**Title:** {{.Subject.Title}}
**As a:** {{.Subject.AsA}}
**I want:** {{.Subject.IWant}}
**So that:** {{.Subject.SoThat}}

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

Output the complete updated acceptance_criteria array using FILE_START/FILE_END markers:

=== FILE_START: {{.ResultPath}} ===
(YAML array of acceptance criteria here)
=== FILE_END: {{.ResultPath}} ===
