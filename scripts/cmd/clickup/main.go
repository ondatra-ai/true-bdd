// Command clickup files deferred findings as ClickUp tasks.
//
// It is the single ClickUp interface: the merge loop calls the package and
// the fix-queue skill calls this binary by path, and both get the same
// four-heading ticket shape.
//
//	clickup render --queue tmp/merge/defer-queue.json --tag fix-now --pr 76
//	clickup file   --queue tmp/merge/defer-queue.json --tag fix-now --pr 76
//	clickup list   --tag fix-now
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ondatra-ai/true-bdd/scripts/clickup"
)

const usage = `usage:
  clickup render --queue <path> --tag <tag> [--pr <number>]
  clickup file   --queue <path> --tag <tag> [--pr <number>]
  clickup list   --tag <tag>
`

func main() {
	err := run(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
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
	case "list":
		return runList(command, rest)
	default:
		return fmt.Errorf("%w: %q\n%s", errUnknownCommand, command, usage)
	}
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

	err = clickup.File(os.Stdout, os.Stderr, queue, tag, pullRequest)
	if err != nil {
		return fmt.Errorf("filing the queue: %w", err)
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

	err = clickup.List(os.Stdout, *tag)
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

	_, _ = fmt.Fprintf(os.Stdout, "%d ticket(s) -> %s\n", len(queue), clickup.TicketsMarkdown)

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
