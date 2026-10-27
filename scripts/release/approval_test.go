package release_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ondatra-ai/true-bdd/scripts/release"
)

func TestLiveRehearsalAnswersEachFixMenuThroughStdin(t *testing.T) {
	const menus = `test "$*" = 'build code --fix' || exit 21
for round in 1 2; do
 printf 'Your choice (1/'
 printf '2/3): '
 read -r answer || exit 22
 test "$answer" = 1 || exit 23
done
echo BUILD CODE COMPLETE
exit 0`

	report, err := rehearseLiveEngine(t, menus)
	if err != nil || !report.Passed || !report.Hosts[0].Passed {
		t.Fatalf("live menus: report=%+v, error=%v", report, err)
	}
}

func TestLiveRehearsalLeavesClarificationAtEndOfInput(t *testing.T) {
	const question = `printf 'Your choice (1/2/3): '
read -r answer || exit 22
test "$answer" = 1 || exit 23
printf '\nCLARIFICATION NEEDED\nWhich meaning?\n1) First\n0) Other\nYour answer: '
if read -r answer; then echo 'guessed clarification'; exit 24; fi
echo 'a person must answer the clarifying question'
exit 1`

	report, err := rehearseLiveEngine(t, question)
	if err == nil || report.Passed {
		t.Fatalf("clarification accepted: %+v, %v", report, err)
	}

	data, readErr := os.ReadFile(report.Hosts[0].Transcript)
	if readErr != nil {
		t.Fatal(readErr)
	}

	if !strings.Contains(string(data), "a person must answer") || strings.Contains(string(data), "guessed clarification") {
		t.Fatalf("clarification transcript: %s", data)
	}
}

func rehearseLiveEngine(t *testing.T, interaction string) (release.Report, error) {
	t.Helper()
	root := sourceRepository(t)
	engine := strings.Replace(installedEngine, "claude\nexit 1", interaction, 1)
	artifacts := artifactSet(t, engine)

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	report, err := release.Rehearse(ctx, root, artifacts, release.Options{Live: true})

	t.Cleanup(func() { _ = os.RemoveAll(report.Root) })

	if err != nil {
		return report, fmt.Errorf("live rehearsal: %w", err)
	}

	return report, nil
}
