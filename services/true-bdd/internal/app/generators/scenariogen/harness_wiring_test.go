package scenariogen_test

import (
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/scenariogen"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"path/filepath"
	"strings"
	"testing"
)

func TestMainFileOwnsHarnessAndNew(t *testing.T) {
	t.Parallel()
	plan, root := wiringPlan(t)
	plan.RecordExclude = []string{"cache/**", "**/scratch/**"}
	plan.Testkit = &architecture.Testkit{RepoLayer: []string{"resources"}, FixtureStep: `fixture "([^"]+)"`}
	plan.Services = []architecture.Service{{Name: wiringService, Dependencies: []architecture.Dependency{
		{Name: "vendor", Protocol: "cli", Scope: "third_party", Recording: architecture.Recording{
			Placeholders: []architecture.Placeholder{{Token: "{{JOB}}", Pattern: `job-\d+`, Directory: "work"}},
			Unrecorded:   []string{"volatile/**"}, Unread: []string{"private/**"},
			ReadOnlyWhen: map[string]string{"REQUEST_ROLE": "read"},
		}},
		{Name: "owned", Protocol: "cli", Scope: "self_hosted"},
		{Name: "remote", Protocol: "http", Scope: "third_party"},
	}}}

	err := scenariogen.AddMainFiles(plan, root, "docs/scenarios.yaml", "docs/architecture/architecture.yaml")
	if err != nil {
		t.Fatal(err)
	}

	_, err = scenariogen.Write(plan, root)
	if err != nil {
		t.Fatal(err)
	}

	source, err := disk.Read(filepath.Join(root, "checks/arithmetic/main_test.go"))
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"package arithmetic_test", `testkit.NewSuite("calc", []proxycli.Dependency{`,
		`Name: "vendor"`, `Token: "{{JOB}}"`,
		`Directory: "work"`, `Unrecorded: []string{"volatile/**"}`, `Unread: []string{"private/**"}`,
		`"REQUEST_ROLE": "read"`, `RecordExclude: []string{"cache/**", "**/scratch/**"}`,
		`RepoLayer: []string{"resources"}`, `FixtureStep:`,
		"steps.Register(suite)", "return &steps.State{ScenarioRun: run}", "os.Exit(suite.Main(m))",
		"suite.New(t, scenarioID)"} {
		if !strings.Contains(string(source), want) {
			t.Errorf("missing %q:\n%s", want, source)
		}
	}

	if strings.Contains(string(source), `Name: "owned"`) || strings.Contains(string(source), `Name: "remote"`) {
		t.Fatalf("non-vendor CLI substituted:\n%s", source)
	}
}

func TestMainFileDriftIsRefused(t *testing.T) {
	t.Parallel()
	plan, root := wiringPlan(t)

	err := scenariogen.AddMainFiles(plan, root, "docs/scenarios.yaml", "docs/architecture/architecture.yaml")
	if err != nil {
		t.Fatal(err)
	}

	_, err = scenariogen.Write(plan, root)
	if err != nil {
		t.Fatal(err)
	}

	drift, err := scenariogen.Verify(plan, root)
	if err != nil || len(drift) != 0 {
		t.Fatalf("verify generated main: %v %v", drift, err)
	}

	err = disk.Write(filepath.Join(root, "checks/arithmetic/main_test.go"), []byte("stale"), disk.Private)
	if err != nil {
		t.Fatal(err)
	}

	drift, err = scenariogen.Verify(plan, root)
	if err != nil || len(drift) != 1 {
		t.Fatalf("main drift not detected: %v %v", drift, err)
	}
}

func TestMainFilePreservesUnmarkedTestMain(t *testing.T) {
	t.Parallel()
	plan, root := wiringPlan(t)
	target := filepath.Join(root, "checks/arithmetic/custom_test.go")
	original := "package arithmetic_test\nfunc TestMain() {}\n"

	err := disk.Write(target, []byte(original), disk.Private)
	if err != nil {
		t.Fatal(err)
	}

	err = scenariogen.AddMainFiles(plan, root, "docs/scenarios.yaml", "docs/architecture/architecture.yaml")
	if err != nil {
		t.Fatal(err)
	}

	_, err = scenariogen.Write(plan, root)
	if err != nil {
		t.Fatal(err)
	}

	source, err := disk.Read(target)
	if err != nil || string(source) != original {
		t.Fatalf("unmarked main changed: %s %v", source, err)
	}
}
