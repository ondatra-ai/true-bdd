# Step definition body

Suite: {{ .Subject.Suite }}
Phrase: {{ .Subject.Phrase.Template }}
Meaning: {{ .Subject.Phrase.Means }}

{{ .Question }}
{{ .Rationale }}
{{ .FixTemplate }}

=== FILE_START: {{ .ResultPath }} ===
answer: <pass | fail>
context:
  - <finding>
=== FILE_END: {{ .ResultPath }} ===
