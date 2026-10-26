package steps

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

const insideSymlink = "inside symlink"
const regularFile = "regular file"

func TestEveryFileMatchIsContainedBeforeReading(t *testing.T) {
	t.Parallel()

	for _, kind := range []string{"escaping symlink", "directory", regularFile, insideSymlink} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()

			plantPromptMatch(t, kind, root)

			err := assertEveryFileMatches(runAt(root), []string{"*.txt", filePatternMatches, "secret"})
			if kind == regularFile || kind == insideSymlink {
				if err != nil {
					t.Fatal(err)
				}
			} else if !errors.Is(err, ErrPathEscapesRun) {
				t.Fatalf("matched %s was not refused before reading: %v", kind, err)
			}
		})
	}
}

func plantPromptMatch(t *testing.T, kind, root string) {
	t.Helper()
	outside := filepath.Join(t.TempDir(), "secret")

	err := os.WriteFile(outside, []byte("secret"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	file := filepath.Join(root, "prompt.txt")

	switch kind {
	case "escaping symlink":
		err = os.Symlink(outside, file)
	case "directory":
		err = os.Mkdir(file, 0o700)
	case regularFile:
		err = os.WriteFile(file, []byte("secret"), 0o600)
	case insideSymlink:
		inside := filepath.Join(root, "real")

		err = os.WriteFile(inside, []byte("secret"), 0o600)
		if err == nil {
			err = os.Symlink(inside, file)
		}
	}

	if err != nil {
		t.Fatal(err)
	}
}
