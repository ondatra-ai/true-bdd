package runner

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// TestPrepareRunDirDropsAbsentPaths: a path the scenario says the tree
// has no copy of is gone from the assembled run dir, even one the repo
// layer brings, while the rest of that layer stays.
func TestPrepareRunDirDropsAbsentPaths(t *testing.T) {
	t.Parallel()

	fixture, err := LoadFixture(writeTestFixture(t, "", map[string]string{"docs/scenarios.yaml": "scenarios: {}\n"}))
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}

	fixture.Absent = []string{"true-bdd/true-bdd.yaml", "docs/scenarios.yaml"}

	tmpDir, err := prepareRunDir(fixture, t.TempDir())
	if err != nil {
		t.Fatalf("prepare run dir: %v", err)
	}

	for _, rel := range fixture.Absent {
		_, err = os.Lstat(filepath.Join(tmpDir, rel))
		if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("%s must be absent from the run dir, stat error = %v", rel, err)
		}
	}

	_, err = os.Stat(filepath.Join(tmpDir, "true-bdd", "checklists", "us-refine.yaml"))
	if err != nil {
		t.Errorf("the rest of the repo layer must stay: %v", err)
	}
}

// TestPrepareRunDirRefusesAbsentPathsItCannotDrop: a path the assembled
// tree does not hold proves nothing by its absence, and one leading out
// of the run dir is never deleted; both fail the assembly.
func TestPrepareRunDirRefusesAbsentPathsItCannotDrop(t *testing.T) {
	t.Parallel()

	cases := map[string]error{
		"true-bdd/no-such-file.yaml": ErrAbsentPathNotHeld,
		"../outside.yaml":            ErrAbsentPathUnsafe,
		"/etc/hosts":                 ErrAbsentPathUnsafe,
		"":                           ErrAbsentPathUnsafe,
	}

	for path, want := range cases {
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			fixture, err := LoadFixture(writeTestFixture(t, "", nil))
			if err != nil {
				t.Fatalf("load fixture: %v", err)
			}

			fixture.Absent = []string{path}

			_, err = prepareRunDir(fixture, t.TempDir())
			if !errors.Is(err, want) {
				t.Fatalf("prepare run dir with absent %q: error = %v, want %v", path, err, want)
			}
		})
	}
}
