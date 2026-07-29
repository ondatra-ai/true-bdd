package inventory_test

// Content-extraction goldens (plan §1/§4 "Go"). These pin the TARGET
// snapshot wire the scanner must emit for the story review popup: per-story
// declared_title/declared_status/match_count/source_file/error/content/raw
// (+ raw_truncated) and the epic Title. To stay COMPILE-SAFE against a
// schema that does not exist yet, every assertion decodes the REAL
// inventory.Scan output as JSON into LOCAL target structs (never
// yet-nonexistent fields on the real types): the tests compile today and
// fail (red) because Scan does not populate the new fields, and go green
// once the scanner emits them. The implementer builds to these structs.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/ondatra-ai/true-bdd/src/internal/app/inventory"
)

// ── Local target-schema decode structs (plan §1, all additive) ──

type gStep struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

type gAC struct {
	ID          string  `json:"id"`
	Description string  `json:"description"`
	Steps       []gStep `json:"steps"`
}

type gStatement struct {
	AsA    string `json:"as_a"`
	IWant  string `json:"i_want"`
	SoThat string `json:"so_that"`
}

type gContent struct {
	ID                 string     `json:"id"`
	Title              string     `json:"title"`
	Status             string     `json:"status"`
	Statement          gStatement `json:"statement"`
	AcceptanceCriteria []gAC      `json:"acceptance_criteria"`
}

type gStory struct {
	CreateID        string    `json:"create_id"`
	DeclaredID      string    `json:"declared_id"`
	FileID          string    `json:"file_id"`
	Created         string    `json:"created"`
	DeclaredTitle   string    `json:"declared_title"`
	DeclaredStatus  string    `json:"declared_status"`
	MatchCount      int       `json:"match_count"`
	SourceFile      string    `json:"source_file"`
	Error           string    `json:"error"`
	Content         *gContent `json:"content"`
	DeclaredContent *gContent `json:"declared_content"`
	Raw             string    `json:"raw"`
	RawTruncated    bool      `json:"raw_truncated"`
	RawOmitted      bool      `json:"raw_omitted"`
	ContentOmitted  bool      `json:"content_omitted"`
	OmissionReason  string    `json:"omission_reason"`
}

type gEpic struct {
	File                 string   `json:"file"`
	Number               int      `json:"number"`
	Status               string   `json:"status"`
	Title                string   `json:"title"`
	NoncanonicalFilename bool     `json:"noncanonical_filename"`
	Error                string   `json:"error"`
	Stories              []gStory `json:"stories"`
}

type gTotals struct {
	Epics           int `json:"epics"`
	DeclaredStories int `json:"declared_stories"`
}

type gSnapshot struct {
	Epics             []gEpic `json:"epics"`
	SnapshotTruncated bool    `json:"snapshot_truncated"`
	StoriesOmitted    int     `json:"stories_omitted"`
	Unavailable       string  `json:"unavailable"`
	Totals            gTotals `json:"totals"`
}

// scanJSON scans a folder and round-trips the REAL Snapshot through JSON
// into the local target schema — the compile-safe way to assert against
// fields the real types do not yet carry.
func scanJSON(t *testing.T, folder string) gSnapshot {
	t.Helper()

	data, err := json.Marshal(inventory.Scan(folder))
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}

	var snap gSnapshot

	err = json.Unmarshal(data, &snap)
	if err != nil {
		t.Fatalf("unmarshal snapshot into target schema: %v", err)
	}

	return snap
}

// gStoryOf finds the target story row for a create id.
func gStoryOf(t *testing.T, snap gSnapshot, createID string) gStory {
	t.Helper()

	for _, epic := range snap.Epics {
		for _, story := range epic.Stories {
			if story.CreateID == createID {
				return story
			}
		}
	}

	t.Fatalf("story %q not found in target snapshot", createID)

	return gStory{}
}

// gEpicOf finds the target epic row for a filename.
func gEpicOf(t *testing.T, snap gSnapshot, file string) gEpic {
	t.Helper()

	for _, epic := range snap.Epics {
		if epic.File == file {
			return epic
		}
	}

	t.Fatalf("epic %q not found in target snapshot", file)

	return gEpic{}
}

func stepKinds(ac gAC) []string {
	kinds := make([]string, 0, len(ac.Steps))
	for _, step := range ac.Steps {
		kinds = append(kinds, step.Kind)
	}

	return kinds
}

