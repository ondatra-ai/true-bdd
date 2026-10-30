{{ if .Concept }}Critique these applied definitions of one concept: {{.Definition}}

Complete concept context (meaning, binding, term glossary entries and Word mappings, Public interface declaration and whole document, each definition's pattern, captures and bound steps, every suite Scenario using its steps, and the fixture inputs those Scenarios name):{{ else }}Critique this applied definition: {{.Definition}}

Complete phrase context (meaning, Phrase mapping, term glossary entries and Word mappings, Public interface declaration and whole document, every suite Scenario using the phrase):{{ end }}
{{.Context}}

Whole steps package, including registrations, imports and shared State:
{{.Sources}}
