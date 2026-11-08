package testkit_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/testkit"
)

func TestContainedFileChecksRegularFileContainmentWithoutReading(t *testing.T) {
	t.Parallel()
	base := t.TempDir()

	file := filepath.Join(base, "unreadable")

	err := testkit.Write(file, []byte("private"), testkit.Private)
	if err != nil {
		t.Fatal(err)
	}

	err = os.Chmod(file, 0)
	if err != nil {
		t.Fatal(err)
	}

	canonical, err := filepath.EvalSymlinks(file)
	if err != nil {
		t.Fatal(err)
	}

	if got, ok := testkit.ContainedFile(base, "nested/../unreadable"); !ok || got != canonical {
		t.Fatalf("contained regular file = %q, %t; want %q despite unreadable bytes", got, ok, canonical)
	}

	outside := filepath.Join(t.TempDir(), "secret")

	err = testkit.Write(outside, []byte("outside"), testkit.Private)
	if err != nil {
		t.Fatal(err)
	}

	err = os.Symlink(outside, filepath.Join(base, "outside"))
	if err != nil {
		t.Fatal(err)
	}

	for _, relative := range []string{".", "missing", "../escape", file, "outside"} {
		if got, ok := testkit.ContainedFile(base, relative); ok {
			t.Errorf("accepted %q as %q", relative, got)
		}
	}
}
