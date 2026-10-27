## Required fixture lifecycle protocol

Change A to return 1 and B to return 2 in services/calc/calc.go. Keep both functions and make no other service change.
Never modify tests or documents. Only services/calc/ is writable.

Fixture transport outcome: empty output
A recording wrapper injects this transport outcome after your turn completes. Make the required file changes and end your final message with nonempty prose describing them, without a confirmation block or result file. Do not try to cause the transport failure yourself.

## Remaining failing set
{{range .Subject.Tests}}Test: {{.TestName}}
{{end}}

## Proposed repair plan
{{.FixPrompt}}
