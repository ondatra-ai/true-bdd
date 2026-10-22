package codexrun

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/cli/codex"
	"github.com/ondatra-ai/true-bdd/pkg/cli/git"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// Runner keeps all artifacts in its checkout; watchdog windows may be shortened in tests.
type Runner struct {
	Root  string
	Stall time.Duration
	Limit time.Duration
	Grace time.Duration
}

// New constructs a runner with the reference's watchdog windows.
func New(root string) Runner {
	const stallMinutes, limitMinutes, graceSeconds = 12, 115, 10

	resolved, err := filepath.EvalSymlinks(root)
	if err == nil {
		root = resolved
	}

	return Runner{Root: root,
		Stall: setting("CODEX_RUN_STALL_MINUTES", stallMinutes, time.Minute),
		Limit: setting("CODEX_RUN_LIMIT_MINUTES", limitMinutes, time.Minute),
		Grace: setting("CODEX_RUN_GRACE_SECONDS", graceSeconds, time.Second)}
}

func setting(key string, fallback float64, unit time.Duration) time.Duration {
	value, err := strconv.ParseFloat(os.Getenv(key), 64)
	if err != nil || value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		value = fallback
	}

	return time.Duration(value * float64(unit))
}

// Start prepares and supervises a fresh run. Refusals consume no attempt.
func (r Runner) Start(ctx context.Context, opts Options) (Record, error) {
	err := opts.validate()
	if err != nil {
		return Record{}, err
	}

	brief, err := disk.Read(opts.Brief)
	if err != nil {
		return Record{}, refuse(err.Error())
	}

	record, err := r.claim(ctx, opts, false)
	if err != nil {
		return Record{}, err
	}

	prompt := strings.TrimRight(string(brief), "\n") + "\n\n## This run\n\n" + record.context(r)

	err = disk.Write(r.file(opts.Name, "brief.md"), []byte(prompt), disk.Private)
	if err != nil {
		return r.finish(record, "failed", err.Error(), -1)
	}

	return r.attempt(ctx, record, "")
}

// Resume allows one retry, naming the saved session rather than the last session.
func (r Runner) Resume(ctx context.Context, name, note string) (Record, error) {
	if strings.TrimSpace(note) == "" {
		return Record{}, refuse("resume needs --note")
	}

	err := (Options{Name: name, Kind: "review"}).validate()
	if err != nil {
		return Record{}, err
	}

	record, err := r.claim(ctx, Options{Name: name}, true)
	if err != nil {
		return Record{}, err
	}

	return r.attempt(ctx, record, note)
}

func (r Record) context(runner Runner) string {
	text := "You run read-only without network. Read saved issues instead of calling gh."
	if r.writer() {
		text = fmt.Sprintf("You run with full access at high reasoning effort in %s. Stay inside this worktree.\n"+
			"Branch: %s; starting commit: %s. Done requires a clean checkout and a new commit.\n"+
			"Use plain local git commits; never push or open a PR. End commit messages with Refs #<issue>.",
			r.Worktree, r.Branch, r.Since)
	}

	text += fmt.Sprintf("\nWeb search enabled: %t. Your final message is the report Claude reads.\n", r.Search)

	var issues strings.Builder
	for _, issue := range r.Issues {
		issues.WriteString("\n- #" + issue + ": " + runner.file(r.Name, "issues/"+issue+".md"))
	}

	text += issues.String()

	return text + "\n"
}

func (r Runner) announce(record Record) {
	slog.Info("Codex run log", "path", record.Log)

	weekly := "unknown"
	if record.WeeklyLimit != nil {
		weekly = record.WeeklyLimit.String()
	}

	slog.Info("Codex weekly limit", "usage", weekly)

	if record.WeeklyLimit != nil && record.WeeklyLimit.UsedPercent > 90 {
		slog.Warn("Codex weekly usage is past 90%; the run will still start")
	}
}

func (r Runner) watch() codex.Watchdog {
	return codex.Watchdog{Stall: r.Stall, Limit: r.Limit, Grace: r.Grace}
}

func (r Runner) repository(dir string) git.Repository {
	return git.Repository{Dir: dir, Watchdog: r.preparationWatch()}
}

func (r Runner) preparationWatch() codex.Watchdog {
	watch := r.watch()
	watch.Limit = r.Stall

	return watch
}
