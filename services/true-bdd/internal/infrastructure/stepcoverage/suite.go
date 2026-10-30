package stepcoverage

import (
	"path"
	"path/filepath"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
)

// Suite is the step definitions one suite's steps/ package registers,
// for a check that binds a step's text to them as the suite would.
type Suite struct {
	defs []definition
}

// StepsDir is the steps/ directory of the suite owning the test file at
// testPath: path.Dir of the file, as Scan resolves it, never the service.
func StepsDir(testPath string) string {
	return path.Join(path.Dir(filepath.ToSlash(testPath)), stepsDir)
}

// LoadSuite reads the definitions the steps/ package at dir registers,
// refusing a pattern that will not fold or compile as Scan does. An
// absent directory is a suite with no definitions.
func LoadSuite(dir string) (*Suite, error) {
	defs, err := loadDefinitions(dir)
	if err != nil {
		return nil, err
	}

	return &Suite{defs: defs}, nil
}

// Bind lists every definition whose pattern matches the step text, in
// registration order; anchoring is the pattern author's, as in the suite.
func (s *Suite) Bind(text string) []catalogue.StepDefinition {
	bound := make([]catalogue.StepDefinition, 0, 1)

	for _, def := range s.defs {
		if def.pattern.FindStringSubmatch(text) != nil {
			bound = append(bound, catalogue.StepDefinition{Pattern: def.source, File: def.file, Line: def.line})
		}
	}

	return bound
}
