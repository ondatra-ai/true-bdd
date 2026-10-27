package drill

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

const (
	goModFile = "go.mod"
	goSumFile = "go.sum"
)

func kept(path string) bool {
	for _, root := range []string{"docs/product/", "true-bdd/", "templates/", "pkg/", "scripts/", "tests/true-bdd/"} {
		if strings.HasPrefix(path, root) {
			return true
		}
	}

	for _, file := range []string{"docs/architecture/architecture.yaml",
		"docs/scenarios.yaml",
		goModFile,
		goSumFile,
		".gitignore"} {
		if path == file {
			return true
		}
	}

	return false
}

func removed(cut, path string, data []byte) bool {
	if strings.HasPrefix(path, "services/true-bdd/") {
		return true
	}

	if cut == "a" {
		return false
	}

	if strings.HasPrefix(path, "tests/true-bdd/steps/") {
		return true
	}

	if filepath.Dir(path) == "tests/true-bdd" && strings.HasSuffix(path,
		"_test.go") && regexp.MustCompile(`(?m)^// Code generated[^\n]*`).Match(data) {
		return true
	}

	if cut == "b" {
		return false
	}

	if strings.HasPrefix(path, "tests/true-bdd/fixtures/") {
		return cut == "d" || strings.Contains(path, "/cassettes/")
	}

	return false
}

func exportTree(directory string, archive []byte, cut string) error {
	reader := tar.NewReader(bytes.NewReader(archive))
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}

		if err != nil {
			return fmt.Errorf("reading archive: %w", err)
		}

		err = exportEntry(directory, cut, header, reader)
		if err != nil {
			return err
		}
	}
}

func exportEntry(directory, cut string, header *tar.Header, reader io.Reader) error {
	if header.Typeflag == tar.TypeDir || header.Typeflag == tar.TypeXGlobalHeader {
		return nil
	}

	if cut != "" && !kept(header.Name) {
		return nil
	}

	path := filepath.Join(directory, header.Name)
	if !within(path, directory) {
		return fmt.Errorf("%w: archive path escapes tree", errRefused)
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("reading archive entry: %w", err)
	}

	if cut != "" && removed(cut, header.Name, data) {
		return nil
	}

	return writeEntry(path, cut, header, data)
}

func writeEntry(path, cut string, header *tar.Header, data []byte) error {
	if header.Typeflag == tar.TypeSymlink && cut == "" {
		return nil
	}

	if header.Typeflag != tar.TypeReg {
		return fmt.Errorf("%w: unsupported archive entry %s", errRefused, header.Name)
	}

	err := disk.Write(path, data, disk.Private)
	if err != nil {
		return err
	}

	if header.Mode&0o100 != 0 {
		return disk.Executable(path)
	}

	return nil
}
