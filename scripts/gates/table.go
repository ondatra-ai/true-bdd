package gates

import (
	"fmt"
	"log/slog"

	"github.com/ondatra-ai/true-bdd/pkg/cli/alint"
	"github.com/ondatra-ai/true-bdd/scripts/gomodules"
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
	goBin         = "go"
	testVerb      = "test"
	tagsFlag      = "-tags"
	bddTag        = "bdd"
	templatesGlob = "templates/**"
	// testkitGlob is the harness tree every BDD gate depends on.
	testkitGlob = "pkg/testkit/**"
	// countOnce defeats the test cache, so a gate re-runs what it claims to.
	countOnce   = "-count=1"
	registry    = "docs/scenarios.yaml"
	goGlob      = "**/*.go"
	goMod       = "go.mod"
	nestedGoMod = "**/go.mod"
	nestedGoSum = "**/go.sum"
	goSum       = "go.sum"
	lintCmd     = "./scripts/cmd/linters"
	runVerb     = "run"
	gatesCmd    = "./scripts/cmd/gates"
)

// Cheapest first, so a doomed run dies in under a second.
//
//nolint:gochecknoglobals // this package IS a table; see the package doc.
var (
	// goSources is every path that can change what the Go compiler sees.
	goSources = []string{goGlob, goMod, goSum, nestedGoMod, nestedGoSum, "go.work", "go.work.sum"}

	// lintInputs is every path .alint.yml has a rule about, which is what the
	// one Lint gate now covers.
	lintInputs = []string{
		goGlob, goMod, goSum, nestedGoMod, nestedGoSum, "go.work", "go.work.sum", "**/*.md", "**/*.sh",
		"**/*.yaml", "**/*.yml", "**/*.py",
		"true-bdd/**",
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
			Name:    "Engine owns repository code",
			Command: []string{goBin, runVerb, gatesCmd, "engine-imports"},
			Run:     engineImports,
			Globs:   append([]string{"services/**", "scripts/gates/**", "scripts/cmd/gates/**"}, goSources...),
		},
		{
			Name:    "Release rehearsal",
			Command: []string{goBin, runVerb, gatesCmd, "release-rehearsal"},
			Run:     ReleaseRehearsal,
			Globs: append([]string{
				"services/**", "pkg/**", "scripts/**", "tests/**", "true-bdd/**", templatesGlob,
			}, goSources...),
		},
		{
			Name:    "Init defaults drift",
			Command: []string{goBin, testVerb, countOnce, "-run", "^TestInitDefaultsMatchProjectFiles$", "./scripts/gates/"},
			Globs: []string{
				"services/true-bdd/internal/infrastructure/initialization/**",
				"true-bdd/true-bdd.yaml", "true-bdd/checklists/**", templatesGlob, "scripts/gates/**",
			},
		},
		{
			Name:    "Build",
			Command: []string{goBin, runVerb, gatesCmd, "build"},
			Run:     gomodules.Build,
			Globs:   goSources,
		},
		{
			Name:    "Test",
			Command: []string{goBin, runVerb, gatesCmd, "test"},
			Run:     gomodules.Test,
			Globs:   goSources,
		},
		{
			// golangci-lint reaches the bdd tree now (run.build-tags), but
			// `generated: lax` skips the generated tests — where a Testfoo
			// that compiles, never runs and passes is exactly the risk.
			Name:    "Vet the BDD tree",
			Command: []string{goBin, runVerb, gatesCmd, "vet"},
			Run:     gomodules.Vet,
			Globs:   goSources,
		},
		{
			// Hand-written, model-free, and answer in a second.
			Name: "BDD cli coverage guards",
			Command: []string{
				goBin, testVerb, tagsFlag, bddTag, countOnce, "-run",
				"^Test(ScenarioCoverage|FixtureTreesArePaired|FixtureTemplateOverlays)$",
				"./tests/true-bdd/",
			},
			Globs: []string{registry, "tests/true-bdd/**", templatesGlob, testkitGlob},
		},
		{
			// Committed recordings must not carry the recording machine's identity.
			Name:    "Recordings",
			Command: []string{goBin, runVerb, gatesCmd, "recordings"},
			Run:     ScanRecordings,
			Globs:   []string{"tests/true-bdd/fixtures/**"},
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
