package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/cli"
	"github.com/ondatra-ai/true-bdd/pkg/cli/spec"
	proxy "github.com/ondatra-ai/true-bdd/pkg/proxy/cli"
)

func TestSubstituteBuildsFromLocalLibraryWithoutChangingHost(t *testing.T) {
	library, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}

	host := t.TempDir()

	manifest := "module example.com/host\n\ngo 1.25.0\n\n" +
		"require github.com/ondatra-ai/true-bdd/pkg v0.0.0\n" +
		"replace github.com/ondatra-ai/true-bdd/pkg => " + library + "\n"

	err = os.WriteFile(filepath.Join(host, "go.mod"), []byte(manifest), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	utility, err := proxy.Build(host, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	shims := filepath.Join(t.TempDir(), "shim")

	err = utility.Substitute(shims, []proxy.Dependency{{Name: "example-cli"}})
	if err != nil {
		t.Fatal(err)
	}

	cassettes, state := t.TempDir(), t.TempDir()

	result, err := spec.Run([]string{filepath.Join(shims, "example-cli")}, cli.Options{
		Dir: host, Timeout: time.Minute,
		Env: cli.Inherit().Set(proxy.Environment("replay", shims, cassettes, state, nil)...),
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.Code != 86 || !strings.Contains(result.Stderr, "no cassette") {
		t.Fatalf("stand-in: code %d: %s", result.Code, result.Stderr)
	}

	after, err := os.ReadFile(filepath.Join(host, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}

	if string(after) != manifest {
		t.Fatal("substitution changed the host module")
	}
}
