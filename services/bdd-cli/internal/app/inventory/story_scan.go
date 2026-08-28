package inventory

import (
	"fmt"
	"path/filepath"
	"unicode/utf8"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	storymodel "github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/story"
	"gopkg.in/yaml.v3"
)

// retainedRawCap bounds only the story `raw` text RETAINED in the snapshot
// (plan §1a) for the modal's Raw tab; the scanner still decodes the
// COMPLETE file regardless, so content never flips on a truncated prefix.
const retainedRawCap = 256 * 1024

// lineageIDFormat mirrors the engine's us apply lineage id derivation
// (`%s-%03d`, story_scenario_parser.go) so the scanner counts against the
// exact ids us apply would merge.
const lineageIDFormat = "%s-%03d"

// legacyStoryProbe mirrors only the legacy `scenarios.test_scenarios[]`
// block, used to detect the deprecated 3.x format; identity and ACs come
// from the real typed model instead, so rejection mirrors us apply exactly.
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
	declared   storymodel.Story
	storiesDir string // absolute
	storiesRel string // folder-relative, matches registry story paths
	lineage    lineageIndex
}

// storyResolution is the structured result of resolving one epic story
// row to its file(s): created status, match/source, raw (possibly
// truncated), extracted content, and parse error.
type storyResolution struct {
	created      string
	matchCount   int
	sourceFile   string
	file         storyFile
	raw          string
	rawTruncated bool
	content      *Content
	parseErr     string
}

// scanStoryRow resolves the story file for one epic row and computes its
// created/applied/refined cells, identity flags, and review-modal content
// (plan §1/§1b); duplicate_declared_id is applied by the caller.
func scanStoryRow(input storyRowInput) Story {
	declared := input.declared
	story := Story{
		CreateID:        fmt.Sprintf("%d.%d", input.epicNumber, input.position),
		EpicNumber:      input.epicNumber,
		Position:        input.position,
		DeclaredID:      declared.ID,
		Refined:         RefinedNotRecorded,
		DeclaredTitle:   declared.Title,
		DeclaredStatus:  declared.Status,
		DeclaredContent: declaredContent(declared),
	}

	resolution := resolveStory(input.storiesDir, declared.ID)
	story.Created = resolution.created
	story.MatchCount = resolution.matchCount
	story.SourceFile = resolution.sourceFile
	story.Raw = resolution.raw
	story.RawTruncated = resolution.rawTruncated
	story.Error = resolution.parseErr

	if resolution.created != CreatedOne {
		story.Applied = Applied{Status: AppliedUnknown, Reason: createdReason(resolution.created)}

		return story
	}

	story.FileID = resolution.file.internalID
	story.Content = resolution.content
	story.Flags = storyFlags(resolution.file, declared.ID)
	story.Applied = countApplied(resolution.file, input.storiesRel, input.lineage)

	return story
}

// resolveStory globs <storiesDir>/<declaredID>-*.yaml (mirroring
// StoryLoader), then parses the single match into a structured result so
// the review modal can render the raw file, content, and any parse error.
func resolveStory(storiesDir, declaredID string) storyResolution {
	if declaredID == "" {
		return storyResolution{created: CreatedMissing}
	}

	matches, err := filepath.Glob(filepath.Join(storiesDir, declaredID+"-*.yaml"))
	if err != nil || len(matches) == 0 {
		return storyResolution{created: CreatedMissing}
	}

	if len(matches) > 1 {
		return storyResolution{created: CreatedAmbiguous, matchCount: len(matches)}
	}

	resolution := storyResolution{matchCount: 1, sourceFile: filepath.Base(matches[0])}

	data, err := disk.Read(matches[0]) //nolint:gosec // path is a globbed host story file
	if err != nil {
		resolution.created = CreatedInvalid
		resolution.parseErr = err.Error()

		return resolution
	}

	// Retain a (possibly truncated) raw copy for the modal's Raw tab; decode
	// the COMPLETE file regardless (plan §1a).
	resolution.raw, resolution.rawTruncated = retainRaw(data)

	file, content, err := parseStory(matches[0], data)
	if err != nil {
		resolution.created = CreatedInvalid
		resolution.parseErr = err.Error()

		return resolution
	}

	resolution.created = CreatedOne
	resolution.file = file
	resolution.content = content

	return resolution
}

// retainRaw returns the story text retained for the modal Raw tab: the
// whole file when within the cap, else a valid-UTF-8 prefix (cut on a rune
// boundary) with truncated=true.
func retainRaw(data []byte) (string, bool) {
	if len(data) <= retainedRawCap {
		return string(data), false
	}

	prefix := data[:retainedRawCap]
	// Back off any trailing incomplete rune so the retained prefix is valid
	// UTF-8 (a multibyte sequence split by the byte cap decodes as
	// RuneError with size <= 1).
	for len(prefix) > 0 {
		last, size := utf8.DecodeLastRune(prefix)
		if last == utf8.RuneError && size <= 1 {
			prefix = prefix[:len(prefix)-1]

			continue
		}

		break
	}

	return string(prefix), true
}

// parseStory decodes via the REAL typed model, so a malformed step (e.g. a scalar `- 42`)
// fails here as created=invalid rather than a misleading counted lineage; a deprecated-format
// story still decodes cleanly (created=one) and is flagged apply-ineligible instead.
func parseStory(path string, data []byte) (storyFile, *Content, error) {
	var legacy legacyStoryProbe

	err := yaml.Unmarshal(data, &legacy)
	if err != nil {
		return storyFile{}, nil, fmt.Errorf("parse story file %s: %w", path, err)
	}

	var typed storymodel.StoryDocument

	err = yaml.Unmarshal(data, &typed)
	if err != nil {
		return storyFile{}, nil, fmt.Errorf("validate story file %s against typed model: %w", path, err)
	}

	file := storyFile{
		basename:   filepath.Base(path),
		internalID: typed.Story.ID,
		acCount:    len(typed.Story.AcceptanceCriteria),
		deprecated: len(legacy.Scenarios.TestScenarios) > 0,
	}

	return file, contentFromStory(typed.Story), nil
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

// countApplied derives the story-applied cell. Eligibility (deprecated,
// zero ACs, empty id) is checked BEFORE the registry, so an ineligible
// story keeps its own reason even when the registry is also unusable.
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
