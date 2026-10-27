package ai

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/disk"
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

	base := codexGlobBase(pattern)

	_, err := codexWriteRoot(root, base)
	if err != nil {
		return nil, false, err
	}

	for _, part := range strings.Split(pattern, "/") {
		_, err := filepath.Match(part, "")
		if err != nil {
			return nil, false, fmt.Errorf("invalid codex write glob: %w", err)
		}
	}

	return []string{base}, true, nil
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

func codexGlobBase(pattern string) string {
	fixed := pattern[:strings.IndexAny(pattern, "*?[")]
	if strings.HasSuffix(fixed, "/") {
		return filepath.Clean(fixed)
	}

	return filepath.Dir(fixed)
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
