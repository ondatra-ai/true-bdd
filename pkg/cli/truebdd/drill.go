// Package truebdd runs and supervises only the true-bdd engine binary for repository tooling.
package truebdd

import (
	"context"
	"io"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

// Options is the external policy of one drill command.
type Options struct {
	Directory string
	Marker    string
	Input     io.ReadCloser
	Output    io.Writer
	Limit     time.Duration
	Grace     time.Duration
}

// Rebuild runs an iteration's engine in its own session.
// It closes opt.Input when stdin forwarding ends so answer writers cannot outlive their reader.
func Rebuild(ctx context.Context, binary string, args []string, opt Options) (int, error) {
	process, err := shell.Start(context.WithoutCancel(ctx), append([]string{binary}, args...), shell.Options{
		Dir: opt.Directory,
		Env: shell.Inherit().Strip("CLAUDECODE",
			"TRUE_BDD_DRILL_WORKER",
			"OLDPWD",
			"GIT_DIR",
			"GIT_WORK_TREE").Set("PWD="+opt.Directory,
			"TRUE_BDD_DRILL_RUN="+opt.Marker),

		Output: shell.To(opt.Output), Session: true, WaitDelay: opt.Grace,
	})
	if err != nil {
		return -1, err
	}
	defer func() { _ = process.Stdin.Close() }()

	go func() {
		defer func() { _ = opt.Input.Close() }()

		_, _ = io.Copy(process.Stdin, opt.Input)
	}()

	result, err := process.Supervise(ctx, opt.Limit, opt.Grace)

	return result.Code, err
}
