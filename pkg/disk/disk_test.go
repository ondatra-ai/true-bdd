package disk_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

func TestWriteThenReadRoundTrips(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "nested", "payload.json")

	err := disk.Write(path, []byte(`{"a":1}`), disk.Shared)
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	got, err := disk.Read(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if string(got) != `{"a":1}` {
		t.Fatalf("round trip = %q", got)
	}
}

// A hold is taken on the parent directory and a write commits through a
// rename, so neither may leave anything behind for the BDD judge to diff.
func TestWriteLeavesNoResidue(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	err := disk.Write(filepath.Join(dir, "only.txt"), []byte("x"), disk.Shared)
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}

	if len(entries) != 1 || entries[0].Name() != "only.txt" {
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}

		t.Fatalf("directory holds %v, want only.txt alone", names)
	}
}

func TestWritePreservesAnExistingFilesMode(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "kept.txt")

	err := os.WriteFile(path, []byte("first"), 0o640)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	err = disk.Write(path, []byte("second"), disk.Private)
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	if info.Mode().Perm() != 0o640 {
		t.Fatalf("mode = %o, want 640 preserved", info.Mode().Perm())
	}
}

// A killed run can abandon a temp under the deterministic name. The next write
// of that target must clear it rather than fail on O_EXCL.
func TestWriteSelfHealsAnAbandonedTemp(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "target.txt")

	err := os.WriteFile(filepath.Join(dir, ".true-bdd.tmp.target.txt"), []byte("junk"), 0o600)
	if err != nil {
		t.Fatalf("plant: %v", err)
	}

	err = disk.Write(path, []byte("fresh"), disk.Shared)
	if err != nil {
		t.Fatalf("write over abandoned temp: %v", err)
	}

	got, err := disk.Read(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if string(got) != "fresh" {
		t.Fatalf("content = %q", got)
	}
}

const appenders = 24

func TestConcurrentAppendsNeverTearALine(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "events.jsonl")

	var group sync.WaitGroup

	for size := range appenders {
		group.Add(1)

		go func() {
			defer group.Done()

			_ = disk.Append(path, []byte(strings.Repeat("x", size+1)), disk.Private)
		}()
	}

	group.Wait()

	data, err := disk.Read(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(lines) != appenders {
		t.Fatalf("got %d lines, want %d", len(lines), appenders)
	}

	for _, line := range lines {
		if strings.Trim(line, "x") != "" {
			t.Fatalf("torn line %q", line)
		}
	}
}

// Update holds across both halves, so concurrent increments cannot lose one —
// the read-modify-write a separate Read and Write cannot express.
func TestConcurrentUpdatesLoseNothing(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "counter")

	var group sync.WaitGroup

	for range appenders {
		group.Add(1)

		go func() {
			defer group.Done()

			_ = disk.Update(path, disk.Private, func(before []byte) ([]byte, error) {
				return append(before, 'x'), nil
			})
		}()
	}

	group.Wait()

	data, err := disk.Read(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if len(data) != appenders {
		t.Fatalf("counter = %d, want %d — an update was lost", len(data), appenders)
	}
}

// os.Remove reports success when the target is already gone; a missing parent
// directory is the same statement about the world.
func TestRemoveToleratesAMissingDirectory(t *testing.T) {
	t.Parallel()

	err := disk.Remove(filepath.Join(t.TempDir(), "never", "made", "it.txt"))
	if err != nil {
		t.Fatalf("remove under a missing directory: %v", err)
	}
}

// Ensure exists so a log file is present from install; truncating an existing
// one instead would silently discard a run's records.
func TestEnsureCreatesButNeverTruncates(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "run.log.json")

	err := disk.Ensure(path, disk.Shared)
	if err != nil {
		t.Fatalf("ensure on absent: %v", err)
	}

	got, err := disk.Read(path)
	if err != nil || len(got) != 0 {
		t.Fatalf("after ensure: %q, %v", got, err)
	}

	err = disk.Append(path, []byte(`{"a":1}`), disk.Shared)
	if err != nil {
		t.Fatalf("append: %v", err)
	}

	err = disk.Ensure(path, disk.Shared)
	if err != nil {
		t.Fatalf("ensure on present: %v", err)
	}

	got, err = disk.Read(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if string(got) != "{\"a\":1}\n" {
		t.Fatalf("ensure truncated an existing file: %q", got)
	}
}

// CopyTree replaced a `cp -R src/. dst` spawn, so what is pinned here is that
// idiom: src's CONTENTS land at dst's top level, over whatever is there.
func TestCopyTreeLandsTheContentsAndOverwrites(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	src := filepath.Join(root, "src")
	dst := filepath.Join(root, "dst")

	write(t, filepath.Join(src, "server.js"), "fresh")
	write(t, filepath.Join(src, "nested", "chunk.js"), "deep")
	write(t, filepath.Join(dst, "server.js"), "stale")

	err := disk.CopyTree(dst, src, disk.Shared)
	if err != nil {
		t.Fatalf("copy tree: %v", err)
	}

	if got := read(t, filepath.Join(dst, "server.js")); got != "fresh" {
		t.Errorf("top-level file = %q, want the source's contents", got)
	}

	if got := read(t, filepath.Join(dst, "nested", "chunk.js")); got != "deep" {
		t.Errorf("nested file = %q, want the source's contents", got)
	}
}

// A symlink refuses rather than being flattened into a copy of its target: the
// Next.js bundle this copies holds none today, and a silent flattening is the
// wrong way to find out that changed.
func TestCopyTreeRefusesASymlink(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	src := filepath.Join(root, "src")

	write(t, filepath.Join(src, "real.js"), "x")

	err := os.Symlink(filepath.Join(src, "real.js"), filepath.Join(src, "link.js"))
	if err != nil {
		t.Fatalf("plant a symlink: %v", err)
	}

	err = disk.CopyTree(filepath.Join(root, "dst"), src, disk.Shared)
	if !errors.Is(err, disk.ErrSymlink) {
		t.Fatalf("copy tree = %v, want ErrSymlink", err)
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()

	err := disk.Write(path, []byte(content), disk.Shared)
	if err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()

	data, err := disk.Read(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	return string(data)
}
