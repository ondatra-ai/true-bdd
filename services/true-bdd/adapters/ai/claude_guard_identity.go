package ai

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// claudeSamePath compares existing inodes, or missing suffixes from their
// nearest existing ancestors. Resolve links before interpreting parent paths.
func claudeSamePath(left, right string) bool {
	left = resolveThroughSymlinks(left)

	right = resolveThroughSymlinks(right)
	if left == "" || right == "" {
		return false
	}

	return claudeSameResolvedPath(left, right)
}

func claudeSameResolvedPath(left, right string) bool {
	for {
		leftInfo, leftErr := os.Stat(left)

		rightInfo, rightErr := os.Stat(right)
		if leftErr == nil && rightErr == nil {
			return os.SameFile(leftInfo, rightInfo)
		}

		if !errors.Is(leftErr, fs.ErrNotExist) || !errors.Is(rightErr, fs.ErrNotExist) {
			return false
		}

		if filepath.Base(left) != filepath.Base(right) || filepath.Dir(left) == left || filepath.Dir(right) == right {
			return false
		}

		left, right = filepath.Dir(left), filepath.Dir(right)
	}
}

func claudePathWithinRoot(target, root string) bool {
	target = resolveThroughSymlinks(target)

	root = resolveThroughSymlinks(root)
	if target == "" || root == "" {
		return false
	}

	for {
		if claudeSamePath(target, root) {
			return true
		}

		parent := filepath.Dir(target)
		if parent == target {
			return false
		}

		target = parent
	}
}
