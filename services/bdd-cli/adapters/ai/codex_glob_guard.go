package ai

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/disk"
)

type codexGlobGuard struct {
	root     string
	patterns []string
	bases    []string
	before   map[string]codexGlobFile
}

type codexGlobFile struct {
	mode   fs.FileMode
	digest [sha256.Size]byte
	link   string
}

func newCodexGlobGuard(req Request) (*codexGlobGuard, error) {
	root, err := filepath.EvalSymlinks(req.WorkDir)
	if err != nil {
		return nil, fmt.Errorf("resolve codex glob workspace: %w", err)
	}

	guard := &codexGlobGuard{root: root}
	for _, pattern := range codexWritePatterns(req.Mode, filepath.Join(root, "tmp")) {
		if !filepath.IsAbs(pattern) {
			pattern = filepath.Join(root, pattern)
		}

		literal := strings.TrimSuffix(pattern, "/**")

		base := literal
		if strings.ContainsAny(literal, "*?[") {
			base = codexGlobBase(literal)
		}

		resolved, err := codexWriteRoot(root, base)
		if err != nil {
			return nil, err
		}

		guard.patterns = append(guard.patterns, resolved+strings.TrimPrefix(pattern, base))
		if base != literal && !slices.Contains(guard.bases, resolved) {
			guard.bases = append(guard.bases, resolved)
		}
	}

	guard.before, err = guard.snapshot()

	return guard, err
}

func (g *codexGlobGuard) allows(path string) (bool, error) {
	for _, pattern := range g.patterns {
		match, err := codexMatchGlob(strings.Split(pattern, "/"), strings.Split(path, "/"))
		if err != nil || match {
			return match, err
		}
	}

	return false, nil
}

func (g *codexGlobGuard) snapshot() (map[string]codexGlobFile, error) {
	files := make(map[string]codexGlobFile)

	for _, base := range g.bases {
		err := filepath.WalkDir(base, func(path string, entry fs.DirEntry, walkErr error) error {
			if errors.Is(walkErr, fs.ErrNotExist) {
				return nil
			}

			if walkErr != nil {
				return walkErr
			}

			if entry.IsDir() {
				protected := filepath.Join(g.root, "tmp", "codex-homes")
				if strings.EqualFold(path, protected) {
					return filepath.SkipDir
				}

				return nil
			}

			allowed, err := g.allows(path)
			if err != nil {
				return err
			}

			if allowed {
				_, err := codexWriteRoot(g.root, path)

				return err
			}

			state, err := g.fileState(path, entry)
			if err != nil {
				return err
			}

			files[path] = state

			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("snapshot codex glob directory: %w", err)
		}
	}

	return files, nil
}

func (g *codexGlobGuard) fileState(path string, entry fs.DirEntry) (codexGlobFile, error) {
	info, err := entry.Info()
	if err != nil {
		return codexGlobFile{}, fmt.Errorf("inspect codex glob file: %w", err)
	}

	state := codexGlobFile{mode: info.Mode()}

	previous, existed := g.before[path]
	if g.before != nil && (!existed || previous.mode != state.mode) {
		return state, nil
	}

	if info.Mode().IsRegular() {
		data, err := disk.Read(path)
		if err != nil {
			return state, fmt.Errorf("read codex glob file: %w", err)
		}

		state.digest = sha256.Sum256(data)
	}

	if info.Mode()&os.ModeSymlink != 0 {
		state.link, err = os.Readlink(path)
		if err != nil {
			return state, fmt.Errorf("read codex glob link: %w", err)
		}
	}

	return state, nil
}

func (g *codexGlobGuard) enforce() error {
	after, err := g.snapshot()
	if err != nil {
		return err
	}

	var failures []error

	for path, state := range after {
		previous, existed := g.before[path]
		if !existed {
			err := disk.Remove(path)
			if err != nil {
				failures = append(failures, fmt.Errorf("remove unmatched codex file: %w", err))
			}
		} else if previous != state {
			failures = append(failures, codexGlobViolation(path))
		}
	}

	for path := range g.before {
		if _, exists := after[path]; !exists {
			failures = append(failures, codexGlobViolation(path))
		}
	}

	return errors.Join(failures...)
}

func codexGlobViolation(path string) error {
	slog.Warn("Codex changed file outside write globs", "path", path)

	return fmt.Errorf("codex changed file outside write globs %q: %w", path, os.ErrPermission)
}
