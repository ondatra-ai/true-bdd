## Required fixture lifecycle protocol

Read services/calc/calc.go. If it does not contain closureGlossary, replace it with package calc and functions A returning closureGlossary and B returning 0 (closureGlossary deliberately remains undefined). If it already contains closureGlossary, repair A to return 1 and B to return 2. Complete only this stage, even if the proposed plan says more. This controlled fixture deliberately leaves a build failure for the next plan.
Never modify tests or documents. Only services/calc/ is writable.

Fixture transport outcome: non-zero exit
A recording wrapper injects this transport outcome after your turn completes. Make the required file changes and end your final message with nonempty prose describing them, without a confirmation block or result file. Do not try to cause the transport failure yourself.

## Remaining failing set
{{range .Subject.Tests}}Test: {{.TestName}}
{{end}}

## Proposed repair plan
{{.FixPrompt}}
