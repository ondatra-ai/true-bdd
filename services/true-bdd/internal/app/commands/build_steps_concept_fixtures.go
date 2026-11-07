package commands

import (
	"path/filepath"
	"slices"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/registry"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/stepcoverage"
)

// conceptFixture is one fixture a Scenario using the Item's steps names:
// its directory name under the context's fixture_root.
type conceptFixture struct {
	Fixture string `yaml:"fixture"`
}

// conceptFixtures gathers the fixture inputs of every Scenario of the suite
// using the Item's steps: each quoted value of its steps that names a
// directory under the suite's fixtures/ is a fixture it starts from.
func conceptFixtures(item *BuildStepsItem, scenarios []*registry.RegistryScenario) []conceptFixture {
	root := filepath.Join(filepath.Dir(filepath.FromSlash(item.Suite)), "fixtures")
	fixtures := make([]conceptFixture, 0)

	for _, scenario := range scenarios {
		if stepcoverage.StepsDir(scenario.Path) != item.Suite || !conceptItemUsed(item, scenario) {
			continue
		}

		for _, name := range registry.NamedFixtures(scenario, root) {
			dir := name

			index := slices.IndexFunc(fixtures, func(fixture conceptFixture) bool { return fixture.Fixture == dir })
			if index >= 0 {
				continue
			}

			fixtures = append(fixtures, conceptFixture{Fixture: dir})
		}
	}

	return fixtures
}
