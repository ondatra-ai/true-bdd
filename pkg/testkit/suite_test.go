package testkit_test

import (
	"flag"
	"os"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/testkit"
)

func TestNewSuiteDefersHostFilesAndServiceBuild(t *testing.T) {
	t.Chdir(t.TempDir())

	previous := flag.CommandLine
	flag.CommandLine = flag.NewFlagSet("suite", flag.ContinueOnError)

	t.Cleanup(func() { flag.CommandLine = previous })

	suite := testkit.NewSuite[struct{}]("missing-service", nil, nil, nil)
	if suite == nil || flag.Lookup("mode") == nil || flag.Lookup("engine-bin") == nil {
		t.Fatal("suite construction did not register its options")
	}

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 0 {
		t.Fatalf("suite construction wrote files before a run: %v", entries)
	}
}
