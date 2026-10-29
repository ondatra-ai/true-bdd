package drill_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/cli/ps"
	"github.com/ondatra-ai/true-bdd/scripts/drill"
)

func TestAnswerCannotCiteSymlinkOutsideSpecification(t *testing.T) {
	runner, record, cut := prepare(t, `#!/bin/sh
printf '\nCLARIFICATION NEEDED\n[1/1] Question?\n1) Follow Specification\n0) Other\n\n    Your answer: '
read answer
`)
	outside := filepath.Join(t.TempDir(), "secret")
	put(t, outside, "not specification")
	must(t, os.Symlink(outside, filepath.Join(cut.Directory, "docs/product/escape")))
	run, err := runner.Prepare(t.Context(), record.Directory, "a", "citation",
		[]string{buildCommand, codeArgument, fixArgument})
	must(t, err)

	ended := make(chan error, 1)
	go func() { ended <- runner.Supervise(t.Context(), record.Directory, run.Tag) }()

	paused := waitRun(t, record.Directory, run.Tag, func(run drill.Attempt) bool { return run.Status == pausedStatus })
	data,
		err := json.Marshal(drill.Answer{Answer: "copy it",
		Document: "docs/product/escape",
		Passage:  "not specification"})
	must(t, err)
	put(t, paused.Questions[0].AnswerFile, string(data))

	err = <-ended
	if err == nil {
		t.Fatal("outside citation accepted")
	}

	final, err := drill.Read(record.Directory)
	must(t, err)

	if !strings.Contains(final.Runs[run.Tag].Reason, "Specification") {
		t.Fatalf("reason: %s", final.Runs[run.Tag].Reason)
	}
}

func TestClarificationCitesOnlyKeptInterfacePassages(t *testing.T) {
	for _, citation := range []struct {
		name     string
		document string
		passage  string
		allowed  bool
		reason   string
	}{
		{"declared", cliDocument, interfacePassage, true, ""},
		{"absent-passage", cliDocument, "The service deletes all numbers.", false, "cited passage does not occur"},
		{"kept-non-specification", testChannel, "seed", false, "outside Specification"},
	} {
		t.Run(citation.name, func(t *testing.T) {
			runner, record := interfaceFixture(t, `#!/bin/sh
printf '\nCLARIFICATION NEEDED\n[1/1] What does the interface accept?\n1) A number\n0) Other\n\n    Your answer: '
read answer
[ "$answer" = 'A number' ]
`)
			cut, err := runner.Cut(t.Context(), record.Directory, "d")
			must(t, err)
			t.Cleanup(func() { _ = os.RemoveAll(cut.Directory) })

			runner.Poll = 10 * time.Millisecond
			runner.Grace = 20 * time.Millisecond
			run, err := runner.Prepare(t.Context(), record.Directory, "d", "interface-answer",
				[]string{buildCommand, stepsArgument, fixArgument})
			must(t, err)

			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()

			ended := make(chan error, 1)
			go func() { ended <- runner.Supervise(ctx, record.Directory, run.Tag) }()

			paused := waitRun(t, record.Directory, run.Tag, func(run drill.Attempt) bool { return run.Status == pausedStatus })
			data, err := json.Marshal(drill.Answer{Answer: "A number", Document: citation.document, Passage: citation.passage})
			must(t, err)
			put(t, paused.Questions[0].AnswerFile, string(data))

			err = <-ended
			if (err == nil) != citation.allowed {
				t.Fatalf("citation accepted=%t: %v", citation.allowed, err)
			}

			if err != nil && !strings.Contains(err.Error(), citation.reason) {
				t.Fatalf("citation failed for wrong reason: %v", err)
			}

			final, err := drill.Read(record.Directory)
			must(t, err)

			saved := final.Runs[run.Tag]
			if citation.allowed {
				assertInterfaceAnswer(t, saved, citation.document, citation.passage)
			} else if saved.Status != "failed" || saved.Questions[0].Answer != "" {
				t.Fatalf("invalid citation answered: %+v", saved)
			}
		})
	}
}

func assertInterfaceAnswer(t *testing.T, saved drill.Attempt, document, passage string) {
	t.Helper()

	if saved.ExitCode == nil || *saved.ExitCode != 0 || saved.Questions[0].Document != document ||
		saved.Questions[0].Passage != passage || saved.Questions[0].Answer != "A number" {
		t.Fatalf("answer or citation lost: %+v", saved)
	}
}

