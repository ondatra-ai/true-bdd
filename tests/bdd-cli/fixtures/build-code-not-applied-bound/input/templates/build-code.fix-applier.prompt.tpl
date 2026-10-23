## Production repair protocol

Read services/calc/prepared-first.txt and services/calc/prepared-second.txt before applying this repair.
If prepared-first.txt is absent, create it containing exactly `prepared` followed by a newline, leave calc.go unchanged, and report applied: false with summary "first preparation only; test still fails".
Otherwise if prepared-second.txt is absent, create it containing exactly `prepared` followed by a newline, leave calc.go unchanged, and report applied: false with summary "second preparation only; test still fails".
If both files exist, apply the fix below to services/calc/calc.go, keep both preparation files, and report applied: true.
Never modify tests or documents. Only services/calc/ is writable.

{{.FixPrompt}}

Return this confirmation with the correct boolean:
=== FILE_START: {{.ResultPath}} ===
applied: <true or false>
target: "services/calc/calc.go"
summary: "<what happened>"
=== FILE_END: {{.ResultPath}} ===
