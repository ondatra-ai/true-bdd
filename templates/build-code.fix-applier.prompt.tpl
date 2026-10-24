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
{{ if .Declared }}
## What the spec says these steps mean

### Phrases
{{ range .Phrases }}
- `{{ .Template }}` ({{ .Block }} step): {{ .Means }}
  Words: {{ if .Terms }}{{ join ", " .Terms }}{{ else }}none{{ end }}
{{ end }}

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
Never modify tests, *_test.go, __tests__, or documents. Never weaken, skip or remove assertions.

{{ if .TestCommand }}
## Test commands
Run in `{{ .TestCommand.Dir }}` using exactly one of these forms:
- Whole suite: `{{ .TestCommand.Command }}`
- Selection: `{{ .TestCommand.Command }} {{ .TestCommand.Selector }} '<selection>'`
The selection is a nonempty framework regular expression. Keep the command in the foreground; {{ if .CodexWriter }}if the tool returns a process/session id, poll that process with your native process tool until it completes.{{ else }}if the tool returns a job id, use job_output to read completion.{{ end }}
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
target: "<repo-relative service path>"
summary: "<what changed and which tests of the set you saw pass>"
=== FILE_END: {{ .ResultPath }} ===
