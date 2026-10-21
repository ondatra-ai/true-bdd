## Subject — Registry Scenario

**Scenario ID:** {{.Subject.ID}}
**Description:** {{.Subject.Description}}

### Steps
{{.Subject.FormatSteps}}
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

{{ if .Subject.Writes "architecture_yaml" -}}
1. Use the Read tool to load the working copy the fix prompt's
   document names, and find the list it changes (`catalogue:` or
   `glossary:` in the product document, `architecture.terms:` in the
   architecture document).
{{- else -}}
1. Use the Read tool to load the working copy the fix prompt's
   document names, and find the list it changes (`catalogue:` or
   `glossary:`).
{{- end }}
2. Apply the changes described in the "Fix Prompt to Apply" section
   above with the Edit tool on that working copy: append a new entry at
   the end of its list, at the list's own indentation, or replace the
   old_string verbatim.
3. Copy each entry's YAML exactly as the fix prompt gives it. Keep a
   template single-quoted: it carries `{`, `[` and `"` of its own.
4. You MAY only edit the working copies listed above. The documents
   themselves, the scenario registry, a generated test file and a step
   definition are all out of bounds.
5. After the edits succeed, output the confirmation block below.

=== FILE_START: {{.ResultPath}} ===
applied: true
target: "<path of the working copy you edited>"
summary: "<one-line summary of what changed>"
=== FILE_END: {{.ResultPath}} ===
