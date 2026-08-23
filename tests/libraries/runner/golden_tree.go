package runner

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode/utf8"
)

// GoldenFile is the recorded outcome's name inside a fixture's
// cassettes/ directory: the cassettes are what the models said, this
// is what the engine then produced.
const GoldenFile = "golden.json"

// goldenSchema versions the on-disk shape.
const goldenSchema = 1

// goldenTextLimit caps the inline copy of a file's content. Above it,
// only the digest is kept: the digest is what the comparison uses, the
// text only exists so a failure can show a diff instead of two hashes.
const goldenTextLimit = 64 * 1024

// GoldenTree is a fixture's recorded outcome: every file the run
// created, modified, or deleted outside its own tmp/ scratch — tmp/
// excluded since its timestamped paths differ on every run.
type GoldenTree struct {
	Schema  int           `json:"schema"`
	Fixture string        `json:"fixture"`
	Files   []GoldenEntry `json:"files"`
}

// GoldenEntry is one recorded file change.
type GoldenEntry struct {
	Path string `json:"path"`
	// Kind is created | modified | deleted, matching the run diff.
	Kind   string `json:"kind"`
	Size   int    `json:"size"`
	SHA256 string `json:"sha256"`
	// Text is the content verbatim, present only for valid UTF-8 under
	// goldenTextLimit. Diagnostics only — SHA256 is what decides.
	Text string `json:"text,omitempty"`
}

// NewGoldenTree projects a run's diff into a recordable outcome.
func NewGoldenTree(fixture string, diff []FileChange) GoldenTree {
	golden := GoldenTree{Schema: goldenSchema, Fixture: fixture, Files: []GoldenEntry{}}

	for _, change := range diff {
		if isScratchPath(change.Path) {
			continue
		}

		golden.Files = append(golden.Files, newGoldenEntry(change))
	}

	sort.Slice(golden.Files, func(i, j int) bool { return golden.Files[i].Path < golden.Files[j].Path })

	return golden
}

func newGoldenEntry(change FileChange) GoldenEntry {
	entry := GoldenEntry{Path: change.Path, Kind: change.Kind}

	// A deletion's claim is that the file is gone; there is no content to
	// digest, and After is empty for exactly that reason.
	if change.Kind == "deleted" {
		return entry
	}

	sum := sha256.Sum256(change.After)
	entry.Size = len(change.After)
	entry.SHA256 = hex.EncodeToString(sum[:])

	if len(change.After) <= goldenTextLimit && utf8.Valid(change.After) {
		entry.Text = string(change.After)
	}

	return entry
}

// isScratchPath reports whether a diff entry is per-run scratch rather
// than a claim about behaviour.
func isScratchPath(path string) bool {
	return path == "tmp" || strings.HasPrefix(path, "tmp/")
}

// WriteGolden persists a recorded outcome into a cassette directory.
func WriteGolden(dir string, golden GoldenTree) error {
	blob, err := json.MarshalIndent(golden, "", "  ")
	if err != nil {
		return fmt.Errorf("encode golden tree: %w", err)
	}

	err = os.WriteFile(goldenPath(dir), append(blob, '\n'), filePerm)
	if err != nil {
		return fmt.Errorf("write golden tree: %w", err)
	}

	return nil
}

// ReadGolden loads a recorded outcome. A cassette directory without one
// was recorded before goldens existed, which the caller must report
// rather than silently treat as "nothing was expected".
func ReadGolden(dir string) (*GoldenTree, error) {
	blob, err := os.ReadFile(goldenPath(dir))
	if err != nil {
		return nil, fmt.Errorf("read golden tree: %w", err)
	}

	var golden GoldenTree

	err = json.Unmarshal(blob, &golden)
	if err != nil {
		return nil, fmt.Errorf("decode golden tree: %w", err)
	}

	if golden.Schema != goldenSchema {
		return nil, fmt.Errorf("%w: golden tree schema %d, want %d",
			ErrGoldenUnreadable, golden.Schema, goldenSchema)
	}

	return &golden, nil
}

func goldenPath(dir string) string {
	return dir + string(os.PathSeparator) + GoldenFile
}
