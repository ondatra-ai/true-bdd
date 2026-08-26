package gates

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
	// does. Only golangci-lint, which the action installs and caches.
	CIAction string
}

// Always reports whether this gate runs regardless of the diff.
func (g Gate) Always() bool { return len(g.Globs) == 0 }

// The argv fragments and the one path that repeat across the table.
const (
	goBin    = "go"
	testVerb = "test"
	tagsFlag = "-tags"
	bddTag   = "bdd"
	registry = "docs/scenarios.yaml"
)

// Order is gates.sh's: cheapest first, so a doomed run dies in under a second.
//
//nolint:gochecknoglobals // this package IS a table; see the package doc.
var (
	// goSources is every path that can change what the Go compiler sees.
	goSources = []string{"**/*.go", "go.mod", "go.sum"}

	// replayInputs is what tests/libraries/runner copies into each fixture
	// tmpdir, plus the trees the binary is rebuilt from.
	replayInputs = []string{
		"services/bdd-cli/**", "true-bdd/**", "templates/**", registry,
		"tests/bdd-cli/**", "tests/libraries/bddgo/**",
		"tests/libraries/runner/**", "tests/libraries/aiproxy/**",
	}

	// All is the pipeline. Every entry must also be a step of CI's gates job.
	All = []Gate{
		{
			// Reads the whole file tree, and shells out to lint-claude.md.sh,
			// which is why that has no row of its own.
			Name:    "Lint repository shape",
			Command: []string{"alint", "check"},
		},
		{
			Name:    "Lint comments",
			Command: []string{"./scripts/lint-comments.sh"},
			Globs:   []string{"**/*.go", "**/*.sh", "**/*.yaml", "**/*.yml"},
		},
		{
			Name:    "Lint document schemas",
			Command: []string{"./scripts/lint-schemas.sh"},
			Globs: []string{
				"true-bdd/**", "docs/architecture/**", "docs/product/**",
				registry, "scripts/cmd/yamlkey/**",
			},
		},
		{
			Name:    "Lint markdown",
			Command: []string{"./scripts/lint-markdown.sh"},
			Globs:   []string{"**/*.md"},
		},
		{
			Name:     "Lint",
			Command:  []string{"golangci-lint", "run"},
			Globs:    goSources,
			CIAction: "golangci/golangci-lint-action",
		},
		{
			Name:    "Build",
			Command: []string{goBin, "build", "-o", "./bin/true-bdd", "./services/bdd-cli"},
			Globs:   []string{"services/bdd-cli/**", "go.mod", "go.sum"},
		},
		{
			Name:    "Test",
			Command: []string{goBin, testVerb, "./..."},
			Globs:   goSources,
		},
		{
			// go vet ./... and golangci-lint both skip the bdd-tagged tree:
			// without this a generated Testfoo compiles, never runs, and passes.
			Name:    "Vet the BDD tree",
			Command: []string{goBin, "vet", tagsFlag, bddTag, "./tests/..."},
			Globs:   []string{"tests/**"},
		},
		{
			Name: "BDD fixtures (replay)",
			Command: []string{
				goBin, testVerb, tagsFlag, bddTag,
				"-timeout=20m", "./tests/bdd-cli/", "-mode=replay",
			},
			Globs: replayInputs,
		},
		{
			Name: "BDD web coverage guards",
			Command: []string{
				goBin, testVerb, tagsFlag, bddTag,
				"-count=1", "-run", "^TestScenarioCoverage$", "./tests/bdd-web/",
			},
			Globs: []string{registry, "tests/bdd-web/**"},
		},
	}
)
