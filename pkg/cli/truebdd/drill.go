// Package truebdd supervises the engine binary for repository tooling.
package truebdd

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

// Options is the external policy of one drill command.
type Options struct {
	Directory string
	Marker    string
	Input     io.Reader
	Output    io.Writer
	Limit     time.Duration
	Grace     time.Duration
}

// Rebuild runs an iteration's engine in its own session.
func Rebuild(ctx context.Context, binary string, args []string, opt Options) (int, error) {
	process, err := shell.Start(context.WithoutCancel(ctx), append([]string{binary}, args...), shell.Options{
		Dir: opt.Directory,
		Env: shell.Inherit().Strip("CLAUDECODE",
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

	go func() { _, _ = io.Copy(process.Stdin, opt.Input) }()

	result, err := process.Supervise(ctx, opt.Limit, opt.Grace)

	return result.Code, err
}

// StartSupervisor detaches a drill worker without inherited terminal descriptors.
func StartSupervisor(ctx context.Context, binary, iteration, tag, marker string) (int, error) {
	process,
		err := shell.Start(context.WithoutCancel(ctx),
		[]string{binary,
			"supervise",
			"--iteration",
			iteration,
			tag},
		shell.Options{
			Session: true, Stdin: strings.NewReader(""), Output: shell.Discard(),
			Env: shell.Inherit().Strip("TRUE_BDD_DRILL_RUN").Set("TRUE_BDD_DRILL_WORKER=" + marker),
		})
	if err != nil {
		return 0, err
	}

	go func() { _, _ = process.Wait() }()

	return process.Pid(), nil
}
