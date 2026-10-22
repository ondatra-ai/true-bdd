// Command codex starts or resumes one supervised delegation attempt.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/ondatra-ai/true-bdd/pkg/cli/git"
	"github.com/ondatra-ai/true-bdd/pkg/logging"
	"github.com/ondatra-ai/true-bdd/scripts/codexrun"
)

const refusedExit = 2

var (
	errUsage = errors.New("usage: codex start --kind <kind> --name <run> --brief <file>; " +
		"resume --name <run> --note <reason>")
	errPositional = errors.New("unexpected positional arguments")
)

func main() {
	logging.Install(logging.Stderr, "", "codex")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)

	code := run(ctx, os.Args[1:])

	stop()
	os.Exit(code)
}

func run(ctx context.Context, args []string) int {
	record, err := command(ctx, args)
	if err != nil {
		slog.Error("Codex command refused", "error", err)

		return refusedExit
	}

	slog.Info("Codex summary", "summary", record.Summary())

	if record.Status != "done" {
		return 1
	}

	return 0
}

func command(ctx context.Context, args []string) (codexrun.Record, error) {
	if len(args) == 0 || args[0] != "start" && args[0] != "resume" {
		return codexrun.Record{}, errUsage
	}

	set := flag.NewFlagSet(args[0], flag.ContinueOnError)
	set.SetOutput(io.Discard)
	name := set.String("name", "", "new run name, or the run to resume")

	var (
		opts codexrun.Options
		note string
	)

	if args[0] == "start" {
		startFlags(set, &opts)
	} else {
		set.StringVar(&note, "note", "", "what went wrong")
	}

	err := set.Parse(args[1:])
	if err != nil {
		return codexrun.Record{}, fmt.Errorf("parsing flags: %w", err)
	}

	if set.NArg() != 0 {
		return codexrun.Record{}, errPositional
	}

	root, err := git.TopLevel()
	if err != nil {
		return codexrun.Record{}, err
	}

	runner := codexrun.New(root)

	var record codexrun.Record
	if args[0] == "resume" {
		record, err = runner.Resume(ctx, *name, note)
	} else {
		opts.Name = *name
		record, err = runner.Start(ctx, opts)
	}

	if err != nil {
		return record, fmt.Errorf("running codex: %w", err)
	}

	return record, nil
}

func startFlags(set *flag.FlagSet, opts *codexrun.Options) {
	set.StringVar(&opts.Kind, "kind", "", "plan-read, review, research, code or fix")
	set.StringVar(&opts.Brief, "brief", "", "brief file")
	set.StringVar(&opts.Branch, "branch", "", "new code branch, or a code run's branch to fix")
	set.StringVar(&opts.Base, "base", "", "code run's starting commit or branch")
	set.BoolVar(&opts.Search, "search", false, "enable web search")
	set.Func("issue", "issue number to save (repeatable)", func(value string) error {
		for _, existing := range opts.Issues {
			if strings.TrimSpace(value) == existing {
				return nil
			}
		}

		opts.Issues = append(opts.Issues, value)

		return nil
	})
}
