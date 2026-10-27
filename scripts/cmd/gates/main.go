// Command gates runs this repository's quality pipeline.
//
// Invoked by CI, by the git pre-commit hook (.githooks/pre-commit) and by
// hand. The table lives in scripts/gates — this is only its CLI.
//
//	gates run                    every gate
//	gates run --changed main     only the gates the diff against main needs
//	gates run --staged           only the gates the staged change needs
//	gates release-rehearsal       the zero-cost release rehearsal alone
//
//	gates recordings             the Recordings gate alone
//	gates engine-imports         enforce engine ownership without compiling
//	gates list                   the gate names, one per line
package main

import (
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/ondatra-ai/true-bdd/pkg/cli/git"
	"github.com/ondatra-ai/true-bdd/pkg/logging"
	"github.com/ondatra-ai/true-bdd/scripts/gates"
)

var (
	errNoCommand = errors.New(
		"usage: gates run [--changed <base> | --staged] | recordings | release-rehearsal | engine-imports | list")
	errTwoScopes = errors.New("--changed and --staged are exclusive")
)

func main() {
	logging.Install(logging.Stderr, "", "gates")

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
	case "release-rehearsal":
		return gates.ReleaseRehearsal() //nolint:wrapcheck // the gate names the failed install step.
	case "engine-imports":
		return gates.CheckEngineImports(".") //nolint:wrapcheck // the gate names each offending source and import.
	case "recordings":
		return gates.ScanRecordings() //nolint:wrapcheck // the gate's own diagnosis.
	case "run":
		return runGates(args[1:])
	default:
		return errNoCommand
	}
}

func runGates(args []string) error {
	set := flag.NewFlagSet("run", flag.ContinueOnError)
	base := set.String("changed", "", "run only the gates a diff against this ref needs")
	staged := set.Bool("staged", false, "run only the gates the staged change needs")

	err := set.Parse(args)
	if err != nil {
		return fmt.Errorf("parsing flags: %w", err)
	}

	selected, err := selection(*base, *staged)
	if err != nil {
		return err
	}

	err = gates.Run(selected)
	if err != nil {
		return fmt.Errorf("running the pipeline: %w", err)
	}

	return nil
}

// selection is the whole table, or the gates one set of changed paths needs.
func selection(base string, staged bool) ([]gates.Gate, error) {
	var (
		changed []string
		err     error
	)

	switch {
	case base != "" && staged:
		return nil, errTwoScopes
	case base != "":
		changed, err = gates.Changed(base)
	case staged:
		changed, err = git.StagedPaths()
	default:
		return gates.All, nil
	}

	if err != nil {
		return nil, fmt.Errorf("selecting gates: %w", err)
	}

	selected := gates.Select(changed)

	slog.Info("Gates selected by the change",
		"selected", len(selected), "total", len(gates.All), "changed", len(changed))

	return selected, nil
}
