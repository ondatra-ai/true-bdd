package testrunner

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/testrunner/dto"
)

// CompileDefinitions compiles the declared Go suite without running any test.
func (r *GoTestRunner) CompileDefinitions(cfg Config) error {
	_, err := r.definitionCommand(cfg, "compile-steps", "^$")

	return err
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
