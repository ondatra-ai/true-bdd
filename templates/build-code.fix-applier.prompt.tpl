## Subject — Failing Test

**Test ID:** {{.Subject.ID}}
**Service:** {{if .Subject.Service}}{{.Subject.Service}}{{else}}none is known{{end}}
**Suite:** {{.Subject.Suite}}
**Framework:** {{.Subject.Framework}}
**Test Name:** {{.Subject.TestName}}
**Source File:** {{.Subject.FilePath}}

### Last Failure Output
```
{{.Subject.FailureOutput}}
```

{{ if .Subject.Scenario }}
## Subject — Registry Scenario

**Scenario ID:** {{.Subject.Scenario.ID}}
**Description:** {{.Subject.Scenario.Description}}
**Service:** {{.Subject.Scenario.Service}}
**Path:** {{.Subject.Scenario.Path}}

### Steps
{{.Subject.Scenario.FormatSteps}}
{{ if .Subject.Spec.Declared }}
## What the spec says these steps mean

The Specification defines the contract the production code must satisfy.
Use these phrases, glossary meanings and word mappings with the whole
Scenario above when repairing the failing test.

### Scenario steps and their phrases
{{ range .Subject.Spec.Steps }}
- {{ .Block }} step "{{ .Text }}"{{ range .Templates }} matches `{{ . }}`{{ else }} matches no catalogue phrase{{ end }}
{{- end }}

### Phrases
{{ range .Subject.Spec.Phrases }}
- `{{ .Template }}` ({{ .Block }} step): {{ .Means }}
  Words: {{ if .Terms }}{{ join ", " .Terms }}{{ else }}none{{ end }}
{{- end }}

### Words
{{ range .Subject.Spec.Terms }}
#### {{ .Term }}

{{ if .Means }}{{ range .Means }}- Means: {{ . }}
{{ end }}{{ else }}- The glossary does not define it.
{{ end }}
{{- range .Mappings }}- In the implementation: {{ .Technical }}
- Observed by: {{ .ObservedBy }}
{{ else }}- The architecture's word mapping does not map it.
{{ end }}
{{- end }}
{{ end }}
{{ end }}

---

## Fix Prompt to Apply

{{.FixPrompt}}

---

## Instructions

1. Use the Read, Glob, and Grep tools to inspect the current state of
   the production source the fix prompt targets.
2. Apply the changes described in the "Fix Prompt to Apply" section
   above:
   - Use Write to create a new file under `services/*` when the fix
     prompt says so.
   - Use Edit to modify an existing file under `services/*` when the
     fix prompt says so.
3. You MAY only modify files under `services/*`.
4. You MUST NOT touch any of the following:
   - Anything under `tests/`
   - Any file matching `*_test.go`
   - Anything under `services/*/__tests__/`
   - Anything under `docs/`
5. Do not weaken, skip, or remove any assertion in the failing test.
   The test is the contract; the production code must conform to it.
6. After the changes succeed, output the confirmation block below.

=== FILE_START: {{.ResultPath}} ===
applied: true
target: "<repo-relative path of the file you wrote/edited>"
summary: "<one-line summary of what changed>"
=== FILE_END: {{.ResultPath}} ===
