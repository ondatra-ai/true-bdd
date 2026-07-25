package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// fixtureManifest is the slice of fixture.yaml this tool needs.
type fixtureManifest struct {
	Cmd string `yaml:"cmd"`
}

// DiscoverChecklistStem determines which checklist a fixture run
// executed, as the hyphenated command stem ("us apply 99.3 --fix" →
// "us-apply"). It prefers the source fixture manifest and falls back to
// the log's "Loaded prompts" command; a conflict is an error. An empty
// stem with nil error means "no checklist command" (e.g. help-flag).
func DiscoverChecklistStem(fixturesDir, fixtureName string, segments []logSegment) (string, error) {
	manifestStem, manifestErr := stemFromManifest(fixturesDir, fixtureName)
	logStem := stemFromLog(segments)

	if manifestErr == nil && manifestStem != "" {
		if logStem != "" && logStem != manifestStem {
			return "", fmt.Errorf("%w: manifest says %q, log says %q",
				errCommandConflict, manifestStem, logStem)
		}

		return manifestStem, nil
	}

	if logStem != "" {
		return logStem, nil
	}

	if manifestErr == nil {
		return "", nil // manifest exists but has no checklist command (--help)
	}

	return "", nil // neither source available: treated as no-command run
}

// stemFromManifest reads tests/bdd/fixtures/<name>/fixture.yaml.
func stemFromManifest(fixturesDir, fixtureName string) (string, error) {
	data, err := os.ReadFile(filepath.Join(fixturesDir, fixtureName, "fixture.yaml"))
	if err != nil {
		return "", fmt.Errorf("reading fixture manifest: %w", err)
	}

	var manifest fixtureManifest

	err = yaml.Unmarshal(data, &manifest)
	if err != nil {
		return "", fmt.Errorf("parsing fixture manifest: %w", err)
	}

	return stemFromCmd(manifest.Cmd), nil
}

// stemFromCmd hyphenates the leading command words of a CLI invocation
// ("us apply 99.3 --fix" → "us-apply"), returning "" for non-checklist
// invocations like "--help".
func stemFromCmd(cmd string) string {
	fields := strings.Fields(cmd)

	words := make([]string, 0, len(fields))

	for _, field := range fields {
		if strings.HasPrefix(field, "-") || isStoryOrDigit(field) {
			break
		}

		words = append(words, field)
	}

	return strings.Join(words, "-")
}

// isStoryOrDigit reports whether the token is an argument (story id or
// number) rather than a subcommand word.
func isStoryOrDigit(field string) bool {
	return field != "" && field[0] >= '0' && field[0] <= '9'
}

// stemFromLog derives the stem from the first segment's Loaded prompts
// command field.
func stemFromLog(segments []logSegment) string {
	for _, seg := range segments {
		for _, event := range seg.Events {
			if event.Kind == evLoadedPrompts && event.Command != "" {
				return strings.ReplaceAll(event.Command, " ", "-")
			}
		}
	}

	return ""
}
