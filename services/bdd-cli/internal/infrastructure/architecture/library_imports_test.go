package architecture_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/disk"
)

func TestLibraryImportsInspectLinkedServiceSource(t *testing.T) {
	t.Parallel()

	for _, linkedRoot := range []bool{true, false} {
		t.Run(map[bool]string{true: "service root", false: "nested directory"}[linkedRoot], func(t *testing.T) {
			t.Parallel()
			host := t.TempDir()
			source := filepath.Join(host, "implementation")

			err := disk.Write(filepath.Join(source, "calc.go"),
				[]byte("package calc\nimport _ \"example.com/testing/assert\"\n"), disk.Private)
			if err != nil {
				t.Fatal(err)
			}

			service := "linked-calc"

			link := filepath.Join(host, service)
			if !linkedRoot {
				err = disk.Dir(link, disk.Private)
				if err != nil {
					t.Fatal(err)
				}

				link = filepath.Join(link, "linked")
			}

			err = os.Symlink(source, link)
			if err != nil {
				t.Fatal(err)
			}

			arch := architecture.Architecture{Services: []architecture.Service{{Name: service, Path: service, Language: "go"}},
				Testing: architecture.Testing{
					Libraries: []architecture.Library{{Module: "example.com/testing", Version: "v1.0.0"}}}}

			err = arch.CheckLibraryImports(host)
			if !errors.Is(err, architecture.ErrTestOnlyLibrary) {
				t.Fatalf("test-only import through link was not refused: %v", err)
			}
		})
	}
}
