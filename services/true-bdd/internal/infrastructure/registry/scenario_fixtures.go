package registry

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
)

var quotedValue = regexp.MustCompile(`"([^"]+)"`)

// NamedFixtures lists, in step order, the fixture directories a Scenario names
// with a quoted value anywhere in its steps. Missing directories are ignored.
func NamedFixtures(scenario *RegistryScenario, root string) []string {
	names := make([]string, 0)

	for _, step := range scenario.CheckedSteps() {
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
