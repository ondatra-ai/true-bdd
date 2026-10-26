package release_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/release"
)

func TestRehearsalUsesOneCachedHarnessAndChecksMissingLibraries(t *testing.T) {
	root := sourceRepository(t)
	report, err := release.Rehearse(context.Background(), root, artifactSet(t, installedEngine), release.Options{})
	t.Cleanup(func() { _ = os.RemoveAll(report.Root) })

	if err != nil {
		t.Fatal(err)
	}

	assertSameHarness(t, report)

	if len(report.Hosts[0].LibraryDownloads) == 0 {
		t.Fatal("engine download before first AI turn is missing")
	}

	if report.LibraryFailure == nil || !report.LibraryFailure.Passed || len(report.LibraryFailure.Turns) != 0 {
		t.Fatalf("missing library did not stop before AI: %+v", report.LibraryFailure)
	}
}

func assertSameHarness(t *testing.T, report release.Report) {
	t.Helper()

	manifest, err := os.ReadFile(filepath.Join(report.Root, "calculator/go.mod"))
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(string(manifest), "replace") {
		t.Fatalf("harness must come from cache: %s", manifest)
	}

	spec, err := os.ReadFile(filepath.Join(report.Root, "calculator/docs/architecture/architecture.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(spec), report.HarnessVersion) ||
		!strings.Contains(string(manifest), report.HarnessVersion) {
		t.Fatalf("installed and declared harness differ: %s\n%s", manifest, spec)
	}
}

func TestRehearsalRequiresTheEngineDownloadBeforeAI(t *testing.T) {
	for _, late := range []bool{false, true} {
		t.Run(map[bool]string{false: "no download", true: "download after AI"}[late], func(t *testing.T) {
			root := sourceRepository(t)

			engine := strings.ReplaceAll(installedEngine, "Libraries downloaded", "Ignored libraries")
			if late {
				engine = strings.Replace(engine, "\nclaude\n", "\nclaude\n"+
					"echo '{\"msg\":\"Libraries downloaded\"}' >> tmp/true-bdd.log.json\n", 1)
			}

			report, err := release.Rehearse(t.Context(), root, artifactSet(t, engine), release.Options{})
			t.Cleanup(func() { _ = os.RemoveAll(report.Root) })

			if err == nil || report.Passed {
				t.Fatalf("missing/late download accepted: %+v", report)
			}

			if !strings.Contains(err.Error(), "before first AI turn") {
				t.Fatal(err)
			}
		})
	}
}

func TestRehearsalRequiresTheMissingLibraryName(t *testing.T) {
	root := sourceRepository(t)
	engine := strings.Replace(installedEngine,
		"echo 'library download failed (example.invalid/true-bdd-missing@v0.0.0)'",
		"echo 'library download failed'", 1)
	report, err := release.Rehearse(t.Context(), root, artifactSet(t, engine), release.Options{})
	t.Cleanup(func() { _ = os.RemoveAll(report.Root) })

	if err == nil || report.Passed {
		t.Fatalf("unnamed library refusal accepted: %+v", report)
	}
}
