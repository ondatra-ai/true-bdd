## Required fixture lifecycle protocol

Change A to return 1 and B to return 2 in services/calc/calc.go. Keep both functions and make no other service change.
Never modify tests or documents. Only services/calc/ is writable.

This fixture deliberately exercises a completed writer response without a confirmation. After making the stage's changes, end your final message in prose describing what changed. Do not include any FILE_START or FILE_END marker, and do not write a result file.

## Remaining failing set
{{range .Subject.Tests}}Test: {{.TestName}}
{{end}}

## Proposed repair plan
{{.FixPrompt}}
