package inventory

import (
	"fmt"
	"os"
	"path/filepath"

	storymodel "github.com/ondatra-ai/true-bdd/src/internal/domain/models/story"
	"gopkg.in/yaml.v3"
)

// lineageIDFormat mirrors the engine's us apply lineage id derivation
// (`%s-%03d`, story_scenario_parser.go) so the scanner counts against the
// exact ids us apply would merge.
const lineageIDFormat = "%s-%03d"

// legacyStoryProbe mirrors only the legacy `scenarios.test_scenarios[]`
// block the scanner needs to detect the deprecated 3.x story format. The
// canonical identity and acceptance-criteria list come from the real
// typed model (storymodel.StoryDocument) instead, so the scanner rejects
// exactly the malformed step shapes us apply rejects.
type legacyStoryProbe struct {
	Scenarios struct {
		TestScenarios []yaml.Node `yaml:"test_scenarios"`
	} `yaml:"scenarios"`
}

// storyFile is the parsed view of a resolved story file. basename is the
// resolved filename, joined onto the stories dir to match the exact path
// the registry stores.
type storyFile struct {
	basename   string
	internalID string
	acCount    int
	deprecated bool
}

// storyRowInput bundles what scanStoryRow needs for one epic story row.
type storyRowInput struct {
	epicNumber int
	position   int
	declaredID string
	storiesDir string // absolute
	storiesRel string // folder-relative, matches registry story paths
	lineage    lineageIndex
}

// scanStoryRow resolves the story file for one epic row and computes its
// created / applied / refined cells and identity flags (plan §3.4). The
// epic-level duplicate_declared_id flag is applied by the caller.
func scanStoryRow(input storyRowInput) Story {
	story := Story{
		CreateID:   fmt.Sprintf("%d.%d", input.epicNumber, input.position),
		EpicNumber: input.epicNumber,
		Position:   input.position,
		DeclaredID: input.declaredID,
		Refined:    RefinedNotRecorded,
	}

	created, file, ok := resolveStoryFile(input.storiesDir, input.declaredID)
	story.Created = created

	if !ok {
		story.Applied = Applied{Status: AppliedUnknown, Reason: createdReason(created)}

		return story
	}

	story.FileID = file.internalID
	story.Flags = storyFlags(file, input.declaredID)
	story.Applied = countApplied(file, input.storiesRel, input.lineage)

	return story
}

// resolveStoryFile globs <storiesDir>/<declaredID>-*.yaml (mirroring the
// StoryLoader) and parses the single match. It returns the created status,
// the parsed file, and whether the file resolved to exactly one parseable
// document.
func resolveStoryFile(storiesDir, declaredID string) (string, storyFile, bool) {
	if declaredID == "" {
		return CreatedMissing, storyFile{}, false
	}

	matches, err := filepath.Glob(filepath.Join(storiesDir, declaredID+"-*.yaml"))
	if err != nil || len(matches) == 0 {
		return CreatedMissing, storyFile{}, false
	}

	if len(matches) > 1 {
		return CreatedAmbiguous, storyFile{}, false
	}

	file, err := parseStoryFile(matches[0])
	if err != nil {
		return CreatedInvalid, storyFile{}, false
	}

	return CreatedOne, file, true
}

// parseStoryFile reads and decodes a story file into the fields the
// scanner reasons about. Honest surface: the story identity and AC list
// are taken from the REAL typed domain model (storymodel.StoryDocument),
// whose ScenarioStep/StepStatement decoders reject malformed step shapes
// (a scalar step like `- 42`, a bad `{and|but}` modifier). A story the
// real model rejects fails here and is surfaced as created=invalid
// (applied unknown(invalid)) — the same honest state resolveStoryFile
// already gives a syntactically unparseable file — instead of a
// misleading created=one with a counted lineage. A deprecated-format
// story still decodes cleanly here (its ACs are well-formed) and keeps
// its own apply-ineligibility (deprecated_format) via the legacy probe.
func parseStoryFile(path string) (storyFile, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is a globbed host story file
	if err != nil {
		return storyFile{}, fmt.Errorf("read story file %s: %w", path, err)
	}

	var legacy legacyStoryProbe

	err = yaml.Unmarshal(data, &legacy)
	if err != nil {
		return storyFile{}, fmt.Errorf("parse story file %s: %w", path, err)
	}

	var typed storymodel.StoryDocument

	err = yaml.Unmarshal(data, &typed)
	if err != nil {
		return storyFile{}, fmt.Errorf("validate story file %s against typed model: %w", path, err)
	}

	return storyFile{
		basename:   filepath.Base(path),
		internalID: typed.Story.ID,
		acCount:    len(typed.Story.AcceptanceCriteria),
		deprecated: len(legacy.Scenarios.TestScenarios) > 0,
	}, nil
}

// storyFlags computes the per-row identity flags for a resolved story
// file. duplicate_declared_id is epic-level and set by the caller.
func storyFlags(file storyFile, declaredID string) StoryFlags {
	return StoryFlags{
		DeprecatedFormat: file.deprecated,
		NoACs:            file.acCount == 0,
		EmptyInternalID:  file.internalID == "",
		IDMismatch:       file.internalID != "" && file.internalID != declaredID,
	}
}

// countApplied derives the story-applied cell for a resolved, parseable
// story. Apply ELIGIBILITY is checked before counting: a story that us
// apply would reject (deprecated format, zero ACs, empty internal id) is
// reported unknown(<reason>) rather than a misleading x/y. Story
// eligibility is checked BEFORE the registry so those reasons survive
// even when the registry is absent. Only for an eligible story does the
// registry status taint the cell (missing/invalid → unknown), otherwise
// counting keys on the EXACT registry story path and position-derived
// lineage ids.
func countApplied(file storyFile, storiesRel string, lineage lineageIndex) Applied {
	if reason, ok := appliedIneligibility(file); ok {
		return Applied{Status: AppliedUnknown, Reason: reason}
	}

	if reason, ok := registryTaint(lineage.status); ok {
		return Applied{Status: AppliedUnknown, Reason: reason}
	}

	storyPath := filepath.Join(storiesRel, file.basename)
	applied := 0

	for position := 1; position <= file.acCount; position++ {
		lineageID := fmt.Sprintf(lineageIDFormat, file.internalID, position)
		if lineage.isCovered(storyPath, lineageID) {
			applied++
		}
	}

	return Applied{Status: AppliedCounted, Applied: applied, Total: file.acCount}
}

// appliedIneligibility returns the unknown reason for a story us apply
// would reject before ever reading the registry, and whether one applies.
func appliedIneligibility(file storyFile) (string, bool) {
	switch {
	case file.deprecated:
		return ReasonDeprecatedFormat, true
	case file.acCount == 0:
		return ReasonNoAcceptanceCriteria, true
	case file.internalID == "":
		return ReasonEmptyInternalID, true
	default:
		return "", false
	}
}

// registryTaint maps a non-ok registry status to the applied unknown
// reason it forces on an otherwise-eligible story, and whether the cell
// is tainted. A present-but-empty registry is registryOK and counts 0/y.
func registryTaint(status registryStatus) (string, bool) {
	switch status {
	case registryMissing:
		return ReasonRegistryMissing, true
	case registryInvalid:
		return ReasonRegistryInvalid, true
	case registryOK:
		return "", false
	default:
		return "", false
	}
}

// createdReason maps a non-resolvable created status to its applied
// unknown reason.
func createdReason(created string) string {
	switch created {
	case CreatedAmbiguous:
		return ReasonAmbiguous
	case CreatedInvalid:
		return ReasonInvalid
	default:
		return ReasonMissing
	}
}
