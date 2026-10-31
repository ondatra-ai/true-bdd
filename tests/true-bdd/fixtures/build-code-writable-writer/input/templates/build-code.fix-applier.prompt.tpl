## Subject — Failing Set

{{ range .Subject.Tests }}
**Test ID:** {{ .ID }}
**Service:** {{ if .Service }}{{ .Service }}{{ else }}none is known{{ end }}
**Suite:** {{ .Suite }}
**Framework:** {{ .Framework }}
**Test Name:** {{ .TestName }}
**Source File:** {{ .FilePath }}
{{ end }}

### Failure groups
{{ range .Subject.Groups }}
Tests: {{ join ", " .IDs }}
```
{{ .Output }}
```
{{ end }}

### Specification documents
{{ range $key, $path := .Subject.Documents }}
- Read(`{{ $path }}`) — {{ $key }}
{{ end }}

{{ range .Subject.Scenarios }}
### Registry Scenario
**Scenario ID:** {{ .ID }}
**Description:** {{ .Description }}
**Service:** {{ .Service }}
**Path:** {{ .Path }}

{{ .FormatSteps }}
{{ end }}

{{ with .Subject.Specification }}
{{ if .Concepts }}
## What the spec says these steps mean

### Concepts
{{ range .Concepts }}
- `{{ .Concept }}` ({{ .Block }} concept): {{ .Means }}
  Words: {{ if .Terms }}{{ join ", " .Terms }}{{ else }}none{{ end }}
{{ range .Bindings }}  Bound for {{ .Service }} {{ if .Intent }}by intent: {{ .Intent }}{{ else }}through {{ join ", " .References }}: {{ .Does }}{{ end }}
{{ else }}  The architecture binds it for no service of these steps.
{{ end }}{{ end }}

### Words
{{ range .Terms }}
#### {{ .Term }}
{{ range .Means }}- Means: {{ . }}
{{ else }}- The glossary does not define it.
{{ end }}
{{ range .Mappings }}- In the implementation: {{ .Technical }}
- Observed by: {{ .ObservedBy }}
{{ else }}- The architecture's word mapping does not map it.
{{ end }}
{{ end }}
{{ end }}
{{ end }}

## Proposed repair plan
{{ .FixPrompt }}

## Edit roots
{{ range .WriteRoots }}
- `{{ . }}`
{{ end }}
Never modify tests or step definitions. Never weaken, skip or remove assertions.

{{ if .TestCommand }}
## Test commands
Run in `{{ .TestCommand.Dir }}` using exactly one of these forms:
- Whole suite: `{{ .TestCommand.Command }}`
- Selection: `{{ .TestCommand.Command }} {{ .TestCommand.Selector }} '<selection>'`
The selection is a nonempty framework regular expression. {{ if .CodexWriter }}Keep the command in the foreground; if the tool returns a process/session id, poll that process with your native process tool until it completes.{{ else }}Run in the foreground and wait for completion. Background commands are denied.{{ end }}
{{ else }}
No test command is granted to this writer. Report that limitation in the confirmation.
{{ end }}

## How to work
1. Read the Specification and the harness first to understand the service's contract.
2. Change the service as a whole, using the plan and your investigation.
3. After each part, run the tests that part serves and fix what fails.
4. Run every test of the failing set before reporting. Tests outside the failing set are not your subject.
5. Make all changes during this turn, then report what changed and which tests of the set you saw pass.

End your final message with the confirmation block below, written in the message itself. The result path in its heading names where the engine saves it; never write that path yourself.

=== FILE_START: {{ .ResultPath }} ===
applied: true
impossible: []
target: "<repo-relative service path>"
summary: "<what changed and which tests of the set you saw pass>"
=== FILE_END: {{ .ResultPath }} ===

Only when no solution exists inside the declared edit roots, fill impossible with a list of {path, reason} for required undeclared files. Keep any edits already made and report their outcome. Otherwise leave impossible empty; applied: false by itself permits another attempt.

## Controlled writer-discovery protocol
Repair Add in services/calc/calc.go, and create services/calc/prepared.txt containing exactly "prepared" followed by a newline. After those changes, report impossible: [{path: go.sum, reason: "deployment requires a root checksum repair outside the declared paths"}]. Report applied: true, retaining the service edits. This exercises an obstruction discovered only by the writer; do not seek another attempt or modify go.sum.
