package gotool_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/cli/gotool"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
)

func TestPackageDocumentationUsesTheHostReplacementAndIncludesMethodMeanings(t *testing.T) {
	host := t.TempDir()
	for path, content := range map[string]string{
		"go.mod": "module example.org/host\n\ngo 1.25.0\nrequire example.org/runtime v1.0.0\n" +
            "replace example.org/runtime => ./runtime\n",
		"runtime/go.mod": "module example.org/runtime\n\ngo 1.25.0\n",
		"runtime/api.go": "// Package runtime supplies fixture trees.\npackage runtime\n" +
            "// Run is one Scenario's state.\ntype Run struct{}\n" +
            "// PrepareTree assembles the named fixture before its command runs.\n" +
            "func (*Run) PrepareTree(name string) error { panic(\"hidden implementation\") }\n",
	} {
		err := disk.Write(filepath.Join(host, path), []byte(content), disk.Private)
		if err != nil {
			t.Fatal(err)
		}
	}

	t.Setenv("GOFLAGS", "-not-a-go-flag")
	t.Setenv("GOWORK", filepath.Join(host, "missing.work"))
	t.Setenv("GOTOOLCHAIN", "missing-toolchain")

	documentation, err := gotool.PackageDocumentation(t.Context(), host, "example.org/runtime")
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"PrepareTree(name string) error",
        "PrepareTree assembles the named fixture before its command runs."} {
		if !strings.Contains(documentation, want) {
			t.Errorf("missing %q in %s", want, documentation)
		}
	}

	if strings.Contains(documentation, "hidden implementation") {
		t.Fatal("documentation embeds implementation")
	}

	_, err = gotool.PackageDocumentation(t.Context(), host, "example.org/missing")
	if err == nil || !strings.Contains(err.Error(), "example.org/missing") {
		t.Fatalf("missing package error: %v", err)
	}
}
