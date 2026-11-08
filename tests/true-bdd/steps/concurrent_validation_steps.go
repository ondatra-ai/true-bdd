package steps

import (
	"path/filepath"
	"strings"

	proxycli "github.com/ondatra-ai/true-bdd/pkg/proxy/cli"
	"github.com/ondatra-ai/true-bdd/pkg/testkit"
)

func registerConcurrentValidationSteps(suite testkit.StepRegistrar[State]) {
	suite.Step(`^Scenario validation overlaps with ordered checklist items and fix menus$`, assertConcurrentValidation)
	suite.Step(`^replay consumes the recorded validation requests in reverse order$`, assertReverseRequests)
	suite.Step(`^the validation CLIs are controlled for concurrent arrival$`, controlConcurrentArrival)
}

//truebdd:concept given "fixture AI CLIs"
func controlConcurrentArrival(state *State, _ []string) error {
	fixture, err := filepath.Abs(state.Fixture.Dir)
	if err != nil {
		return state.fail("resolve validation fixture: %w", err)
	}

	state.ObserveServices(fixture)

	standin, err := proxycli.StandIn(state, "claude")
	if err != nil {
		return state.fail("resolve validation stand-in: %w", err)
	}

	proxycli.FallbackPath(state, filepath.Join(fixture, "cli"))
	state.PrependPath(filepath.Join(fixture, "front"))
	state.SetEnv("TRUE_BDD_CONCURRENT_PROXY=" + standin)

	return nil
}

//truebdd:concept then "engine activity"
func assertConcurrentValidation(state *State, _ []string) error {
	records, err := readEngineLog(state)
	if err != nil {
		return err
	}

	count, err := validationChecklistSize(state)
	if err != nil {
		return err
	}

	observed := newValidationObservation(count)
	for _, record := range records {
		err = observed.observe(record)
		if err != nil {
			return state.fail("observe validation: %w", err)
		}
	}

	err = observed.complete()
	if err != nil {
		return state.fail("observe validation: %w", err)
	}

	if !observed.overlap || strings.Join(observed.menus, ",") != "alpha,beta" {
		return state.fail("overlap=%v, applied menus=%v", observed.overlap, observed.menus)
	}

	return nil
}

//truebdd:concept then "AI CLI observations"
func assertReverseRequests(state *State, _ []string) error {
	if state.Mode() != testkit.ProxyModeReplay {
		return nil
	}

	consumed, err := proxycli.Consumed(state, "claude")
	if err != nil {
		return state.fail("read consumed requests: %w", err)
	}

	if len(consumed) == 0 || consumed[0] != "claude-002" {
		return state.fail("reverse replay did not consume second recorded request first: %v", consumed)
	}

	return nil
}
