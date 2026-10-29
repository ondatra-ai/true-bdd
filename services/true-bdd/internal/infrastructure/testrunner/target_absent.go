package testrunner

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
)

var errAbsentCopy = errors.New("cannot copy Host project without target")

// CopyWithoutTarget copies the Host project, omitting service trees, VCS metadata and scratch.
func CopyWithoutTarget(host, destination string, services []string) error {
	omitted, err := absentPaths(services)
	if err != nil {
		return err
	}

	err = filepath.WalkDir(host, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk Host project: %w", walkErr)
		}

		relative, err := filepath.Rel(host, path)
		if err != nil {
			return fmt.Errorf("locate copied path: %w", err)
		}

		if omitted[relative] {
			if entry.IsDir() {
				return filepath.SkipDir
			}

			return nil
		}

		return copyHostEntry(filepath.Join(destination, relative), path, entry)
	})
	if err != nil {
		return fmt.Errorf("%w: %w", errAbsentCopy, err)
	}

	return nil
}

// CopiedConfig preserves the declared command's working directory inside the copy.
func CopiedConfig(cfg Config, host, destination string) (Config, error) {
	where := CommandDir(cfg)
	if where == "" {
		where = host
	}

	absolute, err := filepath.Abs(where)
	if err != nil {
		return cfg, fmt.Errorf("locate command: %w", err)
	}

	relative, err := filepath.Rel(host, absolute)
	if err != nil {
		return cfg, fmt.Errorf("locate command in Host project: %w", err)
	}

	if outsideCopy(relative) {
		return cfg, fmt.Errorf("%w: command directory %s", errAbsentCopy, where)
	}

	cfg.ConfigFile = filepath.Join(destination, relative, "go.mod")

	return cfg, nil
}

func absentPaths(services []string) (map[string]bool, error) {
	omitted := map[string]bool{".git": true, "tmp": true}

	for _, service := range services {
		path := filepath.Clean(service)
		if filepath.IsAbs(path) || path == "." || outsideCopy(path) {
			return nil, fmt.Errorf("%w: service path %s", errAbsentCopy, service)
		}

		omitted[path] = true
	}

	return omitted, nil
}

func outsideCopy(path string) bool {
	return path == ".." || strings.HasPrefix(path, ".."+string(os.PathSeparator))
}

func copyHostEntry(target, path string, entry fs.DirEntry) error {
	if entry.Type()&fs.ModeSymlink != 0 {
		return fmt.Errorf("%w: %s", disk.ErrSymlink, path)
	}

	if entry.IsDir() {
		return disk.Dir(target, disk.Shared)
	}

	err := disk.Copy(target, path, disk.Shared)
	if err != nil {
		return err
	}

	info, err := entry.Info()
	if err != nil {
		return fmt.Errorf("inspect copied path: %w", err)
	}

	if info.Mode()&0o111 != 0 {
		return disk.Executable(target)
	}

	return nil
}
