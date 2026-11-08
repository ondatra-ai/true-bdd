package gotool_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/cli/gotool"
)

func TestPackageDirectoryResolvesTheHostsCachedModule(t *testing.T) {
	host, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("GOFLAGS", "-not-a-go-flag")
	t.Setenv("GOWORK", filepath.Join(t.TempDir(), "missing.work"))

	directory, err := gotool.PackageDirectory(t.Context(), host, "gopkg.in/yaml.v3")
	if err != nil {
		t.Fatal(err)
	}

	if !filepath.IsAbs(directory) || !strings.HasPrefix(filepath.Base(directory), "yaml.v3@") {
		t.Fatalf("expected the host graph's cached yaml.v3, got %s", directory)
	}
}
