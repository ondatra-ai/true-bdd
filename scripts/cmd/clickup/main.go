// Command clickup files deferred findings as ClickUp tasks.
//
// It is the single ClickUp interface: the merge loop calls the package and
// task-handle's grooming reads it back, and both get the same
// four-heading ticket shape.
//
//	clickup render --queue tmp/merge/defer-queue.json --tag fix-now --pr 76
//	clickup file   --queue tmp/merge/defer-queue.json --tag fix-now --pr 76
//	clickup defer  --doc tmp/deferral.md --tag deferred
//	clickup list   --tag fix-now
package main

import (
	"flag"
	"fmt"
	"github.com/ondatra-ai/true-bdd/scripts/history"
	"github.com/ondatra-ai/true-bdd/scripts/state"
	"os"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/logging"
	"github.com/ondatra-ai/true-bdd/scripts/clickup"
	"log/slog"
)

const usage = `usage:
  clickup render --queue <path> --tag <tag> [--pr <number>]
  clickup file   --queue <path> --tag <tag> [--pr <number>]
  clickup defer  --doc <path> --tag <tag>
  clickup list   --tag <tag>
  clickup close  <STATUS> <comment...>
`

func main() {
	logging.Install(logging.Stderr, state.ToolLog(history.RepoRoot()), "clickup")

	err := run(os.Args[1:])
	if err != nil {
		slog.Error("clickup failed", "error", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("%w\n%s", errNoCommand, usage)
	}

	command, rest := args[0], args[1:]

	switch command {
	case "render":
		return runRender(command, rest)
	case "file":
		return runFile(command, rest)
	case "defer":
		return runDefer(command, rest)
	case "list":
		return runList(command, rest)
	case "status":
		return runStatus(rest)
	case "close":
		return runClose(rest)
	default:
		return fmt.Errorf("%w: %q\n%s", errUnknownCommand, command, usage)
	}
}

// runStatus closes the named Ticket, leaving the binding alone. `close` is
// what the /task-* skills call; this is the by-hand form.
func runStatus(args []string) error {
	const wanted = 3
	if len(args) < wanted {
		return fmt.Errorf("%w\n%s", errMissingFlag, usage)
	}

	err := clickup.Status(args[0], args[1], strings.Join(args[2:], " "))
	if err != nil {
		return fmt.Errorf("closing the ticket: %w", err)
	}

	return nil
}

func runRender(command string, args []string) error {
	queue, tag, pullRequest, err := queueFlags(command, args)
	if err != nil {
		return err
	}

	return render(queue, tag, pullRequest)
}

func runFile(command string, args []string) error {
	queue, tag, pullRequest, err := queueFlags(command, args)
	if err != nil {
		return err
	}

	err = clickup.File(queue, tag, pullRequest)
	if err != nil {
		return fmt.Errorf("filing the queue: %w", err)
	}

	return nil
}

// runDefer files a hand-written deferral. The document already carries the
// four headings, so it is transcribed rather than rendered from a finding.
func runDefer(command string, args []string) error {
	set := flag.NewFlagSet(command, flag.ContinueOnError)
	doc := set.String("doc", "", "path to the markdown document (required)")
	tag := set.String("tag", "", "the ClickUp tag to apply (required)")

	err := parse(set, args, doc, tag)
	if err != nil {
		return err
	}

	err = clickup.FileDocument(*doc, *tag)
	if err != nil {
		return fmt.Errorf("filing %s: %w", *doc, err)
	}

	return nil
}

func runList(command string, args []string) error {
	set := flag.NewFlagSet(command, flag.ContinueOnError)
	tag := set.String("tag", "", "the ClickUp tag to list (required)")

	err := parse(set, args, tag)
	if err != nil {
		return err
	}

	err = clickup.List(*tag)
	if err != nil {
		return fmt.Errorf("listing the queue: %w", err)
	}

	return nil
}

func render(queuePath, tag, pullRequest string) error {
	queue, err := clickup.LoadQueue(queuePath)
	if err != nil {
		return fmt.Errorf("loading the queue: %w", err)
	}

	_, err = clickup.WriteRendered(queue, tag, pullRequest)
	if err != nil {
		return fmt.Errorf("rendering the queue: %w", err)
	}

	slog.Info("Tickets rendered", "count", len(queue), "path", clickup.TicketsMarkdown)

	return nil
}

// queueFlags parses the flags `render` and `file` share.
func queueFlags(command string, args []string) (string, string, string, error) {
	set := flag.NewFlagSet(command, flag.ContinueOnError)
	queueFlag := set.String("queue", "", "path to the queue JSON (required)")
	tagFlag := set.String("tag", "", "the ClickUp tag to apply (required)")
	pullRequestFlag := set.String("pr", "", "the pull request the findings came from")

	err := parse(set, args, queueFlag, tagFlag)
	if err != nil {
		return "", "", "", err
	}

	return *queueFlag, *tagFlag, *pullRequestFlag, nil
}

// parse reads the flag set and insists on the ones with no sensible default.
func parse(set *flag.FlagSet, args []string, required ...*string) error {
	err := set.Parse(args)
	if err != nil {
		return fmt.Errorf("parsing the flags of %s: %w", set.Name(), err)
	}

	for _, value := range required {
		if *value == "" {
			set.Usage()

			return fmt.Errorf("%w\n%s", errMissingFlag, usage)
		}
	}

	return nil
}
