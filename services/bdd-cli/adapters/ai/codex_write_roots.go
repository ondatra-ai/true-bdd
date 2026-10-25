package ai

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

func codexWritePatterns(mode ExecutionMode, temporary string) []string {
	patterns := mode.WriteGlobs()
	for _, root := range mode.SourceWriteRoots {
		if slices.Contains(patterns, root) {
			continue
		}

		pattern := sourceRootGlob(root)
		if pattern != "" && !slices.Contains(patterns, pattern) {
			patterns = append(patterns, pattern)
		}
	}

	return append(patterns, temporary+"/**")
}

func codexWritableRoots(config *strings.Builder, root string, patterns []string) error {
	granted := make(map[string]bool)

	for _, pattern := range patterns {
		paths, directory, err := codexExpandWritePattern(root, pattern)
		if err != nil {
			return err
		}

		for _, path := range paths {
			resolved, err := codexWriteRoot(root, path)
			if err != nil {
				return err
			}

			if granted[resolved] {
				continue
			}

			err = codexPrepareWriteRoot(root, resolved, directory)
			if err != nil {
				return err
			}

			fmt.Fprintf(config, "%s = \"write\"\n", strconv.Quote(resolved))
			granted[resolved] = true
		}
	}

	return nil
}

func codexPrepareWriteRoot(root, path string, directory bool) error {
	target := path
	if !directory {
		info, err := os.Stat(path)
		if err == nil && info.IsDir() {
			return fmt.Errorf("codex file root is a directory %q: %w", path, os.ErrPermission)
		}

		target = filepath.Dir(path)
	}

	relative, err := filepath.Rel(root, target)
	if err != nil {
		return fmt.Errorf("locate codex edit root: %w", err)
	}

	created, err := disk.DirWithin(root, relative, disk.Shared)
	if err != nil {
		return fmt.Errorf("prepare codex edit root: %w", err)
	}

	if created {
		slog.Info("Codex edit root created", "path", target)
	}

	return nil
}

func codexExpandWritePattern(root, pattern string) ([]string, bool, error) {
	directory := strings.HasSuffix(pattern, "/**")

	pattern = strings.TrimSuffix(pattern, "/**")
	if !filepath.IsAbs(pattern) {
		pattern = filepath.Join(root, pattern)
	}

	if !strings.ContainsAny(pattern, "*?[") {
		_, err := codexInsideRoot(root, pattern)

		return []string{pattern}, directory, err
	}

	base, matchPattern, suffix := codexGlobAnchor(pattern, directory)
	if base != root {
		_, err := codexInsideRoot(root, base)
		if err != nil {
			return nil, false, err
		}
	}

	paths, err := codexWalkWritePattern(root, base, matchPattern, suffix, directory)
	if err != nil {
		return nil, false, fmt.Errorf("expand codex write glob %q: %w", pattern, err)
	}

	if len(paths) == 0 {
		return nil, false, fmt.Errorf("codex write glob %q has no existing matches: %w", pattern, os.ErrNotExist)
	}

	slices.Sort(paths)

	return paths, directory, nil
}

func codexMatchGlob(pattern, path []string) (bool, error) {
	if len(pattern) == 0 {
		return len(path) == 0, nil
	}

	if pattern[0] == "**" {
		for offset := 0; offset <= len(path); offset++ {
			match, err := codexMatchGlob(pattern[1:], path[offset:])
			if err != nil || match {
				return match, err
			}
		}

		return false, nil
	}

	if len(path) == 0 {
		return false, nil
	}

	match, err := filepath.Match(pattern[0], path[0])
	if err != nil {
		return false, fmt.Errorf("invalid codex write glob: %w", err)
	}

	if !match {
		return false, nil
	}

	return codexMatchGlob(pattern[1:], path[1:])
}

func codexGlobAnchor(pattern string, directory bool) (string, string, string) {
	fixed := pattern[:strings.IndexAny(pattern, "*?[")]

	base := filepath.Dir(fixed)
	if strings.HasSuffix(fixed, "/") {
		base = filepath.Clean(fixed)
	}

	if !directory {
		return base, pattern, ""
	}

	parts := strings.Split(pattern, "/")

	last := len(parts) - 1
	for !strings.ContainsAny(parts[last], "*?[") {
		last--
	}

	return base, strings.Join(parts[:last+1], "/"), filepath.Join(parts[last+1:]...)
}

func codexWalkWritePattern(root, base, pattern, suffix string, directory bool) ([]string, error) {
	var paths []string

	err := filepath.WalkDir(base, func(path string, _ fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		matched, err := codexMatchGlob(strings.Split(pattern, "/"), strings.Split(path, "/"))
		if err != nil {
			return err
		}

		if matched {
			target, err := codexMatchedWriteRoot(root, path, suffix, directory)
			if err != nil {
				return err
			}

			if target != "" {
				paths = append(paths, target)
			}
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan codex write roots: %w", err)
	}

	return paths, nil
}

func codexMatchedWriteRoot(root, path, suffix string, directory bool) (string, error) {
	resolved := root

	var err error
	if path != root || suffix == "" {
		resolved, err = codexInsideRoot(root, path)
		if err != nil {
			return "", err
		}
	}

	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("inspect codex glob match: %w", err)
	}

	if directory && !info.IsDir() {
		return "", nil
	}

	if !directory && info.IsDir() {
		return "", fmt.Errorf("codex file glob matches directory %q: %w", path, os.ErrPermission)
	}

	return filepath.Join(resolved, suffix), nil
}

func codexWriteRoot(root, path string) (string, error) {
	resolved, err := codexInsideRoot(root, path)
	if err != nil {
		return "", err
	}

	protected, err := codexInsideRoot(root, filepath.Join(root, "tmp", "codex-homes"))
	if err != nil {
		return "", err
	}

	folded := strings.ToLower(resolved)

	private := strings.ToLower(protected)
	if folded == private || strings.HasPrefix(folded, private+string(os.PathSeparator)) {
		return "", fmt.Errorf("codex write root reaches generated homes %q: %w", path, os.ErrPermission)
	}

	return resolved, nil
}
