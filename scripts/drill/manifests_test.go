package drill_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/drill"
)

func TestCutADeletesManifestsAfterOfflineRecreationFromLibraries(t *testing.T) {
	root := t.TempDir()
	put(t, filepath.Join(root, "services/true-bdd/go.mod"),
		"module example.com/engine\n\ngo 1.25.0\n\nrequire gopkg.in/yaml.v3 v3.0.1\n")
	put(t, filepath.Join(root, "services/true-bdd/go.sum"), "")
	put(t, filepath.Join(root, "services/true-bdd/main.go"),
		"package main\nimport \"gopkg.in/yaml.v3\"\nfunc main() { _, _ = yaml.Marshal(1) }\n")
	put(t, filepath.Join(root, "docs/architecture/architecture.yaml"), `architecture:
  testing:
    libraries:
      - module: znkr.io/diff
        version: v1.0.1
  services:
    - name: true-bdd
      path: services/true-bdd
      libraries:
        - module: gopkg.in/yaml.v3
          version: v3.0.1
`)
	git(t, root, "init", "-b", "main")
	git(t, root, "add", ".")
	git(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "base")
	t.Setenv("GOFLAGS", "-mod=mod")

	runner := drill.New(root)
	record, err := runner.Start(t.Context(), "HEAD")
	t.Cleanup(func() {
		_ = filepath.WalkDir(record.Directory, func(path string, entry os.DirEntry, err error) error {
			if err == nil && entry.IsDir() {
				_ = os.Chmod(path, 0o700)
			}

			return nil
		})
		_ = os.RemoveAll(record.Directory)
	})

	if err != nil {
		t.Fatal(err)
	}

	if !record.Manifests.Passed || !record.Manifests.Offline || len(record.Manifests.Deleted) != 2 {
		t.Fatalf("manifest evidence: %+v", record.Manifests)
	}

	_, err = os.Stat(filepath.Join(record.Manifests.Cache, "znkr.io/diff@v1.0.1/go.mod"))
	if err != nil {
		t.Fatalf("testing library was not downloaded: %v", err)
	}

	cut, err := runner.Cut(t.Context(), record.Directory, "a")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = os.RemoveAll(cut.Directory) })

	_, err = os.Stat(filepath.Join(cut.Directory, "services/true-bdd"))
	if !os.IsNotExist(err) {
		t.Fatalf("cut A kept engine files: %v", err)
	}
}
