package drill_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/drill"
)

func TestOracleUsesOriginalSuiteAndCountsOnlyRegistryScenarios(t *testing.T) {
	runner, record, cut := prepare(t, "#!/bin/sh\nexit 0\n")
	put(t, filepath.Join(runner.Root, "docs/scenarios.yaml"), "scenarios:\n  sample-pass: {}\n  sample-fail: {}\n")
	git(t, runner.Root, "add", ".")
	git(t, runner.Root, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "registry")

	next, err := runner.Start(t.Context(), "HEAD")
	must(t, err)

	t.Cleanup(func() { _ = os.RemoveAll(next.Directory) })

	nextCut, err := runner.Cut(t.Context(), next.Directory, "a")
	must(t, err)

	t.Cleanup(func() { _ = os.RemoveAll(nextCut.Directory) })

	_ = record
	_ = cut
	fakeGo := filepath.Join(t.TempDir(), "go")
	put(t, fakeGo, `#!/bin/sh
printf '%s\n' "$@" >> "$DRILL_CAPTURE"
if [ "$1" = build ]; then
 while [ $# -gt 0 ]; do
  if [ "$1" = -o ]; then shift; cp "$DRILL_FAKE_ENGINE" "$1"; exit; fi
  shift
 done
fi
[ -f tests/true-bdd/generated_test.go ] || exit 11
[ ! -d services/true-bdd ] || exit 12
cat <<'JSON'
{"Action":"pass","Test":"TestSamplePass"}
{"Action":"pass","Test":"TestScenarioCoverage"}
{"Action":"output","Test":"TestSampleFail","Output":"first failure details\n"}
{"Action":"fail","Test":"TestSampleFail"}
JSON
exit 1
`)
	t.Setenv("PATH", filepath.Dir(fakeGo)+string(os.PathListSeparator)+os.Getenv("PATH"))

	put(t, filepath.Join(nextCut.Directory, "services/true-bdd/main.go"), "package main\nfunc main() {}\n")

	run, err := runner.PrepareOracle(t.Context(), next.Directory, "a")
	must(t, err)

	must(t, runner.Supervise(t.Context(), next.Directory, run.Tag))

	final, err := drill.Read(next.Directory)
	must(t, err)

	oracle := final.Runs[run.Tag].Oracle
	if oracle == nil || oracle.Passed != 1 || oracle.Total != 2 || len(oracle.Failures) != 1 {
		t.Fatalf("oracle: %+v", oracle)
	}

	capture, err := os.ReadFile(os.Getenv("DRILL_CAPTURE"))
	must(t, err)

	for _, flag := range []string{"-json", "-timeout=40m", "-mode=services:replay,tests:replay", "-engine-bin="} {
		if !strings.Contains(string(capture), flag) {
			t.Errorf("missing %s", flag)
		}
	}
}
