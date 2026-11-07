package testkit

import (
	"github.com/ondatra-ai/true-bdd/pkg/testkit/internal/reporter"
	"github.com/ondatra-ai/true-bdd/pkg/testkit/internal/runner"
	"regexp"
)

// HarnessRecord is the persisted outcome of one Scenario run.
type HarnessRecord = reporter.HarnessRecord

// FileChangeRecord is a persisted file change without its bytes.
type FileChangeRecord = reporter.FileChangeRecord

// JudgeRecord is the persisted judge invocation and its usage.
type JudgeRecord = reporter.JudgeRecord

// ManifestSnapshot is the exact fixture data used by a completed run.
type ManifestSnapshot = reporter.ManifestSnapshot

// SessionMeta identifies a run's mode and planned fixture names.
type SessionMeta = runner.SessionMeta

const (
	// SpawnLogDir names the directory of process-spawn observations.
	SpawnLogDir = runner.SpawnLogDir
	// HarnessRecordFile names the persisted Scenario outcome.
	HarnessRecordFile = runner.HarnessRecordFile
	// ManifestSnapshotFile names the resolved fixture description.
	ManifestSnapshotFile = runner.ManifestSnapshotFile
	// JudgeSystemFile names the judge's saved system prompt.
	JudgeSystemFile = runner.JudgeSystemFile
	// JudgeUserFile names the judge's saved user prompt.
	JudgeUserFile = runner.JudgeUserFile
	// JudgeResponseFile names the judge's saved answer.
	JudgeResponseFile = runner.JudgeResponseFile
	// VerdictPass denotes a passing Scenario test in the run's report.
	VerdictPass = runner.VerdictPass
	// VerdictFail denotes a failing Scenario test in the run's report.
	VerdictFail = runner.VerdictFail
	// VerdictSkip denotes a skipped Scenario test in the run's report.
	VerdictSkip = runner.VerdictSkip
	// ProxyModeLive invokes real dependencies without recording.
	ProxyModeLive = runner.ProxyModeLive
	// ProxyModeRecord invokes real dependencies and records their behavior.
	ProxyModeRecord = runner.ProxyModeRecord
	// ProxyModeReplay reproduces dependencies from committed recordings.
	ProxyModeReplay = runner.ProxyModeReplay
	// DefaultInputDir names a fixture's starting-tree directory.
	DefaultInputDir = runner.DefaultInputDir
)

// ReadSessionMeta loads a session's mode and planned fixture names; an old session may have none.
func ReadSessionMeta(directory string) (*SessionMeta, error) {
	return runner.ReadSessionMeta(directory)
}

// WriteSessionMeta stores a session's mode and planned fixture names atomically.
func WriteSessionMeta(directory, mode string, planned []string) error {
	return runner.WriteSessionMeta(directory, mode, planned)
}

// FindRepoRoot finds the Host project's root from VCS metadata or its Specification.
func FindRepoRoot() (string, error) { return runner.FindRepoRoot() }

// CheckExitCode compares the observed process exit code with the Then's expected value.
func CheckExitCode(actual, expected int) error { return runner.CheckExitCode(actual, expected) }

// CheckStdout requires the Then's pattern to match anywhere in the whole stdout stream.
func CheckStdout(stdout string, pattern *regexp.Regexp) error {
	return runner.CheckStdout(stdout, pattern)
}

// Teardown runs fixture cleanup commands after the Scenario, logging every failure.
func Teardown(directory string, commands []string) { runner.Teardown(directory, commands) }
