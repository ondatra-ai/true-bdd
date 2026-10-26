package git

import (
	"context"
	"fmt"

	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

const addVerb = "add"

// Archive exports a commit without repository history.
func (r Repository) Archive(ctx context.Context, commit string) ([]byte, error) {
	result,
		err := shell.Watch(ctx,
		[]string{Bin,
			"archive",
			"--format=tar",
			commit},
		shell.Options{Dir: r.Dir, Env: Environment()},
		r.Watchdog)
	if err != nil {
		return nil, err
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("git archive: %w", result.Err())
	}

	return []byte(result.Stdout), nil
}

// InitSnapshot records an exported tree as the sole commit of a new repository.
func (r Repository) InitSnapshot(ctx context.Context) (string, error) {
	for _, args := range [][]string{
		{"init", "--template=", "-b", "drill"},
		{"-c", "core.hooksPath=/dev/null", addVerb, "--all", "--force", "."},
		{"-c",
			"core.hooksPath=/dev/null",
			"-c",
			"user.name=TrueBDD drill",
			"-c",
			"user.email=drill@localhost",
			"-c",
			"commit.gpgsign=false",
			"commit",
			"--allow-empty",
			"-m",
			"Drill cut"},
	} {
		_, err := r.output(ctx, args...)
		if err != nil {
			return "", err
		}
	}

	return r.ResolveCommit(ctx, "HEAD")
}
