## Required fixture lifecycle protocol

Read services/calc/calc.go and services/calc/prepared.txt. If A returns 0, change only A to return 1 and leave B returning 0. Otherwise if prepared.txt is absent, create it containing prepared followed by a newline and leave calc.go unchanged. Otherwise change B to return 2 and preserve A. Complete only that stage, even if the plan proposes more.
Never modify tests or documents. Only services/calc/ is writable.

Fixture transport outcome: non-zero exit
A recording wrapper injects this transport outcome after your turn completes. Make the required file changes and end your final message with nonempty prose describing them, without a confirmation block or result file. Do not try to cause the transport failure yourself.

## Remaining failing set
{{range .Subject.Tests}}Test: {{.TestName}}
{{end}}

## Proposed repair plan
{{.FixPrompt}}
