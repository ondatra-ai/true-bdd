package inventory

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"

	models "github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models"
	storymodel "github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/story"
	"gopkg.in/yaml.v3"
)

// epicFilenameRE extracts the raw filename-number digits from an epic
// file (epic-NN-*.yaml). The digits are kept so the scanner can decide
// whether the encoding is the canonical %02d form epic_loader.go's glob
// (epic-%02d-*.yaml) resolves — the number itself is parsed without
// leading zeros for the UI's data-epic-number contract.
var epicFilenameRE = regexp.MustCompile(`^epic-(\d+)-.*\.yaml$`)

// epicScanInput bundles the resolved directories the epic scan walks.
type epicScanInput struct {
	epicsDir   string // absolute
	storiesDir string // absolute
	storiesRel string
	lineage    lineageIndex
}

// parsedEpic is one epic's cheaply-parsed identity plus its epic-declared
// story list — the metadata the incremental (budget > 0) scan needs BEFORE
// it reads any story file (finding 2). Reading the epic document is cheap
// relative to reading N story files, so parsing all epic docs first lets the
// scanner then stream and fit story rows one at a time.
type parsedEpic struct {
	header    Epic
	declared  []storymodel.Story
	canonical bool
}

// scanEpics globs the epics directory, resolves each epic's identity and
// story rows, then marks epics sharing a filename number as duplicates
// (plan §3.4). Epics are ordered by filename for stable rendering. This is
// the UNLIMITED (budget <= 0) path: it retains every enriched story.
func scanEpics(input epicScanInput) []Epic {
	parsed := parseEpicHeaders(input)

	epics := make([]Epic, 0, len(parsed))
	for index := range parsed {
		epic := parsed[index].header
		if parsed[index].canonical {
			epic.Stories = scanEpicStories(epic.Number, parsed[index].declared, input)
		}

		epics = append(epics, epic)
	}

	markDuplicateNumbers(epics)
	markDuplicateDeclaredIDs(epics)

	return epics
}

// scanEpicsFitted is the INCREMENTAL (budget > 0) path (finding 2): it parses
// every epic doc cheaply, then reads and fits story rows ONE AT A TIME
// through the snapshotBuilder so aggregate retained memory is bounded by the
// budget plus one in-flight story. Duplicate-number and duplicate-declared-id
// flags are computed from the parsed metadata (which is fully known before
// any story file is read) so a fitted row still carries the same flags the
// unlimited path would set.
func scanEpicsFitted(input epicScanInput, base Snapshot, budget int) Snapshot {
	parsed := parseEpicHeaders(input)
	dupNumbers := duplicateFilenameNumbers(parsed)
	dupDeclared := duplicateDeclaredIDs(parsed)

	totalStories := 0
	for index := range parsed {
		totalStories += len(parsed[index].declared)
	}

	base.Totals = Totals{Epics: len(parsed), DeclaredStories: totalStories}
	builder := newSnapshotBuilder(base, budget)

	for index := range parsed {
		header := parsed[index].header
		if dupNumbers[header.Number] {
			header.DuplicateNumber = true
		}

		if !builder.addEpicHeader(header) {
			break
		}

		for position := range parsed[index].declared {
			row := scanStoryRow(storyRowInput{
				epicNumber: header.Number,
				position:   position + 1,
				declared:   parsed[index].declared[position],
				storiesDir: input.storiesDir,
				storiesRel: input.storiesRel,
				lineage:    input.lineage,
			})
			if row.DeclaredID != "" && dupDeclared[row.DeclaredID] {
				row.Flags.DuplicateDeclaredID = true
			}

			if !builder.addStory(row) {
				break
			}
		}

		if builder.full {
			break
		}
	}

	return builder.finalize(totalStories)
}

// parseEpicHeaders globs + sorts the epics directory and parses each epic
// document (identity + declared stories) WITHOUT reading any story file.
func parseEpicHeaders(input epicScanInput) []parsedEpic {
	matches, err := filepath.Glob(filepath.Join(input.epicsDir, "epic-*.yaml"))
	if err != nil {
		return nil
	}

	sort.Strings(matches)

	out := make([]parsedEpic, 0, len(matches))
	for _, path := range matches {
		parsed, ok := parseEpicHeader(path)
		if ok {
			out = append(out, parsed)
		}
	}

	return out
}

