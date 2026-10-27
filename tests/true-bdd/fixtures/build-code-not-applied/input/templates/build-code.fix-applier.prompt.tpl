## Production repair protocol

Read services/calc/prepared.txt before applying this repair.
If that file is absent, create it containing exactly `prepared` followed by a newline, change calc.go so Add returns a+b+1 (still failing), and report applied: false with summary "preparation only; test still fails".
If that file exists, apply the fix below to services/calc/calc.go, keep prepared.txt, and report applied: true.
Never modify tests or documents. Only services/calc/ is writable.

{{.FixPrompt}}

Return this confirmation with the correct boolean:
=== FILE_START: {{.ResultPath}} ===
applied: <true or false>
target: "services/calc/calc.go"
summary: "<what happened>"
=== FILE_END: {{.ResultPath}} ===
