## Required fixture lifecycle protocol

Read services/calc/calc.go. If A returns 0, change only A to return 1 and leave B returning 0, even if the plan proposes both. Otherwise change B to return 2 and preserve A. Complete only that stage in this apply.
Never modify tests or documents. Only services/calc/ is writable.

This fixture deliberately exercises a completed writer response without a confirmation. After making the stage's changes, end your final message in prose describing what changed. Do not include any FILE_START or FILE_END marker, and do not write a result file.

## Remaining failing set
{{range .Subject.Tests}}Test: {{.TestName}}
{{end}}

## Proposed repair plan
{{.FixPrompt}}
