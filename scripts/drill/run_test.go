package drill_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/cli/ps"
	"github.com/ondatra-ai/true-bdd/scripts/drill"
)

const fixArgument = "--fix"

func prepare(t *testing.T, script string) (drill.Runner, drill.Record, drill.Cut) {
	t.Helper()
	runner, next := fixtureEngine(t, script)
	cut, err := runner.Cut(t.Context(), next.Directory, "a")
	must(t, err)

	t.Cleanup(func() { _ = os.RemoveAll(cut.Directory) })

	runner.Grace = 20 * time.Millisecond
	runner.Poll = 10 * time.Millisecond

	return runner, next, cut
}

func TestMenuAndClarificationWaitForSpecificationAnswer(t *testing.T) {
	runner, record, _ := prepare(t, `#!/bin/sh
printf 'Your choice (1/2/3): '
read menu
[ "$menu" = 1 ] || exit 12
printf '\nCLARIFICATION NEEDED\n[1/1] How is it specified?\n    Options:\n'
printf '      1) Restore from history\n      2) Write from specification\n\n    Your answer: '

read answer
[ "$answer" = 'Write from specification' ] || exit 13
`)

	run, err := runner.Prepare(t.Context(), record.Directory, "a", "menu",
		[]string{buildCommand, codeArgument, fixArgument})
	must(t, err)

	ended := make(chan error, 1)
	go func() { ended <- runner.Supervise(t.Context(), record.Directory, run.Tag) }()

	paused := waitRun(t, record.Directory, run.Tag, func(run drill.Attempt) bool { return run.Status == pausedStatus })
	if len(paused.Questions) != 1 {
		t.Fatalf("questions: %+v", paused.Questions)
	}

	if !strings.Contains(paused.Questions[0].Prompt, "Restore from history") {
		t.Fatal("options lost")
	}

	select {
	case err := <-ended:
		t.Fatalf("did not wait: %v", err)
	default:
	}

	answer := drill.Answer{Answer: "Write from specification", Document: testProduct, Passage: "seed"}

	data, err := json.Marshal(answer)
	must(t, err)

	put(t, paused.Questions[0].AnswerFile, string(data))

	must(t, <-ended)

	current, err := drill.Read(record.Directory)
	must(t, err)

	final := current.Runs[run.Tag]
	if final.Status != "done" || final.ExitCode == nil || *final.ExitCode != 0 || final.EndedAt.IsZero() {
		t.Fatalf("final: %+v", final)
	}

	if final.Questions[0].Answer != answer.Answer {
		t.Fatal("answer not retained")
	}
}

func TestEveryTerminalPathCleansMarkedProcesses(t *testing.T) {
	for _, mode := range []string{"normal", "failure", "limit", stopMode, "cancel", launchMode} {
		t.Run(mode, func(t *testing.T) {
			script := terminalScript(mode)

			runner, record, _ := prepare(t, script)
			if mode == "limit" {
				runner.Limit = 50 * time.Millisecond
			}

			run, err := runner.Prepare(t.Context(), record.Directory, "a", mode,
				[]string{buildCommand, codeArgument, fixArgument})
			must(t, err)

			if mode == launchMode {
				err := os.Chmod(record.Binary, 0o600)
				must(t, err)
			}

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			if mode == stopMode {
				put(t, filepath.Join(record.Directory, "runs", mode, stopMode), stopMode)
			}

			if mode == "cancel" {
				cancel()
			}

			_ = runner.Supervise(ctx, record.Directory, run.Tag)

			current, err := readJSON(record.Directory)
			must(t, err)

			final := current.Runs[run.Tag]
			if final.EndedAt.IsZero() || final.ExitCode == nil || final.CleanupError != "" {
				t.Fatalf("not finalized after cleanup: %+v", final)
			}

			processes, err := ps.Marked(t.Context(), "TRUE_BDD_DRILL_RUN", []string{run.Marker})
			must(t, err)

			if len(processes) != 0 {
				t.Fatalf("processes remain: %v", processes)
			}
		})
	}
}

func readJSON(directory string) (drill.Record, error) {
	data, err := os.ReadFile(filepath.Join(directory, "record.json"))
	if err != nil {
		return drill.Record{}, fmt.Errorf("reading test record: %w", err)
	}

	var record drill.Record

	err = json.Unmarshal(data, &record)
	if err == nil {
		return record, nil
	}

	return record, fmt.Errorf("decoding test record: %w", err)
}

func terminalScript(mode string) string {
	switch mode {
	case "normal", launchMode:
		return "#!/bin/sh\nexit 0\n"
	case "failure":
		return "#!/bin/sh\nexit 7\n"
	default:
		return "#!/bin/sh\nsleep 30\n"
	}
}

func TestRebuildCommandsAreExplicit(t *testing.T) {
	for _, command := range []struct {
		args    []string
		allowed bool
	}{
		{[]string{buildCommand, "tests", fixArgument}, true},
		{[]string{buildCommand, codeArgument, fixArgument}, true},
		{[]string{buildCommand}, false},
		{[]string{buildCommand, codeArgument}, false},
		{[]string{"scen", "check", fixArgument}, false},
		{[]string{buildCommand, "tests", fixArgument, "--help"}, false},
		{[]string{"oracle"}, false},
	} {
		t.Run(strings.Join(command.args, " "), func(t *testing.T) {
			runner, record, _ := prepare(t, "#!/bin/sh\n[ \"$*\" = '"+strings.Join(command.args, " ")+"' ]\n")

			attempt, err := runner.Prepare(t.Context(), record.Directory, "a", "allowed-command", command.args)
			if (err == nil) != command.allowed {
				t.Fatalf("Prepare(%q): %v", command.args, err)
			}

			if command.allowed {
				must(t, runner.Supervise(t.Context(), record.Directory, attempt.Tag))
			}
		})
	}
}
