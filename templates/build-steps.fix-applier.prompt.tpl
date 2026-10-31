# Apply step definition bodies

Suite: {{ .Subject.Suite }}
Service: {{ .Subject.Service }}
Concept: {{ .Subject.Concept.Name }} ({{ .Subject.Concept.Block }})
Meaning: {{ .Subject.Concept.Means }}

## Complete concept context
{{ .Subject.Context }}

{{ .FixPrompt }}

Read existing state and helpers first. Write only the bodies of the definitions the
context lists and shared state. Preserve the engine's patterns, registrations,
signatures and //truebdd:concept records. Reach the service through its declared
Public interface. Preserve generated tests, the
Specification and service code. Report the changes in the confirmation below.

=== FILE_START: {{.ResultPath}} ===
applied: true
target: "<repo-relative path of the file you wrote/edited>"
summary: "<one-line summary of what changed>"
=== FILE_END: {{.ResultPath}} ===

If the declared Public interface cannot set up or observe the concept, stop without
inventing an operation. Return the same confirmation markers with:
```yaml
applied: false
interface_cannot_serve:
  phrase: {{ .Subject.Label | printf "%q" }}
  lacks: "<what the interface offers no way to set up or observe>"
```
