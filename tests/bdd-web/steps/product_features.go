package steps

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// ErrNoFeatureEntry is returned when the features file declares no feature by
// the id a clause names.
var ErrNoFeatureEntry = errors.New("the features file declares no such feature")

// keyListPattern is the key list a clause names: quoted names joined by " and ".
const keyListPattern = `"[^"]+"(?: and "[^"]+")*`

// featureEntry is one declared feature, kept as its raw mapping so a clause
// about which keys it carries reads them rather than a decoded subset.
type featureEntry map[string]any

// productFeatures are the features the file declares, in the order it does.
func productFeatures(state *State, relPath string) ([]featureEntry, error) {
	raw, err := fixtureFile(state, relPath)
	if err != nil {
		return nil, err
	}

	var file struct {
		Features []featureEntry `yaml:"features"`
	}

	err = yaml.Unmarshal([]byte(raw), &file)
	if err != nil {
		return nil, state.fail("parsing %s: %w", relPath, err)
	}

	return file.Features, nil
}

// featureByID is one declared feature, found by the id it carries.
func featureByID(state *State, relPath, featureID string) (featureEntry, error) {
	features, err := productFeatures(state, relPath)
	if err != nil {
		return nil, err
	}

	for _, feature := range features {
		if fmt.Sprint(feature["id"]) == featureID {
			return feature, nil
		}
	}

	return nil, state.fail("%w: %s in %s", ErrNoFeatureEntry, featureID, relPath)
}

// readFeatureKeys reads one declared feature's own keys as a reader, so a clause
// polls the file rather than one reading of it.
func readFeatureKeys(state *State, relPath, id string) func() (string, error) {
	return func() (string, error) {
		feature, err := featureByID(state, relPath, id)
		if err != nil {
			return "", err
		}

		return entryKeys(feature), nil
	}
}

// keyNames are the names a key-list clause quotes, sorted, so the comparison is
// about the set rather than the order the step happens to write them in.
func keyNames(list string) []string {
	parts := strings.Split(list, " and ")
	names := make([]string, 0, len(parts))

	for _, part := range parts {
		names = append(names, unquote(part))
	}

	slices.Sort(names)

	return names
}

// entryKeys are one entry's own keys, rendered the way keyNames renders a step's.
func entryKeys(entry featureEntry) string {
	names := make([]string, 0, len(entry))

	for name := range entry {
		names = append(names, name)
	}

	slices.Sort(names)

	return strings.Join(names, ", ")
}
