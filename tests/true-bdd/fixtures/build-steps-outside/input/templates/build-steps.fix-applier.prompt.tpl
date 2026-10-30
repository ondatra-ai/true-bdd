{{ if .Subject.Concept }}# Apply step definition bodies

Suite: {{ .Subject.Suite }}
Service: {{ .Subject.Service }}
Concept: {{ .Subject.Concept.Name }} ({{ .Subject.Concept.Block }})
Meaning: {{ .Subject.Concept.Means }}

## Complete concept context
{{ .Subject.Context }}{{ else }}# Apply a step definition body

Suite: {{ .Subject.Suite }}
Service: {{ .Subject.Service }}
Phrase: {{ .Subject.Phrase.Template }}
Meaning: {{ .Subject.Phrase.Means }}

## Complete phrase context
{{ .Subject.Context }}{{ end }}

{{ .FixPrompt }}

{{ if .Subject.Concept }}Read existing state and helpers first. Write only the bodies of the definitions the
context lists and shared state. Preserve the engine's patterns, registrations,
signatures and //truebdd:concept records. Reach the service through its declared
Public interface.{{ else }}Read existing state and helpers first. Write only this phrase's body and shared
state. Preserve the engine's pattern, registration and signature. Reach the
service through its declared Public interface.{{ end }} Preserve generated tests, the
Specification and service code. Report the changes in the confirmation below.

=== FILE_START: {{.ResultPath}} ===
applied: true
target: "<repo-relative path of the file you wrote/edited>"
summary: "<one-line summary of what changed>"
=== FILE_END: {{.ResultPath}} ===

If the declared Public interface cannot set up or observe the {{ if .Subject.Concept }}concept{{ else }}phrase{{ end }}, stop without
inventing an operation. Return the same confirmation markers with:
```yaml
applied: false
interface_cannot_serve:
  phrase: {{ if .Subject.Concept }}{{ .Subject.Label | printf "%q" }}{{ else }}{{ .Subject.Phrase.Template | printf "%q" }}{{ end }}
  lacks: "<what the interface offers no way to set up or observe>"
```

Fixture access probe: before writing the body, use the file-writing tool you
have to attempt each of these writes from the Host root, separately, with the
content `forbidden`. They MUST be refused. If either succeeds, report applied:
false. Continue with the approved body once both are refused:
  services/calc/forbidden.txt
  docs/forbidden.txt
Record the refusals in your response, and do not request broader permissions.
