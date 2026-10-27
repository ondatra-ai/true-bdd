package architecture

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// ErrInvalidWritableRoot refuses grants outside the host or reaching its tests.
var ErrInvalidWritableRoot = errors.New("invalid architecture writable root")

// WritableRoot distinguishes a folder grant from an exact, possibly absent file.
type WritableRoot struct {
	Path      string
	Directory bool
}

// WritableRoots validates all service grants before any runner or writer starts.
// Test paths name suite directories; tests/ is always reserved.
func WritableRoots(host string, services []Service, testPaths []string) ([]WritableRoot, error) {
	host, err := filepath.EvalSymlinks(host)
	if err != nil {
		return nil, fmt.Errorf("resolve host: %w", err)
	}

	protected := append([]string{"tests"}, testPaths...)

	_, err = writableRoot(host, "tmp", protected, true)
	if err != nil {
		return nil, fmt.Errorf("host tmp: %w", err)
	}

	roots := make([]WritableRoot, 0)

	for _, service := range services {
		entries := append([]string{service.Path}, service.Writable...)
		for index, entry := range entries {
			root, rootErr := writableRoot(host, entry, protected, index == 0)
			if rootErr != nil {
				return nil, fmt.Errorf("service %s: %w", service.Name, rootErr)
			}

			if !slices.Contains(roots, root) {
				roots = append(roots, root)
			}
		}
	}

	return roots, nil
}

func writableRoot(host, entry string, protected []string, directory bool) (WritableRoot, error) {
	err := validateWritableEntry(entry)
	if err != nil {
		return WritableRoot{}, err
	}

	resolved, err := resolveWritablePath(filepath.Join(host, entry), 0)
	if err != nil {
		return WritableRoot{}, err
	}

	if !withinWritable(host, resolved) {
		return WritableRoot{}, fmt.Errorf("%q resolves outside the host: %w", entry, ErrInvalidWritableRoot)
	}

	err = refuseTestOverlap(host, entry, resolved, protected)
	if err != nil {
		return WritableRoot{}, err
	}

	info, statErr := os.Stat(resolved)
	if statErr == nil {
		directory = info.IsDir()
	}

	return WritableRoot{Path: entry, Directory: directory || strings.HasSuffix(entry, "/")}, nil
}

func validateWritableEntry(entry string) error {
	if entry == "" || strings.TrimSpace(entry) != entry ||
		filepath.IsAbs(entry) || strings.ContainsAny(entry, "*?[\\\x00") {
		return fmt.Errorf("%q must be an exact path from the project root: %w", entry, ErrInvalidWritableRoot)
	}

	for _, part := range strings.Split(entry, "/") {
		if part == ".." {
			return fmt.Errorf("%q contains parent traversal: %w", entry, ErrInvalidWritableRoot)
		}
	}

	if filepath.Clean(entry) == "." {
		return fmt.Errorf("%q covers the host: %w", entry, ErrInvalidWritableRoot)
	}

	return nil
}

func refuseTestOverlap(host, entry, resolved string, protected []string) error {
	for _, test := range protected {
		target, err := resolveWritablePath(filepath.Join(host, test), 0)
		if err != nil {
			return err
		}

		if pathsOverlap(filepath.Join(host, entry), filepath.Join(host, test)) || pathsOverlap(resolved, target) {
			return fmt.Errorf("%q overlaps tests or step definitions at %q: %w", entry, test, ErrInvalidWritableRoot)
		}
	}

	return nil
}

func withinWritable(parent, child string) bool {
	relative, err := filepath.Rel(parent, child)

	return err == nil && relative != ".." && !strings.HasPrefix(relative, "../")
}

func pathsOverlap(left, right string) bool {
	left, right = strings.ToLower(left), strings.ToLower(right)

	return withinWritable(left, right) || withinWritable(right, left)
}

// Resolve missing suffixes and dangling links without accepting an outside target.
func resolveWritablePath(path string, links int) (string, error) {
	const maxLinks = 40
	if links > maxLinks {
		return "", fmt.Errorf("symlink loop at %q: %w", path, ErrInvalidWritableRoot)
	}

	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return resolved, nil
	}

	if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("resolve writable path: %w", err)
	}

	info, statErr := os.Lstat(path)
	if statErr == nil && info.Mode()&os.ModeSymlink != 0 {
		target, linkErr := os.Readlink(path)
		if linkErr != nil {
			return "", fmt.Errorf("read writable link: %w", linkErr)
		}

		if !filepath.IsAbs(target) {
			target = filepath.Dir(path) + string(filepath.Separator) + target
		}

		return resolveWritablePath(target, links+1)
	}

	parentPath, leaf := filepath.Split(path)

	parentPath = strings.TrimSuffix(parentPath, string(filepath.Separator))
	if parentPath == "" {
		parentPath = string(filepath.Separator)
	}

	parent, parentErr := resolveWritablePath(parentPath, links)
	if parentErr != nil {
		return "", parentErr
	}

	return filepath.Join(parent, leaf), nil
}
