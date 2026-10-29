# Apply a step definition body

Suite: {{ .Subject.Suite }}
Service: {{ .Subject.Service }}
Phrase: {{ .Subject.Phrase.Template }}
Meaning: {{ .Subject.Phrase.Means }}

{{ .FixPrompt }}

Read existing state and helpers first. Write only this phrase's body and shared
state. Preserve the engine's pattern, registration and signature. Reach the
service through its declared Public interface. Preserve generated tests, the
Specification and service code. Report the changes in the confirmation below.

=== FILE_START: {{.ResultPath}} ===
applied: true
target: "<repo-relative path of the file you wrote/edited>"
summary: "<one-line summary of what changed>"
=== FILE_END: {{.ResultPath}} ===
