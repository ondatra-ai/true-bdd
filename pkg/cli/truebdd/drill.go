// Package truebdd runs and supervises only the true-bdd engine binary for repository tooling.
package truebdd

import (
	"context"
	"io"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

const (
	fixFlag   = "--fix"
	buildVerb = "build"
)

// ModelOverride is one unchanged <tier>=<cli>:<model> flag value.
type ModelOverride string

// Options is the external policy of one drill command.
type Options struct {
	ModelOverrides []ModelOverride
	Directory      string
	ModuleCache    string
	Marker         string
	Input          io.ReadCloser
	Output         io.Writer
	Limit          time.Duration
	Grace          time.Duration
}

// BuildTests renders test files with the iteration's engine.
func BuildTests(ctx context.Context, binary string, opt Options) (int, error) {
	return rebuild(ctx, binary, []string{buildVerb, "tests", fixFlag}, opt)
}

// BuildSteps authors and verifies step definitions with the iteration's engine.
func BuildSteps(ctx context.Context, binary string, opt Options) (int, error) {
	return rebuild(ctx, binary, []string{buildVerb, "steps", fixFlag}, opt)
}

// BuildCode repairs production code with the iteration's engine.
func BuildCode(ctx context.Context, binary string, opt Options) (int, error) {
	return rebuild(ctx, binary, []string{buildVerb, "code", fixFlag}, opt)
}

// rebuild closes stdin forwarding so answer writers cannot outlive their reader.
func rebuild(ctx context.Context, binary string, args []string, opt Options) (int, error) {
	for _, override := range opt.ModelOverrides {
		args = append(args, "--model", string(override))
	}

	env := shell.Inherit().Strip("CLAUDECODE", "TRUE_BDD_DRILL_WORKER", "OLDPWD", "GIT_DIR", "GIT_WORK_TREE").Set(
		"PWD="+opt.Directory, "TRUE_BDD_DRILL_RUN="+opt.Marker, "GOWORK=off")
	if opt.ModuleCache != "" {
		env = env.Set("GOMODCACHE=" + opt.ModuleCache)
	}

	process, err := shell.Start(context.WithoutCancel(ctx), append([]string{binary}, args...), shell.Options{
		Dir: opt.Directory,
		Env: env,

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
