# Step definition body

Suite: {{ .Subject.Suite }}
Concept: {{ .Subject.Label }}
Meaning: {{ .Subject.Concept.Means }}

{{ .Question }}
{{ .Rationale }}
{{ .FixTemplate }}

=== FILE_START: {{ .ResultPath }} ===
answer: <pass | fail>
context:
  - <finding>
=== FILE_END: {{ .ResultPath }} ===
