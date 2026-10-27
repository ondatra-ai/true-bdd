{{ if eq .Check "" -}}
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
{{ else -}}
## Story

**ID:** {{.Subject.ID}}
**Title:** {{.Subject.Title}}

The Story is shown for context only. This fix never edits it.

---

## Fix Prompt to Apply

{{.FixPrompt}}

---

## Where to write

Each term document has a working copy in this run's tmp directory. Edit
the working copy, never the document itself:
{{- range .Subject.TermDocuments }}
- `{{ .Key }}`: edit `{{ .Scratch }}`, the working copy of `{{ .Path }}`
{{- end }}

The engine replaces each document with its working copy once the copy
reads as valid YAML, so a copy you leave unparseable changes nothing.

## Instructions

1. Use the Read tool to load the working copy of the `product` document
   and find its `catalogue:` list.
2. Apply the changes described in the "Fix Prompt to Apply" section
   above with the Edit tool on that working copy: append a new entry at
   the end of the list, at the list's own indentation, or replace the
   old_string verbatim.
3. Copy each entry's YAML exactly as the fix prompt gives it. Keep a
   template single-quoted: it carries `{`, `[` and `"` of its own.
4. You MAY only edit the working copies listed above. The documents
   themselves and the Story are out of bounds.
5. After the edits succeed, output the confirmation block below.

=== FILE_START: {{.ResultPath}} ===
applied: true
target: "<path of the working copy you edited>"
summary: "<one-line summary of what changed>"
=== FILE_END: {{.ResultPath}} ===
{{ end -}}
