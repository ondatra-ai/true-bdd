package inventory

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"

	"gopkg.in/yaml.v3"
)

// epicFilenameRE extracts the raw filename-number digits from an epic
// file (epic-NN-*.yaml). The digits are kept so the scanner can decide
// whether the encoding is the canonical %02d form epic_loader.go's glob
// (epic-%02d-*.yaml) resolves — the number itself is parsed without
// leading zeros for the UI's data-epic-number contract.
var epicFilenameRE = regexp.MustCompile(`^epic-(\d+)-.*\.yaml$`)

// scanEpicDoc mirrors only the epic-document fields the scanner needs: the
// document id and the declared story rows.
type scanEpicDoc struct {
	Epic struct {
		ID int `yaml:"id"`
	} `yaml:"epic"`
	Stories []struct {
		ID string `yaml:"id"`
	} `yaml:"stories"`
}

// epicScanInput bundles the resolved directories the epic scan walks.
type epicScanInput struct {
	epicsDir   string // absolute
	storiesDir string // absolute
	storiesRel string
	lineage    lineageIndex
}

// scanEpics globs the epics directory, resolves each epic's identity and
// story rows, then marks epics sharing a filename number as duplicates
// (plan §3.4). Epics are ordered by filename for stable rendering.
func scanEpics(input epicScanInput) []Epic {
	matches, err := filepath.Glob(filepath.Join(input.epicsDir, "epic-*.yaml"))
	if err != nil {
		return nil
	}

	sort.Strings(matches)

	epics := make([]Epic, 0, len(matches))

	for _, path := range matches {
		epic, ok := scanEpicFile(path, input)
		if ok {
			epics = append(epics, epic)
		}
	}

	markDuplicateNumbers(epics)
	markDuplicateDeclaredIDs(epics)

	return epics
}

// scanEpicFile classifies one epic file. A filename that carries no epic
// number is not an epic and is skipped (ok=false). A filename whose
// number is not the canonical %02d encoding epic_loader.go's glob
// resolves is still listed (with its identity), but marked
// NoncanonicalFilename and given NO story rows: `us create <n>.<x>`
// cannot find epic-%02d-*.yaml, so none of its stories are
// Create-addressable and advertising them would be dishonest.
func scanEpicFile(path string, input epicScanInput) (Epic, bool) {
	number, canonical, ok := epicNumber(filepath.Base(path))
	if !ok {
		return Epic{}, false
	}

	epic := Epic{File: filepath.Base(path), Number: number, NoncanonicalFilename: !canonical}

	data, err := os.ReadFile(path) //nolint:gosec // path is a globbed host epic file
	if err != nil {
		epic.Status = EpicInvalid
		epic.Error = err.Error()

		return epic, true
	}

	var doc scanEpicDoc

	err = yaml.Unmarshal(data, &doc)
	if err != nil {
		epic.Status = EpicInvalid
		epic.Error = err.Error()

		return epic, true
	}

	epic.Status = EpicParseable
	epic.DocID = doc.Epic.ID
	epic.IDMismatch = doc.Epic.ID != 0 && doc.Epic.ID != number

	if canonical {
		epic.Stories = scanEpicStories(number, doc, input)
	}

	return epic, true
}

// scanEpicStories builds the story rows for a parseable epic. Duplicate
// epic-declared story ids are flagged snapshot-wide by
// markDuplicateDeclaredIDs after every epic is scanned, not here.
func scanEpicStories(number int, doc scanEpicDoc, input epicScanInput) []Story {
	stories := make([]Story, 0, len(doc.Stories))

	for index, declared := range doc.Stories {
		stories = append(stories, scanStoryRow(storyRowInput{
			epicNumber: number,
			position:   index + 1,
			declaredID: declared.ID,
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
