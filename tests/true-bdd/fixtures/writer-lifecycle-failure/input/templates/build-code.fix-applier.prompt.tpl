## Failing set
{{range .Subject.Tests}}Test: {{.TestName}}
{{end}}
## Plan
{{.FixPrompt}}
Implement calc.A() returning 1 and calc.B() returning 2 in services/calc/calc.go.
Run the granted selection in `{{.TestCommand.Dir}}`:
`{{.TestCommand.Command}} {{.TestCommand.Selector}} 'TestA|TestB'`
Poll your process with the native process tool until completion if it yields a session id. Do not stop before the selection passes.
End your final message with this block in the message itself, never write the result file:
=== FILE_START: {{.ResultPath}} ===
applied: true
target: services/calc/calc.go
summary: Implemented A and B and observed both selection members passing.
=== FILE_END: {{.ResultPath}} ===
