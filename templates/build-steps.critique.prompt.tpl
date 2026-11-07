{{if .Question}}{{.Question}}

{{end}}Verification evidence for one Concept Item. The YAML contains the Concept, its binding, declared terms' glossary entries, Word mappings, the Public interface kind and document path with the referenced entries, the definitions to judge, and runtime_api declarations when used:
{{.Context}}

Selected Steps package source declarations, with origin file paths and imports:
{{.Sources}}
{{if .StepArguments}}
## Step arguments

{{.StepArguments}}{{end}}
