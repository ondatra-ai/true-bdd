package testrunner

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/testrunner/dto"
)

// CompileDefinitions builds only the Steps packages, before any test exists.
func (r *GoTestRunner) CompileDefinitions(suites []string) error {
	for _, suite := range suites {
		absolute, err := filepath.Abs(suite)
		if err != nil {
			return fmt.Errorf("locate Steps package: %w", err)
		}

		stdout, stderr, err := r.exec(absolute, "compile-steps", []string{"go", "build", "-tags", "bdd", "./..."})
		if err != nil {
			return fmt.Errorf("compile Steps package: %w\n%s%s", err, stdout.String(), stderr.String())
		}
	}

	return nil
}

// ListScenarios requires each package-qualified generated test in one listing.
func (r *GoTestRunner) ListScenarios(cfg Config, identities []string) error {
	output, err := r.definitionCommand(cfg, "list-scenarios", ".")
	if err != nil {
		return err
	}

	listed := make(map[string]bool)

	for _, line := range strings.Split(output, "\n") {
		var event dto.GoTestEvent
		if json.Unmarshal([]byte(line), &event) != nil || event.Action != "output" {
			continue
		}

		for _, name := range strings.Split(event.Output, "\n") {
			listed[event.Package+goTestNameSeparator+strings.TrimSpace(name)] = true
		}
	}

	for _, identity := range identities {
		if !listed[identity] {
			return fmt.Errorf("%w: generated test not listed: %s", ErrScenarioNotRun, identity)
		}
	}

	return nil
}

func (r *GoTestRunner) definitionCommand(cfg Config, phase, filter string) (string, error) {
	argv, err := cfg.argv()
	if err != nil {
		return "", err
	}

	argv = append(argv, "-run", "^$", "-list", filter)

	stdout, stderr, err := r.exec(CommandDir(cfg), phase, argv)
	if err != nil {
		return "", fmt.Errorf("%s: %w\n%s%s", phase, err, stdout.String(), stderr.String())
	}

	return stdout.String(), nil
}
