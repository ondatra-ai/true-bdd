package ai

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCodexWriteRootsResolveDanglingSymlinks(t *testing.T) {
	t.Parallel()

	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	outside := t.TempDir()

	const (
		escape = "escape"
		inside = "inside"
		cycle  = "cycle"
	)

	for _, link := range []struct{ name, target string }{
		{escape, filepath.Join(outside, "new.go")},
		{"chain", escape},
		{inside, "new.go"},
		{"inside-chain", inside},
		{cycle, cycle},
	} {
		err := os.Symlink(link.target, filepath.Join(root, link.name))
		if err != nil {
			t.Fatal(err)
		}
	}

	for _, testCase := range []struct {
		name  string
		allow bool
	}{
		{escape, false}, {"chain", false}, {inside, true}, {"inside-chain", true}, {cycle, false},
	} {
		resolved, err := codexInsideRoot(root, filepath.Join(root, testCase.name))
		if (err == nil) != testCase.allow {
			t.Errorf("%s: path=%q error=%v, want allowed=%v", testCase.name, resolved, err, testCase.allow)
		}

		if testCase.allow && resolved != filepath.Join(root, "new.go") {
			t.Errorf("%s: resolved to %q", testCase.name, resolved)
		}
	}
}
