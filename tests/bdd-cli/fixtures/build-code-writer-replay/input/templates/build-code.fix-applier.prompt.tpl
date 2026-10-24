## Production repair and replay protocol

{{.FixPrompt}}

Repair services/calc/calc.go so Add sums its arguments.
Then run exactly this bash command ONCE, with working_dir set to the project root:
go test -v -count=1 ./tests/suite/... -run '^TestSlowReplay$'

Set the bash input run_in_background to false explicitly. Do not proactively
background it: this fixture verifies crush's automatic one-minute handoff.
The selection intentionally takes 65 seconds. Crush backgrounds it after one minute.
Use job_output on the returned job ID until the job is complete. Read the completed
output before returning; it must show SELECTION COMPLETED and PASS.
Do not use any other shell command. Never edit tests or documents.

=== FILE_START: {{.ResultPath}} ===
applied: true
target: "services/calc/calc.go"
summary: "Repaired Add and observed the selected replay complete through job_output"
=== FILE_END: {{.ResultPath}} ===