func TestGoldenEpicTitleFromName(t *testing.T) {
	t.Parallel()

	snap := scanJSON(t, materialize(t, "p3-inventory-spread"))
	epic := gEpicOf(t, snap, "epic-60-inventory-spread.yaml")

	if epic.Title != "Inventory Spread Epic" {
		t.Fatalf("epic title = %q, want %q (from the document name:)", epic.Title, "Inventory Spread Epic")
	}
}

func TestGoldenContentHappyFlattensAndAndBut(t *testing.T) {
	t.Parallel()

	folder := materialize(t, "p3-inventory-spread")
	snap := scanJSON(t, folder)
	story := gStoryOf(t, snap, "60.2")

	if story.Content == nil {
		t.Fatal("60.2 content is nil, want parsed content")
	}

	if story.Content.Statement.IWant != "Claude to give me a short summary of a shared Google Doc when I ask for one" {
		t.Fatalf("60.2 i_want = %q", story.Content.Statement.IWant)
	}

	if len(story.Content.AcceptanceCriteria) != 2 {
		t.Fatalf("60.2 has %d ACs, want 2", len(story.Content.AcceptanceCriteria))
	}

	// AC-1 flattens given/when/then + an `and` modifier, in source order.
	ac1 := story.Content.AcceptanceCriteria[0]
	if got := stepKinds(ac1); strings.Join(got, ",") != "given,when,then,and" {
		t.Fatalf("AC-1 step kinds = %v, want [given when then and]", got)
	}

	// AC-2 flattens given/when/then + a `but` modifier (the real model
	// supports BOTH and AND but).
	ac2 := story.Content.AcceptanceCriteria[1]
	if got := stepKinds(ac2); strings.Join(got, ",") != "given,when,then,but" {
		t.Fatalf("AC-2 step kinds = %v, want [given when then but]", got)
	}

	if last := ac1.Steps[len(ac1.Steps)-1]; last.Text != "the summary names the document title" {
		t.Fatalf("AC-1 and-step text = %q", last.Text)
	}

	// Raw is the verbatim file (byte-exact) — tied to the file, not duplicated.
	rawBytes, err := os.ReadFile(filepath.Join(folder, "docs", "prd", "stories", "60.2-summary-shared-docs.yaml"))
	if err != nil {
		t.Fatalf("read story file: %v", err)
	}

	if story.Raw != string(rawBytes) {
		t.Fatalf("60.2 raw does not match the file bytes (got %d bytes, want %d)", len(story.Raw), len(rawBytes))
	}

	if story.RawTruncated {
		t.Fatal("60.2 raw_truncated = true for a small file, want false")
	}
}

func TestGoldenMissingStoryDeclaredFallback(t *testing.T) {
	t.Parallel()

	snap := scanJSON(t, materialize(t, "p3-inventory-spread"))
	story := gStoryOf(t, snap, "60.1")

	if story.Created != inventory.CreatedMissing {
		t.Fatalf("60.1 created = %q, want missing", story.Created)
	}

	if story.Content != nil {
		t.Fatal("60.1 content should be nil (no story file)")
	}

	if story.DeclaredTitle != "Summary Length Preference" || story.DeclaredStatus != "PLANNED" {
		t.Fatalf("60.1 declared fallback = (%q,%q)", story.DeclaredTitle, story.DeclaredStatus)
	}

	if story.DeclaredContent == nil || story.DeclaredContent.Statement.IWant == "" {
		t.Fatal("60.1 declared_content must carry the epic-declared statement (every story openable)")
	}

	if story.SourceFile != "" {
		t.Fatalf("60.1 source_file = %q, want empty (no resolved file)", story.SourceFile)
	}
}

func TestGoldenAmbiguousMatchCount(t *testing.T) {
	t.Parallel()

	snap := scanJSON(t, materialize(t, "p4-story-ambiguous-files"))
	story := gStoryOf(t, snap, "70.1")

	if story.Created != inventory.CreatedAmbiguous {
		t.Fatalf("70.1 created = %q, want ambiguous", story.Created)
	}

	if story.MatchCount != 2 {
		t.Fatalf("70.1 match_count = %d, want 2", story.MatchCount)
	}

	if story.SourceFile != "" {
		t.Fatalf("70.1 source_file = %q, want empty (ambiguous)", story.SourceFile)
	}

	if story.DeclaredContent == nil {
		t.Fatal("70.1 declared_content must be present so the ambiguous story is still openable")
	}
}

