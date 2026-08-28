// Command gates runs this repository's quality pipeline.
//
// Invoked by scripts/commit and by hand; CI runs the same table step by step.
// The table lives in scripts/gates — this is only its CLI.
//
//	gates run                    every gate
//	gates run --changed main     only the gates the diff against main needs
//	gates list                   the gate names, one per line
package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/ondatra-ai/true-bdd/scripts/history"
	"github.com/ondatra-ai/true-bdd/scripts/state"
	"os"

	"github.com/ondatra-ai/true-bdd/pkg/logging"
	"github.com/ondatra-ai/true-bdd/scripts/gates"
	"log/slog"
)

var errNoCommand = errors.New("usage: gates run [--changed <base>] | gates list")

func main() {
	logging.Install(logging.Stderr, state.ToolLog(history.RepoRoot()), "gates")

	err := run(os.Args[1:])
	if err != nil {
		slog.Error("gates failed", "error", err)
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
			slog.Info("Gate", "name", gate.Name)
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

		slog.Info("Gates selected by the diff",
			"selected", len(selected), "total", len(gates.All),
			"changed", len(changed), "base", *base)
	}

	err = gates.Run(selected)
	if err != nil {
		return fmt.Errorf("running the pipeline: %w", err)
	}

	return nil
}
