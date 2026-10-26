package ai

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// resolveWriterPath follows existing and dangling links before reattaching
// missing suffixes. Keep parent traversal lexical until links resolve.
func resolveWriterPath(path string) (string, error) {
	if path != string(os.PathSeparator) {
		path = strings.TrimRight(path, string(os.PathSeparator))
	}

	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return resolved, nil
	}

	if !errors.Is(err, os.ErrNotExist) || path == "" {
		return "", fmt.Errorf("resolve writer path: %w", err)
	}

	return resolveMissingWriterPath(path)
}

func resolveMissingWriterPath(path string) (string, error) {
	info, err := os.Lstat(path)
	if err == nil && info.Mode()&os.ModeSymlink != 0 {
		return resolveWriterLink(path)
	}

	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("inspect writer path: %w", err)
	}

	parent, leaf := filepath.Split(path)

	parent = strings.TrimSuffix(parent, string(os.PathSeparator))
	if parent == "" {
		parent = string(os.PathSeparator)
	}

	if parent == path {
		return "", fmt.Errorf("resolve writer ancestor: %w", os.ErrNotExist)
	}

	resolved, err := resolveWriterPath(parent)
	if err != nil {
		return "", err
	}

	return filepath.Join(resolved, leaf), nil
}

func resolveWriterLink(path string) (string, error) {
	target, err := os.Readlink(path)
	if err != nil {
		return "", fmt.Errorf("read writer link: %w", err)
	}

	if !filepath.IsAbs(target) {
		target = filepath.Dir(path) + string(os.PathSeparator) + target
	}

	return resolveWriterPath(target)
}
