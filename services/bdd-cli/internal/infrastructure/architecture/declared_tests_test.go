package architecture_test

import (
	"errors"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/disk"
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/architecture"
)

const testConfigFile = "test-config/framework.yaml"

func TestDeclaredTestDirectoriesAreProtectedWithoutRegistry(t *testing.T) {
	t.Parallel()

	for _, entry := range []string{"acceptance", "integration", "live-tests", "test-config", testConfigFile} {
		t.Run(entry, func(t *testing.T) {
			t.Parallel()

			arch := architecture.Architecture{
				Services: []architecture.Service{{Name: writableService, Path: writableFolder, Writable: []string{entry}}},
				Testing: architecture.Testing{Framework: "go-test",
					Commands: architecture.SuiteCommands{Record: `go test "./acceptance/..."`,
						Replay: "go test ./integration/...",
						Live:   "go test ./live-tests/..."}},
			}
			if entry == "test-config" || entry == testConfigFile {
				arch.Testing.ConfigFile = testConfigFile
			}

			_, err := arch.CodeWritableRoots(t.TempDir(), nil)
			if !errors.Is(err, architecture.ErrInvalidWritableRoot) {
				t.Fatalf("test directory grant %q accepted: %v", entry, err)
			}
		})
	}
}

func TestServiceTestsStayWritableAndOutsideTestDirectoriesAreIgnored(t *testing.T) {
	t.Parallel()

	for _, config := range []string{"", "../external/framework.yaml"} {
		t.Run(config, func(t *testing.T) {
			t.Parallel()
			host := t.TempDir()

			err := disk.Write(filepath.Join(host, writableFolder, "calc_test.go"), []byte("package calc\n"), disk.Private)
			if err != nil {
				t.Fatal(err)
			}

			arch := architecture.Architecture{
				Services: []architecture.Service{{Name: writableService, Path: writableFolder}},
				Testing: architecture.Testing{Framework: "go-test", ConfigFile: config,
					Commands: architecture.SuiteCommands{Record: "go test ./...", Replay: "go test ./services/calc/...",
						Live: "go test ../external/..."}},
			}

			_, err = arch.CodeWritableRoots(host, nil)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
