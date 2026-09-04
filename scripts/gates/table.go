package gates

import (
	"fmt"
	"log/slog"

	"github.com/ondatra-ai/true-bdd/pkg/cli/alint"
)

// Gate is one check: what to run, and which changed paths make it necessary.
type Gate struct {
	// Name is the gate's identity, and the CI step name that must carry it.
	Name string
	// Command is argv, run from the repository root.
	Command []string
	// Globs are doublestar patterns against repo-relative paths. Empty means
	// the gate always runs.
	Globs []string
	// CIAction names the GitHub action CI uses instead of Command, when it
	// does. Empty everywhere since the lint rows collapsed into alint.
	CIAction string
	// Run, when set, is the gate — called in THIS process instead of spawning
	// Command. Command stays as the line CI runs and conformance greps, and
	// both reach the same code: nothing here shells out to its own package.
	Run func() error
}

// Always reports whether this gate runs regardless of the diff.
func (g Gate) Always() bool { return len(g.Globs) == 0 }

// The argv fragments and the one path that repeat across the table.
const (
	goBin    = "go"
	testVerb = "test"
	tagsFlag = "-tags"
	bddTag   = "bdd"
	// testkitGlob is the harness tree every BDD gate depends on.
	testkitGlob = "pkg/testkit/**"
	// countOnce defeats the test cache, so a gate re-runs what it claims to.
	countOnce = "-count=1"
	registry  = "docs/scenarios.yaml"
	goGlob    = "**/*.go"
	goMod     = "go.mod"
	goSum     = "go.sum"
	lintCmd   = "./scripts/cmd/linters"
	runVerb   = "run"
)

// Cheapest first, so a doomed run dies in under a second.
//
//nolint:gochecknoglobals // this package IS a table; see the package doc.
var (
	// goSources is every path that can change what the Go compiler sees.
	goSources = []string{goGlob, goMod, goSum}

	// lintInputs is every path .alint.yml has a rule about, which is what the
	// one Lint gate now covers.
	lintInputs = []string{
		goGlob, goMod, goSum, "**/*.md", "**/*.sh",
		"**/*.yaml", "**/*.yml", "**/*.py",
		"services/bdd-web/**", "true-bdd/**",
		"docs/architecture/**", "docs/product/**", "scripts/lint/**",
	}

	// All is the pipeline. Every entry must also be a step of CI's gates job.
	All = []Gate{
		{
			// The WHOLE lint pipeline. .alint.yml is the one map from a file to
			// the checks it selects, and every leaf it names is
			// ./scripts/cmd/linters — so this table holds no second list.
			Name:    "Lint",
			Command: []string{goBin, runVerb, "./scripts/cmd/alint"},
			Run:     runAlint,
			// The union of what the five lint rows this replaced claimed. Not
			// empty: an Always gate claims no path, and a path claimed by
			// nothing trips Select's fail-safe into running the whole table.
			Globs: lintInputs,
		},
		{
			Name:    "Build",
			Command: []string{goBin, "build", "-o", "./bin/true-bdd", "./services/bdd-cli"},
			Globs:   []string{"services/bdd-cli/**", "pkg/**", "go.mod", "go.sum"},
		},
		{
			Name:    "Test",
			Command: []string{goBin, testVerb, "./..."},
			Globs:   goSources,
		},
		{
			// golangci-lint reaches the bdd tree now (run.build-tags), but
			// `generated: lax` skips the generated tests — where a Testfoo
			// that compiles, never runs and passes is exactly the risk.
			Name:    "Vet the BDD tree",
			Command: []string{goBin, "vet", tagsFlag, bddTag, "./tests/...", "./pkg/testkit/..."},
			Globs:   []string{"tests/**", testkitGlob},
		},
		{
			// Back on the gate: with a mode per caller the judge replays
			// from its own shelf, so a full run reaches no model, needs no
			// key and costs nothing — about a minute for the whole suite.
			Name: "BDD cli replay",
			Command: []string{
				goBin, testVerb, tagsFlag, bddTag, countOnce, "-timeout=40m",
				"./tests/bdd-cli/", "-mode=services:replay,tests:replay",
			},
			Globs: []string{registry, "tests/bdd-cli/**", testkitGlob, "services/bdd-cli/**", "pkg/**"},
		},
		{
			// Hand-written, model-free, and answer in a second.
			Name: "BDD cli coverage guards",
			Command: []string{
				goBin, testVerb, tagsFlag, bddTag, countOnce, "-run",
				"^Test(ScenarioCoverage|StepCoverage|FixtureTreesArePaired)$",
				"./tests/bdd-cli/",
			},
			Globs: []string{registry, "tests/bdd-cli/**", testkitGlob},
		},
		{
			Name: "BDD web coverage guards",
			Command: []string{
				goBin, testVerb, tagsFlag, bddTag,
				countOnce, "-run", "^TestScenarioCoverage$", "./tests/bdd-web/",
			},
			Globs: []string{registry, "tests/bdd-web/**", testkitGlob},
		},
	}
)

// runAlint is the Lint gate. It calls pkg/cli/alint rather than spawning
// ./scripts/cmd/alint, which would fork code this package already links.
func runAlint() error {
	report, err := alint.Check()
	if err != nil {
		return fmt.Errorf("running alint: %w", err)
	}

	left := report.Outstanding()
	for _, finding := range left {
		slog.Error("lint", "rule", finding.RuleID, "path", finding.Path, "message", finding.Message)
	}

	if len(left) > 0 {
		return fmt.Errorf("%w: %d finding(s)", errLintFailed, len(left))
	}

	return nil
}