func TestLimitClosesBlockedAnswerPipe(t *testing.T) {
	runner, record, _ := prepare(t, `#!/bin/sh
exec 0<&-
printf 'Your choice (1/2/3): '
sleep 0.05
printf 'Your choice (1/2/3): '
sleep 30
`)
	runner.Limit = 200 * time.Millisecond
	run, err := runner.Prepare(t.Context(), record.Directory, "a", "closed-input",
		[]string{buildCommand, stepsArgument, fixArgument})
	must(t, err)

	err = runner.Supervise(t.Context(), record.Directory, run.Tag)
	if err == nil {
		t.Fatal("engine ignored the command limit")
	}

	final, err := drill.Read(record.Directory)
	must(t, err)

	if final.Runs[run.Tag].EndedAt.IsZero() {
		t.Fatal("blocked answer prevented cleanup")
	}
}

func TestEngineExitEndsUnansweredClarificationAndCleansUp(t *testing.T) {
	runner, record, _ := prepare(t, `#!/bin/sh
printf '\nCLARIFICATION NEEDED\n[1/1] What is specified?\n1) Follow Specification\n0) Other\n\n    Your answer: '
sleep 0.2
exec "$DRILL_TEST_BINARY" -test.run=^TestDrillProcessFixture$
`)
	t.Setenv("DRILL_TEST_BINARY", os.Args[0])
	t.Setenv("DRILL_PROCESS_MODE", "parent")
	childFile := filepath.Join(t.TempDir(), "child")
	t.Setenv("DRILL_CHILD_FILE", childFile)

	runner.Limit = 10 * time.Second
	run, err := runner.Prepare(t.Context(), record.Directory, "a", "unanswered",
		[]string{buildCommand, codeArgument, fixArgument})
	must(t, err)

	began := time.Now()

	must(t, runner.Supervise(t.Context(), record.Directory, run.Tag))

	if time.Since(began) >= 5*time.Second {
		t.Fatal("engine exit waited for the deadline")
	}

	_, err = os.Stat(childFile)
	must(t, err)

	final, err := drill.Read(record.Directory)
	must(t, err)

	saved := final.Runs[run.Tag]
	assertUnansweredQuestion(t, saved)

	if saved.Status != doneStatus || saved.Reason != "" || saved.EndedAt.IsZero() ||
		saved.ExitCode == nil || *saved.ExitCode != 0 || saved.CleanupError != "" {
		t.Fatalf("exit did not finalize cleanup: %+v", saved)
	}

	processes, err := ps.Marked(t.Context(), "TRUE_BDD_DRILL_RUN", []string{run.Marker})
	must(t, err)

	if len(processes) != 0 {
		t.Fatalf("marked processes remain: %v", processes)
	}
}

func assertUnansweredQuestion(t *testing.T, saved drill.Attempt) {
	t.Helper()

	if len(saved.Questions) != 1 || saved.Questions[0].Answer != "" || !saved.Questions[0].AnsweredAt.IsZero() {
		t.Fatalf("question was not retained unanswered: %+v", saved.Questions)
	}
}

func TestEngineExitPreservesOutcomeWithPendingQuestion(t *testing.T) {
	for _, outcome := range []struct {
		code   string
		status string
	}{
		{"0", doneStatus},
		{"7", "failed"},
	} {
		t.Run(outcome.status, func(t *testing.T) {
			runner, record, _ := prepare(t, `#!/bin/sh
printf 'What is specified?\n1) Follow Specification\n0) Other\n'
while [ ! -f "$DRILL_EXIT_FILE" ]; do sleep 0.01; done
exit "$DRILL_EXIT_CODE"
`)
			exitFile := filepath.Join(t.TempDir(), "exit")
			t.Setenv("DRILL_EXIT_FILE", exitFile)
			t.Setenv("DRILL_EXIT_CODE", outcome.code)
			run, err := runner.Prepare(t.Context(), record.Directory, "a", "pending-exit",
				[]string{buildCommand, codeArgument, fixArgument})
			must(t, err)

			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()

			ended := make(chan error, 1)
			go func() { ended <- runner.Supervise(ctx, record.Directory, run.Tag) }()

			waitRun(t, record.Directory, run.Tag, func(run drill.Attempt) bool { return run.Status == pausedStatus })
			put(t, exitFile, "exit")
			must(t, <-ended)

			final, err := drill.Read(record.Directory)
			must(t, err)

			saved := final.Runs[run.Tag]
			assertUnansweredQuestion(t, saved)

			if saved.Status != outcome.status || saved.Reason != "" || saved.EndedAt.IsZero() ||
				saved.ExitCode == nil || strconv.Itoa(*saved.ExitCode) != outcome.code {
				t.Fatalf("pending question changed engine outcome: %+v", saved)
			}
		})
	}
}
