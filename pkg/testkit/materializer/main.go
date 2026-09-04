package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ondatra-ai/true-bdd/pkg/console"
	"github.com/ondatra-ai/true-bdd/pkg/logging"
	"github.com/ondatra-ai/true-bdd/pkg/testkit/runner"
	"log/slog"
)

// ErrUsage is returned for invalid flag combinations; see doc.go for
// the CLI contract.
var ErrUsage = errors.New(
	"usage: materializer -fixture <dir> -target <dir> [-repo <dir>] | -list-baseline -target <dir>")

func main() {
	logging.Install(logging.Stderr, "", "materializer")

	fixtureDir := flag.String("fixture", "", "fixture directory containing fixture.yaml")
	targetDir := flag.String("target", "", "directory to materialize into (or hash with -list-baseline)")
	repoRoot := flag.String("repo", "", "repo root supplying the engine layer (default: walk up from cwd)")
	listBaseline := flag.Bool("list-baseline", false, "hash an existing -target tree and print JSON; skip materialization")

	flag.Parse()

	err := run(*fixtureDir, *targetDir, *repoRoot, *listBaseline, console.Out())
	if err != nil {
		slog.Error("materializer failed", "error", err)
		os.Exit(1)
	}
}

func run(fixtureDir, targetDir, repoRoot string, listBaseline bool, out *os.File) error {
	if targetDir == "" {
		return fmt.Errorf("%w: -target is required", ErrUsage)
	}

	if listBaseline {
		return printBaseline(targetDir, out)
	}

	if fixtureDir == "" {
		return fmt.Errorf("%w: -fixture is required", ErrUsage)
	}

	if repoRoot == "" {
		// Best-effort default; Materialize enforces presence only for
		// base: engine fixtures.
		repoRoot, _ = runner.FindRepoRoot()
	}

	result, err := Materialize(Options{
		FixtureDir: fixtureDir,
		TargetDir:  targetDir,
		RepoRoot:   repoRoot,
	})
	if err != nil {
		return err
	}

	return encodeJSON(out, result)
}

// baselineOnly is the -list-baseline output document.
type baselineOnly struct {
	Target   string            `json:"target"`
	Baseline map[string]string `json:"baseline"`
}

func printBaseline(targetDir string, out *os.File) error {
	target, err := filepath.Abs(targetDir)
	if err != nil {
		return fmt.Errorf("resolve target %s: %w", targetDir, err)
	}

	baseline, err := HashTree(target)
	if err != nil {
		return fmt.Errorf("tree hash: %w", err)
	}

	return encodeJSON(out, &baselineOnly{Target: target, Baseline: baseline})
}

func encodeJSON(out *os.File, doc any) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")

	err := encoder.Encode(doc)
	if err != nil {
		return fmt.Errorf("encode result JSON: %w", err)
	}

	return nil
}
