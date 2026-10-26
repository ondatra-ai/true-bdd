package testrunner

import (
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/registry"
)

// FailingTest is one discovered failure, a member of build code's failing set.
// RunSet refreshes its own verdict and output after each completed apply.
type FailingTest struct {
	Scenario      *registry.RegistryScenario
	Spec          catalogue.Specification
	ID            string    // "<service>/<suite>/<sanitized framework id>"
	Service       string    // sole architecture service or matched Scenario service
	Suite         string    // discovery framework label
	Framework     string    // "go-test" | "playwright" | "jest"
	TestName      string    // framework-native re-run identifier
	FilePath      string    // repo-relative path of the test source
	FailureOutput string    // complete stdout/stderr from last run
	LastRunPassed bool      // updated by RunSet after an apply
	LastRunAt     time.Time // updated by RunSet after an apply
	// RunnerConfig captures the original architecture.yaml block that
	// produced this test, so RunOne can re-invoke the framework with the
	// same config file and root path; populated by Discover, not prompt-visible.
	RunnerConfig Config
}

// Subject returns one member's identity for singleton Item headers and artifacts.
func Subject(item *FailingTest) (string, string) {
	return item.ID, item.TestName
}

// BuildID assembles a deterministic, filename-safe id from the four inputs
// that identify a failing test; slashes in the framework-native id are
// folded to `.` so the id can be spliced into tmp file paths unescaped.
func BuildID(service, suite, framework, frameworkID string) string {
	safe := strings.ReplaceAll(frameworkID, "/", ".")
	safe = strings.ReplaceAll(safe, " ", "_")

	return service + "/" + suite + "/" + framework + ":" + safe
}
