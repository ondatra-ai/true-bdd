// Command gates runs this repository's quality pipeline.
//
// Invoked by .claude/skills/pr-commit/gates.sh, which is what CI and every
// human commit run. The table lives in scripts/gates; this is only its CLI.
//
//	gates run                    every gate
//	gates run --changed main     only the gates the diff against main needs
//	gates list                   the gate names, one per line
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/ondatra-ai/true-bdd/scripts/gates"
)

var errNoCommand = errors.New("usage: gates run [--changed <base>] | gates list")

func main() {
	err := run(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errNoCommand
	}

	switch args[0] {
	case "list":
		for _, gate := range gates.All {
			_, _ = fmt.Fprintln(os.Stdout, gate.Name)
		}

		return nil
	case "run":
		return runGates(args[1:])
	default:
		return errNoCommand
	}
}

func runGates(args []string) error {
	set := flag.NewFlagSet("run", flag.ContinueOnError)
	base := set.String("changed", "", "run only the gates a diff against this ref needs")

	err := set.Parse(args)
	if err != nil {
		return fmt.Errorf("parsing flags: %w", err)
	}

	selected := gates.All

	if *base != "" {
		changed, err := gates.Changed(*base)
		if err != nil {
			return fmt.Errorf("selecting gates: %w", err)
		}

		selected = gates.Select(changed)

		_, _ = fmt.Fprintf(os.Stdout, "%d/%d gates for %d changed path(s) vs %s\n",
			len(selected), len(gates.All), len(changed), *base)
	}

	timings, err := gates.Run(os.Stdout, selected)

	gates.RenderSummary(os.Stdout, gates.All, selected, timings)

	if err != nil {
		return fmt.Errorf("running the pipeline: %w", err)
	}

	return nil
}
