package release_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/release"
)

func TestProxyHarnessUsesTheNamedRevisionWithoutLocalReplacement(t *testing.T) {
	root := sourceRepository(t)
	tool := filepath.Join(os.Getenv("PATH"), "go")

	err := os.Remove(tool)
	if err != nil {
		t.Fatal(err)
	}

	write(t, tool, `#!/bin/sh
if test "$1" = mod; then exit 0; fi
if test "$1" = list; then echo github.com/ondatra-ai/true-bdd/pkg v0.0.0-published; exit 0; fi
if test "$1" = install; then
 test "$2" = github.com/ondatra-ai/true-bdd/services/true-bdd@published-commit || exit 13
 /bin/mkdir -p "$GOBIN"
 printf '#!/bin/sh\necho "true-bdd version v0.0.0-published commit unknown"\n' > "$GOBIN/true-bdd"
 /bin/chmod 700 "$GOBIN/true-bdd"
 exit 0
fi
test "$1" = get || exit 11
test "$2" = github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo@published-commit || exit 12
echo 'require github.com/ondatra-ai/true-bdd/pkg v0.0.0-published' >> go.mod
echo proxy-installed > proxy-receipt
`)

	err = os.Chmod(tool, 0o700)
	if err != nil {
		t.Fatal(err)
	}

	engine := installedEngine
	artifacts := artifactSet(t, engine)
	report, err := release.Rehearse(context.Background(), root, artifacts,
		release.Options{HarnessRevision: "published-commit"})

	t.Cleanup(func() { _ = os.RemoveAll(report.Root) })

	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(report.Harness, "module proxy at published-commit") {
		t.Fatalf("wrong harness report: %+v", report)
	}

	data, err := os.ReadFile(filepath.Join(report.Root, "calculator/go.mod"))
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(string(data), "replace") {
		t.Fatalf("proxy install uses local source: %s", data)
	}
}
