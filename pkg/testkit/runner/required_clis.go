package runner

import (
	"fmt"
	"regexp"
	"slices"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// modelRefPattern matches an `engine.models` entry's `"<cli>:<model>"`
// value and captures the cli. Deliberately a scan, not a YAML parse:
// this package can't import the engine's internal config packages.
var modelRefPattern = regexp.MustCompile(`(?m)^\s+\w+:\s*"?([a-z]+):[^"\n]+"?\s*$`)

// supportedCLICount sizes the result: claude, crush, codex.
const supportedCLICount = 3

// RequiredCLIs returns the distinct agent CLIs the engine config binds
// its model tiers to. The suite gates on these, so a missing binary
// surfaces as an honest skip instead of a confusing mid-walk failure.
func RequiredCLIs(configPath string) ([]string, error) {
	raw, err := disk.Read(configPath)
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
