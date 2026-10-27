package gates_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/gates"
)

func TestEngineImportsRejectsRepositoryCodeOutsideEngine(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name, source string
		bad          bool
	}{
		{"shared-channel", `package engine; import "github.com/ondatra-ai/true-bdd/pkg/disk"`, true},
		{"tooling", `package engine; import _ "github.com/ondatra-ai/true-bdd/scripts/drill"`, true},
		{"other-service", `package engine; import alias "github.com/ondatra-ai/true-bdd/services/other"`, true},
		{"root", "package engine; import `github.com/ondatra-ai/true-bdd`", true},
		{"prefix-collision", `package engine; import "github.com/ondatra-ai/true-bdd/services/bdd-cli-x"`, true},
		{"own-package", `package engine; import "github.com/ondatra-ai/true-bdd/services/bdd-cli/api"`, false},
		{"third-party", `package engine; import "github.com/spf13/cobra"`, false},
		{"other-repo", `package engine; import "github.com/ondatra-ai/true-bdd-extra/pkg"`, false},
		{"malformed", `package engine; import "unterminated`, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()

			dir := filepath.Join(root, "services/bdd-cli/nested")

			err := os.MkdirAll(dir, 0o700)
			if err != nil {
				t.Fatal(err)
			}

			path := filepath.Join(dir, "hidden_windows_test.go")

			err = os.WriteFile(path, []byte("//go:build imaginary\n\n"+test.source), 0o600)
			if err != nil {
				t.Fatal(err)
			}

			err = gates.CheckEngineImports(root)
			if (err != nil) != test.bad {
				t.Fatalf("bad=%v, got %v", test.bad, err)
			}

			if test.bad && !strings.Contains(err.Error(), "hidden_windows_test.go") {
				t.Fatalf("missing source diagnosis: %v", err)
			}
		})
	}
}

func TestEngineImportsRequiresEngineSource(t *testing.T) {
	t.Parallel()

	err := gates.CheckEngineImports(t.TempDir())
	if err == nil {
		t.Fatal("a missing rebuilt engine passed")
	}
}