// parseEpicHeader classifies one epic file and returns its identity plus the
// epic-declared story list. A filename that carries no epic number is not an
// epic and is skipped (ok=false). A filename whose number is not the
// canonical %02d encoding epic_loader.go's glob resolves is still listed
// (with its identity), but marked NoncanonicalFilename and given NO declared
// stories: `us create <n>.<x>` cannot find epic-%02d-*.yaml, so none of its
// stories are Create-addressable and advertising them would be dishonest.
//
// Decode uses the REAL typed models.EpicDocument (§1c): its declared stories
// carry the same rejecting ScenarioStep semantics us create enforces, so a
// YAML-valid epic with an invalid step is `invalid` with no rows — the
// harness never renders Create controls us create rejects.
func parseEpicHeader(path string) (parsedEpic, bool) {
	number, canonical, ok := epicNumber(filepath.Base(path))
	if !ok {
		return parsedEpic{}, false
	}

	header := Epic{File: filepath.Base(path), Number: number, NoncanonicalFilename: !canonical}

	data, err := os.ReadFile(path) //nolint:gosec // path is a globbed host epic file
	if err != nil {
		header.Status = EpicInvalid
		header.Error = err.Error()

		return parsedEpic{header: header}, true
	}

	var doc models.EpicDocument

	err = yaml.Unmarshal(data, &doc)
	if err != nil {
		header.Status = EpicInvalid
		header.Error = err.Error()

		return parsedEpic{header: header}, true
	}

	header.Status = EpicParseable
	header.Title = doc.Epic.Name
	header.DocID = doc.Epic.ID
	header.IDMismatch = doc.Epic.ID != 0 && doc.Epic.ID != number

	parsed := parsedEpic{header: header, canonical: canonical}
	if canonical {
		parsed.declared = doc.Stories
	}

	return parsed, true
}

// scanEpicStories builds the story rows for a parseable epic's declared
// stories. Duplicate epic-declared story ids are flagged snapshot-wide by
// markDuplicateDeclaredIDs after every epic is scanned (unlimited path), not
// here. The whole declared story (its title/status/statement/ACs) is passed
// through so each row carries the epic-declared content fallback (plan §1b).
func scanEpicStories(number int, declared []storymodel.Story, input epicScanInput) []Story {
	stories := make([]Story, 0, len(declared))

	for index := range declared {
		stories = append(stories, scanStoryRow(storyRowInput{
			epicNumber: number,
			position:   index + 1,
			declared:   declared[index],
			storiesDir: input.storiesDir,
			storiesRel: input.storiesRel,
			lineage:    input.lineage,
		}))
	}

	return stories
}

// epicNumber parses the filename number from an epic-NN-*.yaml basename
// and reports whether the encoding is the canonical %02d form
// epic_loader.go resolves. `epic-42-*` and `epic-07-*` are canonical
// (%02d of 42 is "42", of 7 is "07"); `epic-7-*` and `epic-042-*` are
// not — the loader glob would never find them.
func epicNumber(basename string) (int, bool, bool) {
	groups := epicFilenameRE.FindStringSubmatch(basename)
	if groups == nil {
		return 0, false, false
	}

	digits := groups[1]

	number, err := strconv.Atoi(digits)
	if err != nil {
		return 0, false, false
	}

	canonical := digits == fmt.Sprintf("%02d", number)

	return number, canonical, true
}

// markDuplicateNumbers flags every epic that shares its filename number
// with another epic — the glob epic-%02d-*.yaml can no longer resolve
// `us create N.x` unambiguously.
func markDuplicateNumbers(epics []Epic) {
	counts := make(map[int]int, len(epics))
	for index := range epics {
		counts[epics[index].Number]++
	}

	for index := range epics {
		if counts[epics[index].Number] > 1 {
			epics[index].DuplicateNumber = true
		}
	}
}

// duplicateFilenameNumbers reports which filename numbers are shared by more
// than one epic — the incremental path's equivalent of markDuplicateNumbers,
// computed from the parsed headers before any row is built.
func duplicateFilenameNumbers(parsed []parsedEpic) map[int]bool {
	counts := make(map[int]int, len(parsed))
	for index := range parsed {
		counts[parsed[index].header.Number]++
	}

	dup := make(map[int]bool)

	for number, count := range counts {
		if count > 1 {
			dup[number] = true
		}
	}

	return dup
}

// markDuplicateDeclaredIDs flags every displayed story row whose declared
// id appears more than once across the ENTIRE snapshot — not just within
// one epic. Two epics that both declare `70.1` collide on `us refine
// 70.1` / `us apply 70.1` just as two rows in one epic do, so both rows
// must raise the flag. Empty declared ids are skipped: a missing id is
// its own state (created=missing), not a cross-row duplicate.
func markDuplicateDeclaredIDs(epics []Epic) {
	counts := make(map[string]int)

	for epicIdx := range epics {
		for storyIdx := range epics[epicIdx].Stories {
			declaredID := epics[epicIdx].Stories[storyIdx].DeclaredID
			if declaredID != "" {
				counts[declaredID]++
			}
		}
	}

	for epicIdx := range epics {
		for storyIdx := range epics[epicIdx].Stories {
			story := &epics[epicIdx].Stories[storyIdx]
			if story.DeclaredID != "" && counts[story.DeclaredID] > 1 {
				story.Flags.DuplicateDeclaredID = true
			}
		}
	}
}

// duplicateDeclaredIDs reports which epic-declared story ids appear more
// than once across all parsed epics — the incremental path's equivalent of
// markDuplicateDeclaredIDs, computed from the parsed declarations.
func duplicateDeclaredIDs(parsed []parsedEpic) map[string]bool {
	counts := make(map[string]int)

	for index := range parsed {
		for _, declared := range parsed[index].declared {
			if declared.ID != "" {
				counts[declared.ID]++
			}
		}
	}

	dup := make(map[string]bool)

	for id, count := range counts {
		if count > 1 {
			dup[id] = true
		}
	}

	return dup
}
