# Apply step definition bodies

Suite: {{ .Subject.Suite }}
Service: {{ .Subject.Service }}
Concept: {{ .Subject.Concept.Name }} ({{ .Subject.Concept.Block }})
Meaning: {{ .Subject.Concept.Means }}

{{ if regexMatch "(?m)^runtime_api:" .Subject.Context }}Read the semantic core and Public interface first; next read the saved complete go doc -all files
named by runtime_api for the runtime's API meanings; then read suite source and fixture inputs.

{{ end }}Read the listed Scenario keys from the Registry path and inspect their fixture
input directories under fixture_root as needed (exclude cassettes/). Read the definitions, State,
helpers and architecture-declared test libraries. These paths carry the complete
evidence; no digest or summary replaces it. Never read service implementation.

## Concept context and evidence paths
{{ .Subject.Context }}

{{ .FixPrompt }}

{{ if regexMatch "(?m)^runtime_api:" .Subject.Context }}After reading the runtime API, read existing state and helpers.{{ else }}Read existing state and helpers first.{{ end }} Write only the bodies of the definitions the
context lists, shared state and code they share in tests/<suite>/helpers/. Preserve the engine's patterns, registrations,
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
  concept: {{ .Subject.Label | printf "%q" }}
  lacks: "<what the interface offers no way to set up or observe>"
```
{{if .StepArguments}}
## Step arguments

{{.StepArguments}}{{end}}