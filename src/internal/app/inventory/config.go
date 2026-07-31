package inventory

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Canonical default paths (plan §2: canonical paths only in v1). These
// mirror the engine's config seed and the hardcoded CLI defaults
// (src/cmd/us.go registry path, src/cmd/build.go architecture default).
const (
	defaultEpicsDir         = "docs/prd/epics"
	defaultStoriesDir       = "docs/prd/stories"
	defaultChecklistsDir    = "true-bdd/checklists"
	defaultPRDPath          = "docs/prd/prd.yaml"
	defaultArchitecturePath = "docs/architecture/architecture.yaml"
	// registryRelPath is HARDCODED in src/cmd/us.go (not a config key),
	// so the scanner hardcodes it too.
	registryRelPath = "docs/scenarios.yaml"
	// configRelPath is the engine config file the ViperConfig loader
	// reads (true-bdd/true-bdd.yaml).
	configRelPath = "true-bdd/true-bdd.yaml"
)

// rawConfig mirrors only the true-bdd.yaml keys the scanner needs. Parsed
// directly (never via the Viper loader) so a malformed file degrades to
// an `invalid` chip with canonical-default paths rather than aborting.
type rawConfig struct {
	Epics struct {
		Path string `yaml:"path"`
	} `yaml:"epics"`
	Paths struct {
		StoriesDir    string `yaml:"stories_dir"`
		ChecklistsDir string `yaml:"checklists_dir"`
	} `yaml:"paths"`
	Documents struct {
		PRD              string `yaml:"prd"`
		ArchitectureYAML string `yaml:"architecture_yaml"`
	} `yaml:"documents"`
}

// resolvedConfig is the scanner's view of the host config after applying
// per-field defaults. Directory/file fields are folder-relative (cleaned);
// the scanner joins them onto the absolute folder as needed.
type resolvedConfig struct {
	configStatus string
	configError  string

	epicsRel      string
	storiesRel    string
	checklistsRel string
	prdRel        string

	architectureRel      string
	architectureMismatch bool
}

// resolveConfig reads <folder>/true-bdd/true-bdd.yaml, classifies the
// config chip, and returns the resolved (defaulted) relative paths. A
// missing or unparseable config still yields a usable resolvedConfig
// backed by canonical defaults.
func resolveConfig(folder string) resolvedConfig {
	defaults := resolvedConfig{
		configStatus:    StatusMissing,
		epicsRel:        defaultEpicsDir,
		storiesRel:      defaultStoriesDir,
		checklistsRel:   defaultChecklistsDir,
		prdRel:          defaultPRDPath,
		architectureRel: defaultArchitecturePath,
	}

	data, err := os.ReadFile(filepath.Join(folder, configRelPath))
	if err != nil {
		return defaults
	}

	var raw rawConfig

	err = yaml.Unmarshal(data, &raw)
	if err != nil {
		defaults.configStatus = StatusInvalid
		defaults.configError = err.Error()

		return defaults
	}

	return applyConfig(defaults, raw)
}

// applyConfig overlays the parsed config onto the defaults, computing the
// architecture path-mismatch (plan §2: divergence reported + warned,
// mirrored not fixed).
func applyConfig(base resolvedConfig, raw rawConfig) resolvedConfig {
	base.configStatus = StatusPresent
	base.epicsRel = defaultRel(raw.Epics.Path, defaultEpicsDir)
	base.storiesRel = defaultRel(raw.Paths.StoriesDir, defaultStoriesDir)
	base.checklistsRel = defaultRel(raw.Paths.ChecklistsDir, defaultChecklistsDir)
	base.prdRel = defaultRel(raw.Documents.PRD, defaultPRDPath)
	base.architectureRel = defaultRel(raw.Documents.ArchitectureYAML, defaultArchitecturePath)
	base.architectureMismatch = base.architectureRel != defaultArchitecturePath

	return base
}

// defaultRel returns cleanRel(value) or the fallback when value is empty.
func defaultRel(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}

	return cleanRel(value)
}

// cleanRel normalizes a folder-relative path: it strips a leading "./"
// and collapses redundant separators so config paths compare equal to
// the registry's stored story paths regardless of "./" prefixes.
func cleanRel(path string) string {
	return filepath.Clean(strings.TrimSpace(path))
}
