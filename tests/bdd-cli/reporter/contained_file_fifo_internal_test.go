//go:build unix

package reporter

import (
	"path/filepath"
	"syscall"
	"testing"
)

// A FIFO inside the fixture is contained by every path rule there is,
// and reading it blocks until a writer appears. ContainedFile is
// therefore asserted DIRECTLY: routing this through ReadContained would
// not fail the test if the guard regressed, it would hang it.
func TestContainedFileRejectsFIFO(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fifo := filepath.Join(dir, "scenarios.yaml")

	err := syscall.Mkfifo(fifo, 0o600)
	if err != nil {
		t.Skipf("mkfifo unavailable: %v", err)
	}

	if path, ok := ContainedFile(dir, "scenarios.yaml"); ok {
		t.Errorf("a FIFO was accepted as readable: %q", path)
	}
}

// The same guard rejects a directory, which is contained and is not a
// file anyone can read to EOF either.
func TestContainedFileRejectsDirectory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	write(t, dir, "true-bdd/checklists/us-apply.yaml", "sections: []\n")

	if _, ok := ContainedFile(dir, "true-bdd/checklists"); ok {
		t.Error("a directory was accepted as readable")
	}
}
