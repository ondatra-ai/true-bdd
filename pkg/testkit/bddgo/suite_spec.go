package bddgo

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"gopkg.in/yaml.v3"
)

// ErrSuiteNotDeclared signals that the architectural spec declares no
// service by the name a test binary asked for. Refused rather than
// defaulted, to keep coverage predictable.
var ErrSuiteNotDeclared = errors.New("architecture declares no service by that name")

// SuiteSpec is what the test binary needs to know about itself: the
// service whose scenarios it owns, and the tree its step definitions
// live under. There are no suites in the document (docs/adr/0009).
type SuiteSpec struct {
	Name      string
	Service   string
	Path      string
	Framework string
}

type rawService struct {
	Name string `yaml:"name"`
}

type rawArchitecture struct {
	Architecture struct {
		Testing struct {
			Framework string `yaml:"framework"`
		} `yaml:"testing"`
		Services []rawService `yaml:"services"`
	} `yaml:"architecture"`
}

// LoadSuiteSpec reads the architectural spec and returns the spec for the
// named service's tests. The error names every service the document does
// declare, since the mistake this catches is almost always a typo.
func LoadSuiteSpec(path, name string) (SuiteSpec, error) {
	data, err := disk.Read(path)
	if err != nil {
		return SuiteSpec{}, fmt.Errorf("read architecture %s: %w", path, err)
	}

	var raw rawArchitecture

	err = yaml.Unmarshal(data, &raw)
	if err != nil {
		return SuiteSpec{}, fmt.Errorf("parse architecture %s: %w", path, err)
	}

	declared := make([]string, 0, len(raw.Architecture.Services))

	for _, service := range raw.Architecture.Services {
		if service.Name == name {
			return SuiteSpec{
				Name:      name,
				Service:   name,
				Path:      "tests/" + name,
				Framework: raw.Architecture.Testing.Framework,
			}, nil
		}

		declared = append(declared, service.Name)
	}

	return SuiteSpec{}, fmt.Errorf("%s: %q: %w (declared: %s)",
		path, name, ErrSuiteNotDeclared, strings.Join(declared, ", "))
}

// Owns reports whether a scenario belongs to this suite: the scenario's
// `service:` against this one's — one line of the spec, not an
// id-prefix convention.
func (s SuiteSpec) Owns(scenario Scenario) bool {
	return scenario.Service == s.Service
}
