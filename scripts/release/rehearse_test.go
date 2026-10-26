package release_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/release"
)

func TestMissingArchiveIsAnInstallFailure(t *testing.T) {
	t.Parallel()
	artifacts := t.TempDir()
	write(t, filepath.Join(artifacts, "release.json"), `{"version":"test","commit":"HEAD"}`)
	write(t, filepath.Join(artifacts, "checksums.txt"), "")

	_, err := release.Rehearse(context.Background(), t.TempDir(), artifacts, release.Options{})
	if err == nil || !strings.Contains(err.Error(), "install") || !strings.Contains(err.Error(), ".tar.gz") {
		t.Fatalf("missing archive should be refused at install: %v", err)
	}
}

func write(t *testing.T, path, data string) {
	t.Helper()

	err := os.MkdirAll(filepath.Dir(path), 0o700)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(path, []byte(data), 0o600)
	if err != nil {
		t.Fatal(err)
	}
}

func TestRehearsalInstallsAHostBeforeItsFirstAITurn(t *testing.T) {
	root := sourceRepository(t)
	artifacts := artifactSet(t, installedEngine)

	report, err := release.Rehearse(context.Background(), root, artifacts, release.Options{})
	if report.Root != "" {
		t.Cleanup(func() { _ = os.RemoveAll(report.Root) })
	}

	if err != nil {
		t.Fatalf("rehearsal: %v", err)
	}

	if !report.Passed || len(report.Hosts) != 2 || !report.Hosts[0].Passed {
		t.Fatalf("report: %+v", report)
	}

	assertInstallSteps(t, report)

	if report.Hosts[1].Skipped == "" {
		t.Error("missing node/npm must produce an explicit Jest skip")
	}
}

const installedEngine = `#!/bin/sh
case "$1" in
 version) exit 0 ;;
 init)
  test ! -e true-bdd/true-bdd.yaml || exit 11
  test ! -e templates/poison || exit 12
  /bin/mkdir -p true-bdd/checklists templates
  echo init-created > true-bdd/true-bdd.yaml
  echo init-created > true-bdd/checklists/build-code.yaml
  echo init-created > templates/build-code.prompt.tpl
  exit 0 ;;
esac
/bin/mkdir -p tmp
if /usr/bin/grep -q 'example.invalid/true-bdd-missing' docs/architecture/architecture.yaml; then
 printf '{"msg":"Refusing to start","error":"%s"}\n' \
  'library download failed (example.invalid/true-bdd-missing@v0.0.0)' > tmp/true-bdd.log.json
 echo 'library download failed (example.invalid/true-bdd-missing@v0.0.0)'
 exit 1
fi
test ! -e "$GOMODCACHE" || exit 18
test -d .git || exit 13
/usr/bin/grep -q init-created true-bdd/true-bdd.yaml || exit 14
if /usr/bin/grep -q replace go.mod; then exit 15; fi
/usr/bin/grep -q 'github.com/ondatra-ai/true-bdd v0.0.0' go.mod || test -f proxy-receipt || exit 17
test ! -e services/calc || exit 16
/bin/mkdir -p tmp
pin=$(/usr/bin/awk '$1 == "github.com/ondatra-ai/true-bdd" {print $2}' go.mod)
printf '{"msg":"Libraries downloaded","libraries":["%s"],"read_roots":["%s"]}\n' \
 "github.com/ondatra-ai/true-bdd@$pin" "$GOMODCACHE/github.com/ondatra-ai/true-bdd@$pin" > tmp/true-bdd.log.json
echo '{"msg":"Dispatching AI turn","cli":"claude","role":"fix_generator"}' >> tmp/true-bdd.log.json
claude
exit 1
`

func assertInstallSteps(t *testing.T, report release.Report) {
	t.Helper()

	for _, step := range []string{"install checksums", "binary", "source", "sample host",
		"configuration", "harness", "git init", aiBoundary} {
		found := false

		for _, result := range report.Steps {
			if result.Name == step && result.Result == "passed" {
				found = true
			}
		}

		if !found {
			t.Errorf("missing successful install step %q: %+v", step, report.Steps)
		}
	}
}
