package inventory

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// rawRegistry mirrors only the scenario-registry shape the scanner
// needs: the scenarios map and each entry's user_stories lineage list.
type rawRegistry struct {
	Scenarios map[string]struct {
		UserStories []struct {
			Story      string `yaml:"story"`
			ScenarioID string `yaml:"scenario_id"`
		} `yaml:"user_stories"`
	} `yaml:"scenarios"`
}

// registryStatus classifies the scenarios registry the applied cell
// counts against. It is carried through lineageIndex so an eligible
// story is tainted honestly (unknown(registry_missing|registry_invalid))
// when the registry us apply requires cannot be loaded, rather than
// silently rendered counted 0/y. A present-but-empty registry
// (`scenarios: {}`) parses cleanly and stays registryOK — present_empty
// is legitimate coverage, not a taint.
type registryStatus int

const (
	// registryOK — the registry parsed (including a valid empty map).
	registryOK registryStatus = iota
	// registryMissing — no registry file on disk.
	registryMissing
	// registryInvalid — the registry file failed to parse.
	registryInvalid
)

// lineageIndex answers "is (storyPath, lineageID) covered by the
// registry?" — the exact predicate us apply merges against (plan §3.4):
// a registry entry whose user_stories[] references the EXACT story path
// and the position-derived lineage id <internal-id>-NNN. status carries
// whether the registry itself was loadable.
type lineageIndex struct {
	status  registryStatus
	covered map[string]map[string]bool
}

// loadLineageIndex reads the registry and indexes every (story path,
// scenario id) pair, recording whether the registry was missing,
// unparseable, or ok. A valid empty `scenarios:` map is registryOK (the
// registry chip reports present_empty separately); missing/invalid taint
// the applied cell in countApplied.
func loadLineageIndex(path string) lineageIndex {
	index := lineageIndex{covered: make(map[string]map[string]bool)}

	data, err := os.ReadFile(path)
	if err != nil {
		index.status = registryMissing

		return index
	}

	var raw rawRegistry

	err = yaml.Unmarshal(data, &raw)
	if err != nil {
		index.status = registryInvalid

		return index
	}

	index.status = registryOK

	for _, scenario := range raw.Scenarios {
		for _, ref := range scenario.UserStories {
			index.add(ref.Story, ref.ScenarioID)
		}
	}

	return index
}

// add records that lineageID is covered for storyPath (paths normalized
// so "./"-prefixed and clean forms compare equal).
func (l lineageIndex) add(storyPath, lineageID string) {
	key := filepath.Clean(storyPath)

	byLineage, ok := l.covered[key]
	if !ok {
		byLineage = make(map[string]bool)
		l.covered[key] = byLineage
	}

	byLineage[lineageID] = true
}

// isCovered reports whether the registry references storyPath with
// lineageID.
func (l lineageIndex) isCovered(storyPath, lineageID string) bool {
	byLineage, ok := l.covered[filepath.Clean(storyPath)]
	if !ok {
		return false
	}

	return byLineage[lineageID]
}
