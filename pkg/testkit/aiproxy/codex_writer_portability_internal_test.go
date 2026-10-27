package main

import (
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

func TestCodexWriterCassetteHashesSurviveRelocation(t *testing.T) {
	for _, fixture := range []string{"build-code-fix-by-codex", "build-code-codex-selection", "build-code-codex-nonzero"} {
		t.Run(fixture, func(t *testing.T) {
			dirs, err := filepath.Glob(filepath.Join("../../../tests/true-bdd/fixtures", fixture, "cassettes/codex-*"))
			if err != nil || len(dirs) == 0 {
				t.Fatalf("codex cassettes: %v, %v", dirs, err)
			}

			for _, dir := range dirs {
				assertRelocatedCodexHash(t, dir)
			}
		})
	}
}

func assertRelocatedCodexHash(t *testing.T, dir string) {
	t.Helper()

	manifest, err := readMeta(dir)
	if err != nil {
		t.Fatal(err)
	}

	stdin, err := disk.Read(filepath.Join(dir, stdinFile))
	if err != nil {
		t.Fatal(err)
	}

	for _, installation := range []string{"/machine-a", "/different/machine-b"} {
		t.Setenv("HOME", installation+"/home")
		t.Setenv("CODEX_HOME", installation+"/auth")
		t.Setenv("GOROOT", installation+"/toolchain")
		t.Setenv("GOMODCACHE", installation+"/modules")
		cwd := installation + "/host"

		argv := make([]string, len(manifest.Argv))
		for index, arg := range manifest.Argv {
			argv[index], err = denormalize(arg, cwd, "2026-10-24-00-00-00-1234")
			if err != nil {
				t.Fatal(err)
			}
		}

		prompt, decodeErr := denormalize(string(stdin), cwd, "2026-10-24-00-00-00-1234")
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}

		if hash := requestHash(argv, []byte(prompt), cwd); hash != manifest.RequestHash {
			t.Fatalf("%s: relocated hash %s, recorded %s", dir, hash, manifest.RequestHash)
		}
	}
}
