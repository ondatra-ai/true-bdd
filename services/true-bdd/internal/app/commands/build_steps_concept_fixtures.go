package commands

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"unicode/utf8"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/registry"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/stepcoverage"
)

// conceptFixture is one fixture a Scenario using the Item's steps names:
// its directory, the Scenarios naming it, and its input files whole.
type conceptFixture struct {
	Fixture   string        `yaml:"fixture"`
	Scenarios []string      `yaml:"scenarios"`
	Files     []fixtureFile `yaml:"files"`
}

type fixtureFile struct {
	Path    string `yaml:"path"`
	Content string `yaml:"content"`
}

// fixtureRecordings holds a fixture's recorded turns, which are not its inputs.
const fixtureRecordings = "cassettes"

var quotedValue = regexp.MustCompile(`"([^"]+)"`)

// conceptFixtures gathers the fixture inputs of every Scenario of the suite
// using the Item's steps: each quoted value of its steps that names a
// directory under the suite's fixtures/ is a fixture it starts from.
func conceptFixtures(item *BuildStepsItem, scenarios []*registry.RegistryScenario) ([]conceptFixture, error) {
	root := filepath.Join(filepath.Dir(filepath.FromSlash(item.Suite)), "fixtures")
	fixtures := make([]conceptFixture, 0)

	for _, scenario := range scenarios {
		if stepcoverage.StepsDir(scenario.Path) != item.Suite || !conceptItemUsed(item, scenario) {
			continue
		}

		for _, name := range namedFixtures(scenario, root) {
			dir := filepath.ToSlash(filepath.Join(root, name))

			index := slices.IndexFunc(fixtures, func(fixture conceptFixture) bool { return fixture.Fixture == dir })
			if index >= 0 {
				if !slices.Contains(fixtures[index].Scenarios, scenario.ID) {
					fixtures[index].Scenarios = append(fixtures[index].Scenarios, scenario.ID)
				}

				continue
			}

			files, err := fixtureInputs(dir)
			if err != nil {
				return nil, err
			}

			fixtures = append(fixtures, conceptFixture{Fixture: dir, Scenarios: []string{scenario.ID}, Files: files})
		}
	}

	return fixtures, nil
}

// namedFixtures lists, in step order, the fixture directories a Scenario names.
func namedFixtures(scenario *registry.RegistryScenario, root string) []string {
	names := make([]string, 0)

	for _, step := range scenario.Phrases() {
		for _, match := range quotedValue.FindAllStringSubmatch(step.Text, -1) {
			name := match[1]
			if name != filepath.Base(name) || name == "." || name == ".." || slices.Contains(names, name) {
				continue
			}

			info, err := os.Stat(filepath.Join(root, name))
			if err == nil && info.IsDir() {
				names = append(names, name)
			}
		}
	}

	return names
}

// fixtureInputs reads every regular file of a fixture in lexical path
// order, leaving out its recordings; a file that is not text is named only.
func fixtureInputs(dir string) ([]fixtureFile, error) {
	files := make([]fixtureFile, 0)

	err := filepath.WalkDir(filepath.FromSlash(dir), func(path string, entry fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case entry.IsDir() && entry.Name() == fixtureRecordings:
			return filepath.SkipDir
		case !entry.Type().IsRegular():
			return nil
		}

		content, err := disk.Read(path)
		if err != nil {
			return fmt.Errorf("read fixture input: %w", err)
		}

		text := string(content)
		if !utf8.Valid(content) {
			text = fmt.Sprintf("(%d bytes, not text)", len(content))
		}

		files = append(files, fixtureFile{Path: filepath.ToSlash(path), Content: text})

		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("read fixture %s: %w", dir, err)
	}

	return files, nil
}
