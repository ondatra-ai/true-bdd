// Package fstree provides content snapshots and structural diffs of
// directory trees. Extracted from the BDD runner so the harness and the
// aiproxy record/replay shim (tests/aiproxy) share one diff algorithm:
// the runner diffs a fixture's whole tmpdir around the CLI run, the
// shim diffs the same tree around a single AI-CLI call.
package fstree

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ErrSymlinkInTree is returned when a snapshot meets a symbolic link.
// The tree model stores file contents, so a link can be neither
// recorded faithfully nor reproduced — and following it would read
// outside the root the snapshot claims to describe.
var (
	ErrSymlinkInTree = errors.New("fstree: symbolic link in snapshot tree")

	// ErrHardLinkInTree is returned for a regular file with more than one
	// link. Its content may live outside the root, and a replay writing
	// through it would escape the tmpdir the run is confined to.
	ErrHardLinkInTree = errors.New("fstree: hard-linked file in snapshot tree")
)

// Change describes one file's difference between two snapshots.
type Change struct {
	Path   string // path relative to the snapshot root
	Kind   string // "created", "modified", "deleted"
	Before []byte // empty for "created"
	After  []byte // empty for "deleted"
}

// Snapshot reads every file under root into a path → content map.
// skipDirs entries are root-relative paths (e.g. "tmp", ".git") whose
// subtrees are excluded from the snapshot entirely — the walk does not
// descend into them.
func Snapshot(root string, skipDirs ...string) (map[string][]byte, error) {
	out := make(map[string][]byte)

	walkErr := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return fmt.Errorf("filepath rel: %w", relErr)
		}

		if entry.IsDir() {
			if isSkipped(rel, skipDirs) {
				return fs.SkipDir
			}

			return nil
		}

		if isSkipped(rel, skipDirs) {
			return nil
		}

		escapeErr := checkEscape(entry, rel)
		if escapeErr != nil {
			return escapeErr
		}

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("read %s: %w", path, readErr)
		}

		out[rel] = data

		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("walk %s: %w", root, walkErr)
	}

	return out, nil
}

// checkEscape rejects the two entries whose content may not belong to
// the tree being snapshotted.
//
// os.ReadFile FOLLOWS symlinks, so a link planted inside the root pulls
// outside content into the diff — and, in record mode, into a committed
// cassette. Replay could not honour it either: Change carries bytes, not
// link targets. A hard link is the same escape wearing different
// clothes: the entry IS a regular file, so Type() says nothing, and the
// link count is the only signal the walk gets.
func checkEscape(entry fs.DirEntry, rel string) error {
	if entry.Type()&fs.ModeSymlink != 0 {
		return fmt.Errorf("%w: %s", ErrSymlinkInTree, rel)
	}

	info, err := entry.Info()
	if err != nil {
		return fmt.Errorf("stat %s: %w", rel, err)
	}

	if hardLinked(info) {
		return fmt.Errorf("%w: %s", ErrHardLinkInTree, rel)
	}

	return nil
}

// isSkipped reports whether a root-relative path lies in an excluded
// subtree.
//
// A skip entry containing a separator ("docs/history") is anchored at
// the root, and matches only there. A bare name ("node_modules") matches
// that directory at ANY depth — because that is where these trees
// actually appear: a fixture's `prep:` runs `npm install --prefix
// tests`, so the tree lands at tests/node_modules, and a root-anchored
// rule would walk straight into a directory full of symlinks into a
// package cache.
func isSkipped(rel string, skipDirs []string) bool {
	for _, skip := range skipDirs {
		if rel == skip || strings.HasPrefix(rel, skip+string(filepath.Separator)) {
			return true
		}

		if !strings.ContainsRune(skip, filepath.Separator) && hasComponent(rel, skip) {
			return true
		}
	}

	return false
}

func hasComponent(rel, name string) bool {
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if part == name {
			return true
		}
	}

	return false
}

// Diff compares two snapshots (path → content) and returns the list of
// created/modified/deleted entries, sorted by path. Callers are
// responsible for taking the snapshots — this lets them bracket exactly
// the window they care about without extra file IO.
func Diff(before, after map[string][]byte) []Change {
	var changes []Change

	for path, beforeBytes := range before {
		afterBytes, present := after[path]
		if !present {
			changes = append(changes, Change{
				Path: path, Kind: "deleted", Before: beforeBytes,
			})

			continue
		}

		if !bytes.Equal(beforeBytes, afterBytes) {
			changes = append(changes, Change{
				Path: path, Kind: "modified", Before: beforeBytes, After: afterBytes,
			})
		}
	}

	for path, afterBytes := range after {
		if _, present := before[path]; !present {
			changes = append(changes, Change{
				Path: path, Kind: "created", After: afterBytes,
			})
		}
	}

	sort.Slice(changes, func(i, j int) bool { return changes[i].Path < changes[j].Path })

	return changes
}
