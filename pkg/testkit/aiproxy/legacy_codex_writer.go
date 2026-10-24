package main

import "strings"

// #201 preserves the exact old routing cassette; new fixtures prove confinement live.
const legacyCodexWriterHash = "sha256:fb2c35fa5e2a4c08b5ab165a5fd9e8310e213d20c76645f95d7d4a1164a24d43"

func replayExpectation(manifest *meta, stdin string) ([]string, string, string) {
	if manifest.Binary != "codex" || manifest.RequestHash != legacyCodexWriterHash ||
		requestHash(manifest.Argv, []byte(stdin), cwdPlaceholder) != legacyCodexWriterHash {
		return manifest.Argv, stdin, manifest.RequestHash
	}

	args := append([]string{manifest.Argv[0]}, manifest.Argv[3:7]...)
	args = append(args, "--strict-config", "--ignore-rules", "--json")
	args = append(args, manifest.Argv[7:]...)
	prompt := strings.Replace(stdin,
		"No test command is granted to this writer. Report that limitation in the confirmation.",
		legacyCodexReplayForms, 1)

	return args, prompt, requestHash(args, []byte(prompt), cwdPlaceholder)
}

const legacyCodexReplayForms = "## Test commands\n" +
	"Run in `{{CWD}}` using exactly one of these forms:\n" +
	"- Whole suite: `go test -count=1 ./tests/suite/...`\n" +
	"- Selection: `go test -count=1 ./tests/suite/... -run '<selection>'`\n" +
	"The selection is a nonempty framework regular expression. Keep the command in the foreground; " +
	"if the tool returns a process/session id, poll that process with your native process tool until it completes."
