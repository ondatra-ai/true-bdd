package testkit

import "github.com/ondatra-ai/true-bdd/pkg/testkit/internal/runner"

// FileChange is one created, modified or deleted run-relative file, with both byte states.
type FileChange struct {
	// Path names the changed file relative to the run tree.
	Path string
	// Kind is created, modified or deleted.
	Kind string
	// Before contains the bytes before the run.
	Before []byte
	// After contains the bytes after the run.
	After []byte
}

// RunResult is what the service run produced; it remains available after an execution error.
type RunResult struct {
	// ExitCode is the observed process exit status.
	ExitCode int
	// Stdout is the captured standard output.
	Stdout string
	// Stderr is the captured standard error.
	Stderr string
	// Diff contains the observed file changes.
	Diff []FileChange
	// TmpDir locates the assembled run tree.
	TmpDir string
	// StdoutFile locates the saved standard-output stream.
	StdoutFile string
	// StderrFile locates the saved standard-error stream.
	StderrFile string
	// ServiceBinary locates the service executable supplied by the suite.
	ServiceBinary string
}

func resultView(source *runner.RunResult, binary string) *RunResult {
	if source == nil {
		return nil
	}

	changes := make([]FileChange, 0, len(source.Diff))
	for _, change := range source.Diff {
		changes = append(changes, FileChange(change))
	}

	return &RunResult{ExitCode: source.ExitCode, Stdout: source.Stdout, Stderr: source.Stderr, Diff: changes,
		TmpDir:     source.TmpDir,
		StdoutFile: source.StdoutFile, StderrFile: source.StderrFile, ServiceBinary: binary}
}

func resultToRunner(source *RunResult) *runner.RunResult {
	if source == nil {
		return nil
	}

	changes := make([]runner.FileChange, 0, len(source.Diff))
	for _, change := range source.Diff {
		changes = append(changes, runner.FileChange(change))
	}

	return &runner.RunResult{ExitCode: source.ExitCode, Stdout: source.Stdout, Stderr: source.Stderr, Diff: changes,
		TmpDir:     source.TmpDir,
		StdoutFile: source.StdoutFile, StderrFile: source.StderrFile}
}
