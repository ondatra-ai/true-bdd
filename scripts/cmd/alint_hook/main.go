// Command alint_hook is the PostToolUse hook: Claude Code writes a file, this
// runs alint over that file alone, and what alint could not fix comes back as
// the block verdict.
//
// It is the only program that spawns alint on a scope. The gates it ends up
// running are alint's business, declared in .alint.yml — nothing here knows
// which file selects which check, which is the whole point of the move.
package main

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/claude/hooks"
	"github.com/ondatra-ai/true-bdd/pkg/cli/alint"
	"github.com/ondatra-ai/true-bdd/pkg/cli/git"
	"github.com/ondatra-ai/true-bdd/scripts/history"
	"github.com/ondatra-ai/true-bdd/scripts/state"
)

const advice = `LINT FAILED on %s. Fix it in that file now, before any other work:
these same gates run at commit time and reject the branch otherwise. What was
auto-fixable is already applied; what follows needs a real edit.

%s`

// tool names this program in the Task's shared log.
const tool = "alint-hook"

func main() {
	hooks.PostToolUse(tool, state.TaskLog(history.RepoRoot()), judge)
}

// judge answers for the file the tool just wrote. Everything unjudgeable — no
// path, a path outside the repository, an ignored file — says nothing at all.
func judge(params hooks.PostToolUseParams, log *slog.Logger) error {
	relative, ok := inRepository(params.FilePath)
	if !ok {
		return nil
	}

	report, err := alint.Fix([]string{relative})
	if err != nil {
		log.Error("running alint", "path", relative, "error", err)

		return nil
	}

	left := report.Outstanding()
	if len(left) == 0 {
		log.Info("clean", "path", relative, "fixed", report.Applied)

		return nil
	}

	return fmt.Errorf(advice, relative, findings(left)) //nolint:err113 // the reason IS the message.
}

// findings is one line per thing still standing. Deduplicated by message:
// every scoped rule shells out to the same `go run`, so a file that does not
// compile fails all of them with one compiler error.
func findings(left []alint.Finding) string {
	seen := map[string]bool{}
	lines := make([]string, 0, len(left))

	for _, found := range left {
		if seen[found.Message] {
			continue
		}

		seen[found.Message] = true

		lines = append(lines, found.String())
	}

	return strings.Join(lines, "\n")
}

// inRepository makes the path repo-relative, and reports false for anything
// this gate has no opinion about.
func inRepository(path string) (string, bool) {
	if path == "" {
		return "", false
	}

	root, err := filepath.Abs(".")
	if err != nil {
		return "", false
	}

	relative, err := filepath.Rel(root, path)
	if err != nil || strings.HasPrefix(relative, "..") {
		return "", false
	}

	ignored, err := git.IsIgnored(relative)
	if err != nil || ignored {
		return "", false
	}

	return relative, true
}
