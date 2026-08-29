package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// runtimeDir is the single declared runtime path excluded from tree
// hashes (true-bdd.yaml paths.tmp_glob): only the root-level tmp/
// subtree — a nested services/foo/tmp/ is still hashed.
const runtimeDir = "tmp"

// HashTree returns a path → sha256(hex) map for every file under root.
// Keys are slash-separated paths relative to root, directly comparable
// with the TypeScript oracle (tests/legacy/bdd-web-playwright/helpers/tree-hash.ts).
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

		data, readErr := disk.Read(path)
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
