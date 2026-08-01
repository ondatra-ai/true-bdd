package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// runtimeDir is the single declared runtime path excluded from tree
// hashes: the engine's `./tmp/**` working directory (per
// true-bdd.yaml paths.tmp_glob and plan §4.5 — "excluding only
// declared runtime paths (`tmp/**`), not broad directories"). Only
// the ROOT-level tmp/ subtree is excluded; a nested
// services/foo/tmp/ would still be hashed.
const runtimeDir = "tmp"

// HashTree returns a path → sha256(hex) map for every file under
// root. Keys are slash-separated paths relative to root, so the map
// is directly comparable with the TypeScript oracle
// (tests/harness/helpers/tree-hash.ts), which implements the
// identical algorithm.
func HashTree(root string) (map[string]string, error) {
	out := make(map[string]string)

	walkErr := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return fmt.Errorf("filepath rel: %w", relErr)
		}

		if entry.IsDir() {
			if rel == runtimeDir {
				return fs.SkipDir
			}

			return nil
		}

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("read %s: %w", path, readErr)
		}

		sum := sha256.Sum256(data)
		out[filepath.ToSlash(rel)] = hex.EncodeToString(sum[:])

		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("walk %s: %w", root, walkErr)
	}

	return out, nil
}
