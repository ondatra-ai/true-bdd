package fstree

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A snapshot must describe the tree it was pointed at: os.ReadFile follows
// symlinks, and a hard-linked entry looks like a regular file to
// fs.DirEntry, so both could silently pull in content from outside it.
func TestSnapshotRejectsSymlink(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret.txt")

	err := os.WriteFile(outside, []byte("not yours"), 0o600)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	err = os.Symlink(outside, filepath.Join(root, "link.txt"))
	if err != nil {
		t.Fatalf("symlink: %v", err)
	}

	_, err = Snapshot(root)
	if !errors.Is(err, ErrSymlinkInTree) {
		t.Fatalf("err = %v, want ErrSymlinkInTree", err)
	}
}

func TestSnapshotRejectsHardLink(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret.txt")

	err := os.WriteFile(outside, []byte("not yours"), 0o600)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	err = os.Link(outside, filepath.Join(root, "linked.txt"))
	if err != nil {
		t.Skipf("hard links unsupported here: %v", err)
	}

	_, err = Snapshot(root)
	if !errors.Is(err, ErrHardLinkInTree) {
		t.Fatalf("err = %v, want ErrHardLinkInTree", err)
	}
}

func TestSnapshotReadsOrdinaryFiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("hello"), 0o600)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	tree, err := Snapshot(root)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	if string(tree["a.txt"]) != "hello" {
		t.Fatalf("tree = %v, want a.txt=hello", tree)
	}
}

// A bare skip name must exclude that directory wherever it sits: a
// fixture's prep installs to tests/node_modules, and a root-anchored
// rule would walk into it and meet the package manager's symlinks.
func TestSnapshotSkipsNestedDirectoriesByName(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	nested := filepath.Join(root, "tests", "node_modules", ".bin")

	err := os.MkdirAll(nested, 0o755)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// The kind of entry that made this necessary.
	err = os.Symlink("/usr/bin/true", filepath.Join(nested, "playwright"))
	if err != nil {
		t.Skipf("symlinks unsupported here: %v", err)
	}

	err = os.WriteFile(filepath.Join(root, "kept.txt"), []byte("x"), 0o600)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	tree, err := Snapshot(root, "node_modules")
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	if _, present := tree["kept.txt"]; !present {
		t.Fatal("the ordinary file was dropped")
	}

	for path := range tree {
		if strings.Contains(path, "node_modules") {
			t.Fatalf("%s was walked despite the skip", path)
		}
	}
}
