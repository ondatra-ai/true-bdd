// Command alint runs this repository's whole lint pipeline, and is the only
// command line that does.
//
// It holds no gate list and no file globs. Both live in .alint.yml, which is
// the single map from a file to the checks it selects; every leaf reached from
// there is ./scripts/cmd/linters. CI runs this; a person runs this; the gate
// table calls pkg/cli/alint directly, being Go itself.
//
//	alint            every rule, whole tree, report only
//	alint <path>...  the rules those paths select, fixing what has a fixer
package main

import (
	"log/slog"
	"os"

	"github.com/ondatra-ai/true-bdd/pkg/cli/alint"
	"github.com/ondatra-ai/true-bdd/pkg/logging"
	"github.com/ondatra-ai/true-bdd/scripts/history"
	"github.com/ondatra-ai/true-bdd/scripts/state"
)

// tool names this program in the Task's shared log.
const tool = "alint"

func main() {
	logging.Install(logging.Stderr, state.TaskLog(history.RepoRoot()), tool)

	report, err := run(os.Args[1:])
	if err != nil {
		slog.Error("alint failed", "error", err)
		os.Exit(1)
	}

	left := report.Outstanding()
	if len(left) == 0 {
		slog.Info("lint clean", "fixed", report.Applied)

		return
	}

	for _, finding := range left {
		slog.Error("lint", "rule", finding.RuleID, "path", finding.Path, "message", finding.Message)
	}

	os.Exit(1)
}

// run checks the whole tree, or fixes the named paths. Which of the two is the
// only decision this command makes; pkg/cli/alint owns the rest.
func run(paths []string) (alint.Report, error) {
	if len(paths) == 0 {
		return alint.Check()
	}

	return alint.Fix(paths)
}
