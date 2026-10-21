package enginecoverage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/cli"
	"github.com/ondatra-ai/true-bdd/pkg/cli/gotool"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// ErrNoCoverageData marks an exercise that never ran the binary it was handed.
// A report from it would call every block of the engine unspecified.
var ErrNoCoverageData = errors.New("the exercise left no coverage counters: it never ran the covered binary")

const (
	// buildTimeout caps the covered build; a cold one compiles the whole engine.
	buildTimeout = 5 * time.Minute
	// mergeTimeout caps covdata's merge of every run's counters.
	mergeTimeout = 2 * time.Minute
	// counterPrefix names the file a covered binary writes as it exits.
	counterPrefix = "covcounters."
)

// Plan is one measurement: what to build, whose blocks count, and what drives
// the built binary.
type Plan struct {
	// Root is the module directory, absolute; Main and Packages resolve in it.
	Root string
	// Main is the main package built, as `go build` names it.
	Main string
	// Packages is the -coverpkg pattern: the code whose blocks are counted.
	Packages string
	// Exercise drives the covered binary at binPath. Every process it starts
	// must carry env, or its counters land nowhere.
	Exercise func(binPath string, env []string) error
}

// Measure builds Main with coverage on Packages, runs Exercise against it and
// reports every block no run reached. A failed Exercise is a refusal, not a
// report: a red suite leaves blocks unrun that its Scenarios do specify.
func Measure(plan Plan) (Report, error) {
	work, err := disk.TempDir("", "engine-coverage-")
	if err != nil {
		return Report{}, fmt.Errorf("create the work dir: %w", err)
	}

	defer func() { _ = disk.RemoveTree(work) }()

	dataDir := filepath.Join(work, "covdata")

	err = disk.Dir(dataDir, disk.Private)
	if err != nil {
		return Report{}, fmt.Errorf("create the counter dir: %w", err)
	}

	binPath := filepath.Join(work, "covered")

	err = gotool.BuildCovered(cli.Options{Timeout: buildTimeout},
		plan.Root, binPath, plan.Main, plan.Packages)
	if err != nil {
		return Report{}, fmt.Errorf("build the covered binary: %w", err)
	}

	err = plan.Exercise(binPath, []string{"GOCOVERDIR=" + dataDir})
	if err != nil {
		return Report{}, fmt.Errorf("exercise the covered binary: %w", err)
	}

	profile, err := mergeCounters(work, dataDir)
	if err != nil {
		return Report{}, err
	}

	module, err := modulePath(plan.Root)
	if err != nil {
		return Report{}, err
	}

	return newReport(profile, module, plan.Root)
}

// mergeCounters folds every run's counters into one parsed profile.
func mergeCounters(work, dataDir string) ([]block, error) {
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		return nil, fmt.Errorf("list the counter dir: %w", err)
	}

	if !hasCounters(entries) {
		return nil, ErrNoCoverageData
	}

	profilePath := filepath.Join(work, "profile.txt")

	err = gotool.CoverProfile(cli.Options{Timeout: mergeTimeout}, dataDir, profilePath)
	if err != nil {
		return nil, fmt.Errorf("merge the counters: %w", err)
	}

	raw, err := disk.Read(profilePath)
	if err != nil {
		return nil, fmt.Errorf("read the merged profile: %w", err)
	}

	return parseProfile(string(raw))
}

// hasCounters reports whether any run of the binary got as far as exiting.
func hasCounters(entries []os.DirEntry) bool {
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), counterPrefix) {
			return true
		}
	}

	return false
}
