package drill

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/cli/git"
	"github.com/ondatra-ai/true-bdd/pkg/cli/gotool"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// Runner owns the repository and the external time policy, never AI budgets.
type Runner struct {
	Root  string
	Limit time.Duration
	Grace time.Duration
	Poll  time.Duration
}

// New uses the drill's three-hour command limit.
func New(root string) Runner {
	const (
		limit = 3 * time.Hour
		poll  = 100 * time.Millisecond
	)

	return Runner{Root: root, Limit: limit, Grace: time.Second, Poll: poll}
}

// Start builds the base before creating any cuts.
func (r Runner) Start(ctx context.Context, base string) (Record, error) {
	repo := git.Repository{Dir: r.Root}

	commit, err := repo.ResolveCommit(ctx, base)
	if err != nil {
		return Record{}, err
	}

	directory, err := privateDirectory(r.Root)
	if err != nil {
		return Record{}, err
	}

	record := Record{Directory: directory,
		Repository: r.Root,
		Base:       commit,
		Binary: filepath.Join(directory,
			"engine"),
		StartedAt: time.Now().UTC(),
		Cuts:      map[string]Cut{},
		Runs:      map[string]Attempt{},
		Audits:    map[string][]Audit{}}

	record.Digest, err = buildBase(ctx, record)
	if err != nil {
		return record, err
	}

	err = saveIdentity(record)
	if err != nil {
		return record, err
	}

	record.Manifests, err = checkManifests(ctx, record)
	if err != nil {
		_ = save(record)

		return record, err
	}

	record.AllowedRoots, err = gotool.ToolchainRoots(ctx)
	if err != nil {
		return record, err
	}

	record.AllowedRoots = append(record.AllowedRoots, record.Manifests.Cache)
	record.AllowedRoots = append(record.AllowedRoots, configurationRoots()...)

	err = save(record)
	if err != nil {
		return record, err
	}

	slog.Info("Drill iteration created", "iteration", directory, "base", commit, "binary", record.Binary)

	return record, nil
}

func privateDirectory(checkout string) (string, error) {
	directory, err := disk.TempDir("", "true-bdd-")
	if err != nil {
		return "", err
	}

	resolved, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return "", fmt.Errorf("resolving private directory: %w", err)
	}

	if within(resolved, checkout) {
		_ = disk.RemoveTree(resolved)

		return "", fmt.Errorf("%w: temporary directory is inside checkout", errRefused)
	}

	return resolved, nil
}

func configurationRoots() []string {
	home, _ := os.UserHomeDir()

	codex := os.Getenv("CODEX_HOME")
	if codex == "" {
		codex = filepath.Join(home, ".codex")
	}

	return []string{filepath.Join(home, ".claude"), codex}
}

func within(path, root string) bool {
	relative, err := filepath.Rel(root, path)

	return root != "" && err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func buildBase(ctx context.Context, record Record) (string, error) {
	source, err := exportBase(ctx, record)
	if err != nil {
		return "", err
	}
	defer func() { _ = disk.RemoveTree(source) }()

	err = gotool.BuildTrimmed(ctx, source, record.Binary)
	if err != nil {
		return "", err
	}

	return fingerprint(record.Binary)
}

func exportBase(ctx context.Context, record Record) (string, error) {
	source, err := privateDirectory(record.Repository)
	if err != nil {
		return "", err
	}

	archive, err := (git.Repository{Dir: record.Repository}).Archive(ctx, record.Base)
	if err == nil {
		err = exportTree(source, archive, "")
	}

	if err != nil {
		_ = disk.RemoveTree(source)

		return "", err
	}

	return source, nil
}
