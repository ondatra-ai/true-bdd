package testkit

import (
	"encoding/json"
	"flag"
	"path/filepath"
	"slices"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	proxycli "github.com/ondatra-ai/true-bdd/pkg/proxy/cli"
	"github.com/ondatra-ai/true-bdd/pkg/testkit/internal/runner"
)

const (
	dependencyPrivate = "private/**"
	hostPrivate       = "shared/**"
)

func TestSuitePassesRecordExcludeToEveryDependency(t *testing.T) {
	previous := flag.CommandLine
	flag.CommandLine = flag.NewFlagSet("record-exclude", flag.ContinueOnError)

	t.Cleanup(func() { flag.CommandLine = previous })

	dependencies := []proxycli.Dependency{
		{Name: "first", Recording: proxycli.Recording{Unread: []string{dependencyPrivate}}},
		{Name: "second"},
	}
	suite := NewSuite[struct{}]("probe", dependencies, nil, nil, SuiteOptions{RecordExclude: []string{hostPrivate}})

	installed := installedRecordingRules(t, suite)

	if len(installed) != 2 {
		t.Fatalf("dependencies lost: %+v", installed)
	}

	if !slices.Equal(installed["first"].Unread, []string{dependencyPrivate, hostPrivate}) ||
		!slices.Equal(installed["second"].Unread, []string{hostPrivate}) {
		t.Fatalf("exclusions not added: %+v", installed)
	}

	if !slices.Equal(dependencies[0].Recording.Unread, []string{dependencyPrivate}) ||
		len(dependencies[1].Recording.Unread) != 0 {
		t.Fatalf("caller rules mutated: %+v", dependencies)
	}

	state := hookScenarioRun(t)

	state.harness.Dependencies = suite.dependencies
	for _, path := range []string{"shared/file", "private/file"} {
		if !state.omittedPath(path) {
			t.Errorf("run snapshot reads %s", path)
		}
	}

	if state.omittedPath("tmp/fixture-output") {
		t.Fatal("fixture scratch excluded")
	}
}

func installedRecordingRules(t *testing.T, suite *Suite[struct{}]) map[string]proxycli.Recording {
	t.Helper()

	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}

	utility, err := proxycli.Build(root, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	directory, err := suite.installShimNames(t.TempDir(), runner.CallerServices, utility)
	if err != nil {
		t.Fatal(err)
	}

	data, err := disk.Read(filepath.Join(directory, "recording.json"))
	if err != nil {
		t.Fatal(err)
	}

	var installed map[string]proxycli.Recording

	err = json.Unmarshal(data, &installed)
	if err != nil {
		t.Fatal(err)
	}

	return installed
}
