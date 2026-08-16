package runner

import (
	"fmt"
	"os"
	"regexp"
	"slices"
)

// modelRefPattern matches an `engine.models` entry's `"<cli>:<model>"`
// value and captures the cli. Deliberately a scan rather than a YAML
// parse: this package cannot import the engine's internal config
// packages, and the only fact needed is which binaries must exist.
var modelRefPattern = regexp.MustCompile(`(?m)^\s+\w+:\s*"?([a-z]+):[^"\n]+"?\s*$`)

// supportedCLICount sizes the result: claude, crush, codex.
const supportedCLICount = 3

// RequiredCLIs returns the distinct agent CLIs the engine config binds
// its model tiers to.
//
// The suite gates on these: with tiers pointing at crush or codex, a
// missing binary would otherwise surface as a confusing mid-walk
// failure instead of an honest skip.
func RequiredCLIs(configPath string) ([]string, error) {
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read engine config %s: %w", configPath, err)
	}

	clis := make([]string, 0, supportedCLICount)

	for _, match := range modelRefPattern.FindAllStringSubmatch(string(raw), -1) {
		if !slices.Contains(clis, match[1]) {
			clis = append(clis, match[1])
		}
	}

	slices.Sort(clis)

	return clis, nil
}
