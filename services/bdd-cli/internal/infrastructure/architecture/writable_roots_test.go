package architecture_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/disk"
)

const writableService = "calculator"
const protectedTests = "tests"
const writableFolder = "services/calc"

func TestWritableRootsDeclareFoldersAndExactFiles(t *testing.T) {
	root := t.TempDir()

	err := disk.Dir(filepath.Join(root, "config"), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	service := architecture.Service{
		Name: writableService, Path: writableFolder, Writable: []string{"go.mod", "go.sum", "config"},
	}

	roots, err := architecture.WritableRoots(root, []architecture.Service{service}, []string{"tests/calc"})
	if err != nil {
		t.Fatal(err)
	}

	want := []architecture.WritableRoot{
		{Path: writableFolder, Directory: true}, {Path: "go.mod"}, {Path: "go.sum"},
		{Path: "config", Directory: true},
	}
	if !reflect.DeepEqual(roots, want) {
		t.Fatalf("roots = %#v; want %#v", roots, want)
	}
}

func TestWritableRootsRefusePatternsEscapesAndTests(t *testing.T) {
	entries := []string{
		"*.mod", "configs/**", "go.?um", "[ab]", "../outside", "/outside", ".", "",
		protectedTests, "tests/calc/a_test.go", "spec-tests", "spec-tests/steps",
	}
	for _, entry := range entries {
		t.Run(entry, func(t *testing.T) {
			service := architecture.Service{Name: writableService, Path: writableFolder, Writable: []string{entry}}

			_, err := architecture.WritableRoots(t.TempDir(), []architecture.Service{service}, []string{"spec-tests"})
			if err == nil {
				t.Fatalf("accepted forbidden writable %q", entry)
			}
		})
	}
}

func TestWritableRootsRefuseServiceCoveringTests(t *testing.T) {
	for _, path := range []string{".", protectedTests, "spec", "spec/tests/steps"} {
		t.Run(path, func(t *testing.T) {
			services := []architecture.Service{{Name: writableService, Path: path}}

			_, err := architecture.WritableRoots(t.TempDir(), services, []string{"spec/tests"})
			if err == nil {
				t.Fatalf("accepted service over tests: %q", path)
			}
		})
	}
}

func TestWritableRootsResolveSymlinks(t *testing.T) {
	for _, target := range []string{"outside", "missing", protectedTests} {
		t.Run(target, func(t *testing.T) {
			host, outside := t.TempDir(), t.TempDir()

			destination := outside
			if target == "missing" {
				destination = filepath.Join(outside, "not-created")
			}

			if target == protectedTests {
				destination = filepath.Join(host, "tests/calc")
			}

			err := os.Symlink(destination, filepath.Join(host, "link"))
			if err != nil {
				t.Fatal(err)
			}

			service := architecture.Service{Name: writableService, Path: writableFolder, Writable: []string{"link"}}

			_, err = architecture.WritableRoots(host, []architecture.Service{service}, nil)
			if err == nil {
				t.Fatalf("accepted symlink to %q", destination)
			}
		})
	}
}

func TestWritableRootsRefuseEscapingOrTestOverlappingScratch(t *testing.T) {
	for _, target := range []string{"outside", protectedTests} {
		t.Run(target, func(t *testing.T) {
			host, outside := t.TempDir(), t.TempDir()

			destination := outside
			if target == protectedTests {
				destination = filepath.Join(host, "tests")
			}

			err := os.Symlink(destination, filepath.Join(host, "tmp"))
			if err != nil {
				t.Fatal(err)
			}

			_, err = architecture.WritableRoots(host, []architecture.Service{{Name: writableService, Path: writableFolder}}, nil)
			if err == nil {
				t.Fatal("accepted unsafe host tmp")
			}
		})
	}
}

func TestWritableRootsResolveLinkBeforeParentTraversal(t *testing.T) {
	host, outside := t.TempDir(), t.TempDir()

	err := disk.Dir(filepath.Join(outside, "folder"), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	err = os.Symlink(filepath.Join(outside, "folder"), filepath.Join(host, "pivot"))
	if err != nil {
		t.Fatal(err)
	}

	err = os.Symlink("pivot/../missing", filepath.Join(host, "link"))
	if err != nil {
		t.Fatal(err)
	}

	service := architecture.Service{Name: writableService, Path: writableFolder, Writable: []string{"link"}}

	_, err = architecture.WritableRoots(host, []architecture.Service{service}, nil)
	if err == nil {
		t.Fatal("accepted a link whose parent traversal resolves outside the host")
	}
}
