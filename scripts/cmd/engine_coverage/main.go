// Command engine_coverage reports the engine's code no Scenario exercises. It
// builds services/bdd-cli with coverage counters, replays the whole BDD suite
// against that binary (`-engine-bin`), and writes every package's coverage and
// every block no run reached as Markdown. Run from the repository root:
//
//	go run ./scripts/cmd/engine_coverage            report at tmp/engine-coverage.md
//	go run ./scripts/cmd/engine_coverage -o <path>  report at path
//
// A red suite is a refusal, not a report. The library is scripts/enginecoverage.
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/cli"
	"github.com/ondatra-ai/true-bdd/pkg/cli/spec"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/pkg/logging"
	"github.com/ondatra-ai/true-bdd/scripts/enginecoverage"
)

const (
	// tool names this program in its log records.
	tool = "engine_coverage"
	// defaultReport is gitignored scratch, like every other run artefact.
	defaultReport = "tmp/engine-coverage.md"
	// suiteTimeout caps the replay, which takes well under a minute.
	suiteTimeout = 15 * time.Minute
	// headlinePackages is how many of the least covered the log names.
	headlinePackages = 5
)

func main() {
	logging.Install(logging.Stderr, "", tool)

	err := run(os.Args[1:])
	if err != nil {
		slog.Error("engine coverage failed", "error", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	set := flag.NewFlagSet(tool, flag.ContinueOnError)
	out := set.String("o", defaultReport, "where the Markdown report is written")

	err := set.Parse(args)
	if err != nil {
		return fmt.Errorf("parsing flags: %w", err)
	}

	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve the repository root: %w", err)
	}

	report, err := enginecoverage.Measure(enginecoverage.Plan{
		Root:     root,
		Main:     "./services/bdd-cli",
		Packages: "./services/bdd-cli/...",
		Exercise: replaySuite(root),
	})
	if err != nil {
		return fmt.Errorf("measure the engine: %w", err)
	}

	path := *out
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}

	err = disk.Write(path, report.Markdown(), disk.Shared)
	if err != nil {
		return fmt.Errorf("write the report: %w", err)
	}

	logHeadline(report, path)

	return nil
}

// replaySuite runs every Scenario in replay against the covered binary. The
// counters reach it through the environment the harness hands each engine run.
func replaySuite(root string) func(binPath string, env []string) error {
	return func(binPath string, env []string) error {
		argv := []string{
			"go", "test", "-count=1", "-tags", "bdd", "./tests/bdd-cli/",
			"-mode=services:replay,tests:replay", "-engine-bin=" + binPath,
		}

		result, err := spec.Run(argv, cli.Options{
			Dir: root, Env: cli.Inherit().Set(env...), Output: cli.Console(), Timeout: suiteTimeout,
		})
		if err != nil {
			return fmt.Errorf("replay the suite: %w", err)
		}

		err = result.Err()
		if err != nil {
			return fmt.Errorf("the replay suite failed: %w", err)
		}

		return nil
	}
}

// logHeadline names the total and the least covered packages; the report
// holds the rest.
func logHeadline(report enginecoverage.Report, path string) {
	slog.Info("Engine coverage",
		"coverage", enginecoverage.Ratio(report.Covered(), report.Statements()),
		"covered", report.Covered(), "statements", report.Statements(),
		"uncovered_blocks", report.UncoveredBlocks(), "report", path)

	for _, pkg := range report.Packages[:min(headlinePackages, len(report.Packages))] {
		slog.Info("Least covered",
			"package", pkg.Path, "coverage", enginecoverage.Ratio(pkg.Covered, pkg.Statements),
			"covered", pkg.Covered, "statements", pkg.Statements)
	}
}