func TestGoldenSyntaxInvalidStoryCarriesRawAndError(t *testing.T) {
	t.Parallel()

	folder := materialize(t, "p4-story-malformed")
	snap := scanJSON(t, folder)
	story := gStoryOf(t, snap, "70.1")

	if story.Created != inventory.CreatedInvalid {
		t.Fatalf("70.1 created = %q, want invalid", story.Created)
	}

	if story.Content != nil {
		t.Fatal("invalid story must have nil content")
	}

	if story.Error == "" {
		t.Fatal("invalid story must carry a parse error string")
	}

	// Raw is preserved verbatim even for an unparseable file (the modal Raw tab).
	rawBytes, err := os.ReadFile(filepath.Join(folder, "docs", "prd", "stories", "70.1-broken.yaml"))
	if err != nil {
		t.Fatalf("read broken story: %v", err)
	}

	if story.Raw != string(rawBytes) {
		t.Fatalf("invalid story raw does not match file bytes (got %d, want %d)", len(story.Raw), len(rawBytes))
	}

	// The epic declares 70.1, so declared_content still backs the Review tab.
	if story.DeclaredContent == nil {
		t.Fatal("declared_content must back the Review tab of an invalid-but-declared story")
	}
}

func TestGoldenTypedModelInvalidStoryIsInvalidWithoutContent(t *testing.T) {
	t.Parallel()

	// A YAML-valid story the REAL typed model rejects (a scalar scenario
	// step) must be created=invalid with NO content, distinct from a
	// syntactically-broken file — mirroring us apply's typed decoder.
	folder := honestyTree(t, map[string]string{
		"docs/prd/epics/epic-70-probe.yaml": "epic:\n  id: 70\n  name: Probe Epic\nstories:\n  - id: \"70.1\"\n" +
			"    title: Probe\n    as_a: user\n    i_want: x\n    so_that: y\n",
		"docs/prd/stories/70.1-probe.yaml": "story:\n  id: \"70.1\"\n  acceptance_criteria:\n    - id: AC-1\n" +
			"      description: rule\n      steps:\n        - 42\n",
	})

	story := gStoryOf(t, scanJSON(t, folder), "70.1")
	if story.Created != inventory.CreatedInvalid {
		t.Fatalf("scalar-step story created = %q, want invalid", story.Created)
	}

	if story.Content != nil {
		t.Fatal("typed-invalid story must have nil content")
	}

	if story.DeclaredContent == nil {
		t.Fatal("declared_content must still back the modal (epic declares 70.1)")
	}
}

func TestGoldenRawTruncatedAtUTF8Boundary(t *testing.T) {
	t.Parallel()

	// A story whose file exceeds the 256 KiB retained-raw cap must set
	// raw_truncated and retain a valid-UTF-8 prefix (cut at a rune
	// boundary), while decoding the WHOLE file for its content.
	const capBytes = 256 * 1024
	// Pad the description with a long multibyte run so the byte cut lands
	// inside a multibyte sequence unless the scanner cuts on a rune boundary.
	padding := strings.Repeat("é", capBytes) // 2 bytes each ⇒ ~512 KiB of body
	storyYAML := "story:\n  id: \"70.1\"\n  title: Big Story\n  as_a: user\n  i_want: x\n  so_that: y\n" +
		"  acceptance_criteria:\n    - id: AC-1\n      description: \"" + padding + "\"\n"

	folder := honestyTree(t, map[string]string{
		"docs/prd/epics/epic-70-probe.yaml": "epic:\n  id: 70\n  name: Probe\nstories:\n  - id: \"70.1\"\n",
		"docs/prd/stories/70.1-big.yaml":    storyYAML,
	})

	story := gStoryOf(t, scanJSON(t, folder), "70.1")

	if !story.RawTruncated {
		t.Fatal("oversized story raw_truncated = false, want true")
	}

	if len(story.Raw) > capBytes {
		t.Fatalf("retained raw = %d bytes, want <= %d (cap)", len(story.Raw), capBytes)
	}

	if !utf8.ValidString(story.Raw) {
		t.Fatal("retained raw prefix is not valid UTF-8 (cut mid-rune)")
	}

	// Decoding is over the WHOLE file (not the truncated prefix): content
	// is still extracted from the complete, valid document.
	if story.Content == nil {
		t.Fatal("oversized-but-valid story must still decode its content in full")
	}
}
