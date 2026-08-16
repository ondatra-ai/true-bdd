package bddgo

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// ErrSuiteNotDeclared signals that the architectural spec has no
// `architecture.testing.suites[]` entry with the name a test binary
// asked for. Refused rather than defaulted: a suite that runs scenarios
// the spec never assigned it is a suite whose coverage no reader can
// predict.
var ErrSuiteNotDeclared = errors.New("architecture declares no test suite by that name")

// SuiteSpec is one `architecture.testing.suites[]` entry, as the test
// binary needs it: the suite's own name, the service whose scenarios it
// owns, and the tree its step definitions live under.
//
// It is read from the same document the engine reads. The alternative —
// a constant in the test binary — is a second place to say which
// scenarios a suite covers, and the two would disagree the first time
// someone edited only one.
type SuiteSpec struct {
	Name      string `yaml:"name"`
	Service   string `yaml:"service"`
	Path      string `yaml:"path"`
	Framework string `yaml:"framework"`
}

type rawArchitecture struct {
	Architecture struct {
		Testing struct {
			Suites []SuiteSpec `yaml:"suites"`
		} `yaml:"testing"`
	} `yaml:"architecture"`
}

// LoadSuiteSpec reads the architectural spec and returns the named
// suite. The error names every suite the document does declare, because
// the mistake this catches is almost always a typo and the fix is
// almost always visible in that list.
func LoadSuiteSpec(path, name string) (SuiteSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return SuiteSpec{}, fmt.Errorf("read architecture %s: %w", path, err)
	}

	var raw rawArchitecture

	err = yaml.Unmarshal(data, &raw)
	if err != nil {
		return SuiteSpec{}, fmt.Errorf("parse architecture %s: %w", path, err)
	}

	declared := make([]string, 0, len(raw.Architecture.Testing.Suites))

	for _, suite := range raw.Architecture.Testing.Suites {
		if suite.Name == name {
			return suite, nil
		}

		declared = append(declared, suite.Name)
	}

	return SuiteSpec{}, fmt.Errorf("%s: %q: %w (declared: %s)",
		path, name, ErrSuiteNotDeclared, strings.Join(declared, ", "))
}

// Owns reports whether a scenario belongs to this suite. The join is the
// scenario's `service:` against the suite's — one line of the spec,
// replacing the id-prefix convention (`INT-` / `E2E-`) that used to
// decide it in prompt prose and nowhere in code.
func (s SuiteSpec) Owns(scenario Scenario) bool {
	return scenario.Service == s.Service
}
