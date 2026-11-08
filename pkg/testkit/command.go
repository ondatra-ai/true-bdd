package testkit

import (
	"context"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/cli"
	"github.com/ondatra-ai/true-bdd/pkg/cli/gotool"
	"github.com/ondatra-ai/true-bdd/pkg/cli/spec"
)

// CommandResult contains a declared command's exit code and captured streams.
type CommandResult struct {
	// Code is the process exit status.
	Code int
	// Stdout is the captured standard output.
	Stdout string
	// Stderr is the captured standard error.
	Stderr string
}

// RunCommand executes a Scenario-declared command in the assembled host tree with GOWORK=off and CLAUDECODE removed.
func RunCommand(argv []string, directory string, timeout time.Duration) (CommandResult, error) {
	result, err := spec.Run(argv, cli.Options{
		Dir:     directory,
		Env:     cli.Inherit().Strip("CLAUDECODE").Set("GOWORK=off"),
		Timeout: timeout,
	})

	return CommandResult{Code: result.Code, Stdout: result.Stdout, Stderr: result.Stderr}, err
}

// FindCLI resolves a fixture dependency on the caller's current PATH.
func FindCLI(name string) (string, error) { return cli.Find(name) }

// GoEnvironment resolves the installed Go runtime and module cache for path observations.
func GoEnvironment(ctx context.Context) (map[string]string, error) { return gotool.Environment(ctx) }
