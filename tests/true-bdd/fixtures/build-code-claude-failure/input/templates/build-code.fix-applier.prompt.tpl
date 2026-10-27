## Required fixture protocol
Read services/calc/calc.go. Change A to return 1 and B to return 2. Keep both functions and make no other service change. Never modify tests or documents.
After the repair you MUST use Bash to run this exact foreground selection in {{.TestCommand.Dir}}:
{{.TestCommand.Command}} {{.TestCommand.Selector}} '^Test(A|B)$'
Wait for its passing result. Do not use any other command or background execution.

{{.FixPrompt}}

End your final message with:
=== FILE_START: {{.ResultPath}} ===
applied: true
target: "services/calc/calc.go"
summary: "Fixed A and B and ran the granted selection."
=== FILE_END: {{.ResultPath}} ===

Fixture transport outcome: non-zero exit
The recording wrapper runs this turn unchanged and then injects exit 7. Do not cause a failure yourself.
