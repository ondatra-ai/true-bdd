package enginecoverage

import (
	"cmp"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

var (
	errMalformedProfile = errors.New("malformed coverage profile line")
	errNoModule         = errors.New("go.mod declares no module")
	errOutsideModule    = errors.New("profile names a file outside the module")
)

// position is a line and byte column, both from 1, as the profile and
// go/token both count them.
type position struct{ line, col int }

func (p position) before(other position) bool {
	return p.line < other.line || (p.line == other.line && p.col < other.col)
}

// block is one basic block of the merged profile: a span of one file, how
// many statements it holds and how many times any run reached it.
type block struct {
	file       string
	start, end position
	statements int
	count      int
}

// parseProfile reads a text profile (`mode:` line, then one block per line),
// folding repeats of a block into one, in file then source order.
func parseProfile(text string) ([]block, error) {
	type key struct {
		file       string
		start, end position
	}

	merged := map[key]block{}

	for line := range strings.Lines(text) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "mode:") {
			continue
		}

		parsed, err := parseBlock(line)
		if err != nil {
			return nil, err
		}

		id := key{parsed.file, parsed.start, parsed.end}
		if seen, ok := merged[id]; ok {
			parsed.count += seen.count
		}

		merged[id] = parsed
	}

	blocks := make([]block, 0, len(merged))
	for _, each := range merged {
		blocks = append(blocks, each)
	}

	slices.SortFunc(blocks, func(left, right block) int {
		if byFile := cmp.Compare(left.file, right.file); byFile != 0 {
			return byFile
		}

		return cmp.Or(cmp.Compare(left.start.line, right.start.line),
			cmp.Compare(left.start.col, right.start.col))
	})

	return blocks, nil
}

// parseBlock reads `file.go:12.34,15.2 3 0`.
func parseBlock(line string) (block, error) {
	colon := strings.LastIndex(line, ":")
	if colon < 0 {
		return block{}, fmt.Errorf("%w: %q", errMalformedProfile, line)
	}

	parsed := block{file: line[:colon]}

	_, err := fmt.Sscanf(line[colon+1:], "%d.%d,%d.%d %d %d",
		&parsed.start.line, &parsed.start.col, &parsed.end.line, &parsed.end.col,
		&parsed.statements, &parsed.count)
	if err != nil {
		return block{}, fmt.Errorf("%w: %q: %w", errMalformedProfile, line, err)
	}

	return parsed, nil
}

// modulePath is the module go.mod at root declares; the profile names every
// file by it.
func modulePath(root string) (string, error) {
	raw, err := disk.Read(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("read go.mod: %w", err)
	}

	for line := range strings.Lines(string(raw)) {
		name, found := strings.CutPrefix(strings.TrimSpace(line), "module ")
		if found {
			return strings.Trim(strings.TrimSpace(name), `"`), nil
		}
	}

	return "", fmt.Errorf("%w: %s", errNoModule, root)
}

// relativeFile turns the profile's import-path file name into a path from the
// module root.
func relativeFile(module, file string) (string, error) {
	rel, found := strings.CutPrefix(file, module+"/")
	if !found {
		return "", fmt.Errorf("%w %s: %s", errOutsideModule, module, file)
	}

	return rel, nil
}
