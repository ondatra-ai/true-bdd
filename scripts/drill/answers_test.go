package drill_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ondatra-ai/true-bdd/scripts/drill"
)

func TestAnswerCannotCiteSymlinkOutsideSpecification(t *testing.T) {
	runner, record, cut := prepare(t, `#!/bin/sh
printf '\nCLARIFICATION NEEDED\n[1/1] Question?\n\n    Your answer: '
read answer
`)
	outside := filepath.Join(t.TempDir(), "secret")
	put(t, outside, "not specification")
	must(t, os.Symlink(outside, filepath.Join(cut.Directory, "docs/product/escape")))
	run, err := runner.Prepare(t.Context(), record.Directory, "a", "citation", []string{buildCommand})
	must(t, err)

	ended := make(chan error, 1)
	go func() { ended <- runner.Supervise(t.Context(), record.Directory, run.Tag) }()

	paused := waitRun(t, record.Directory, run.Tag, func(run drill.Run) bool { return run.Status == pausedStatus })
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

func TestLimitClosesBlockedAnswerPipe(t *testing.T) {
	runner, record, _ := prepare(t, `#!/bin/sh
exec 0<&-
printf 'Your choice (1/2/3): '
sleep 0.05
printf 'Your choice (1/2/3): '
sleep 30
`)
	runner.Limit = 200 * time.Millisecond
	run, err := runner.Prepare(t.Context(), record.Directory, "a", "closed-input", []string{buildCommand})
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
