## Required fixture lifecycle protocol

Read services/calc/calc.go and services/calc/prepared.txt. If A returns 0, change only A to return 1 and leave B returning 0. Otherwise if prepared.txt is absent, create it containing prepared followed by a newline and leave calc.go unchanged. Otherwise change B to return 2 and preserve A. Complete only that stage in this apply, even if the plan proposes more.
Never modify tests or documents. Only services/calc/ is writable.

This fixture deliberately exercises a completed writer response without a confirmation. After making the stage's changes, end your final message in prose describing what changed. Do not include any FILE_START or FILE_END marker, and do not write a result file.

## Remaining failing set
{{range .Subject.Tests}}Test: {{.TestName}}
{{end}}

## Proposed repair plan
{{.FixPrompt}}

## Fixture cache policy

Use the engine-provided GOCACHE, HOME and TMPDIR unchanged. Do not override cache or scratch environment variables, copy build caches, or create alternative cache directories such as tmp/go-cache or tmp/go-build. Run the granted test command with its supplied environment. This policy changes no repair stage or confirmation protocol.
