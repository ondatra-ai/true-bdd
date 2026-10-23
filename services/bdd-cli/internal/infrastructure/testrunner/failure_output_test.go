package testrunner_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/testrunner"
)

func TestGoDiscoveryKeepsWholeRuntimeFailure(t *testing.T) {
	t.Parallel()

	const longLineBytes = 5 * 1024 * 1024

	output := "EVIDENCE-BEGIN\n" + strings.Repeat("e", longLineBytes) + "\nEVIDENCE-END\n"
	root := t.TempDir()

	var payload strings.Builder

	for _, event := range []map[string]string{
		{"Action": "output", "Package": "fixture", "Test": "TestWhole", "Output": output},
		{"Action": "fail", "Package": "fixture", "Test": "TestWhole"},
	} {
		data, err := json.Marshal(event)
		if err != nil {
			t.Fatal(err)
		}

		payload.Write(data)
		payload.WriteByte('\n')
	}

	err := disk.Write(filepath.Join(root, "events.json"), []byte(payload.String()), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	cfg := testrunner.Config{
		Framework: testrunner.FrameworkGoTest, ConfigFile: filepath.Join(root, "go.mod"), Command: "cat events.json",
	}

	failures, err := testrunner.NewGoTestRunner(nil).Discover(context.Background(), cfg, "", "")
	if err != nil || len(failures) != 1 {
		t.Fatalf("discovery: %v, %v", failures, err)
	}

	if failures[0].FailureOutput != output {
		t.Fatalf("failure must retain all %d bytes, got %d", len(output), len(failures[0].FailureOutput))
	}
}

func TestJSONRunnersKeepWholeRuntimeFailure(t *testing.T) {
	t.Parallel()

	const evidenceLines = 1000

	output := "EVIDENCE-BEGIN\n" + strings.Repeat("full runtime evidence\n", evidenceLines) + "EVIDENCE-END\n"

	encoded, err := json.Marshal(output)
	if err != nil {
		t.Fatal(err)
	}

	for _, testCase := range []struct {
		framework, report string
		runner            testrunner.Runner
	}{
		{testrunner.FrameworkJest,
			`{"testResults":[{"name":"whole.test.js","assertionResults":[{` +
				`"fullName":"whole","status":"failed","failureMessages":[` + string(encoded) + `]}]}]}`,
			testrunner.NewJestRunner(nil)},
		{testrunner.FrameworkPlaywright,
			`{"suites":[{"file":"whole.spec.ts","specs":[{"title":"whole","tests":[{"results":[{` +
				`"status":"failed","errors":[{"message":` + string(encoded) + `}]}]}]}]}]}`,
			testrunner.NewPlaywrightRunner(nil)},
	} {
		t.Run(testCase.framework, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()

			writeErr := disk.Write(filepath.Join(root, "report.json"), []byte(testCase.report), disk.Shared)
			if writeErr != nil {
				t.Fatal(writeErr)
			}

			cfg := testrunner.Config{
				Framework: testCase.framework, ConfigFile: filepath.Join(root, "config"), Command: "cat report.json",
			}

			failures, discoverErr := testCase.runner.Discover(context.Background(), cfg, "", "")
			if discoverErr != nil || len(failures) != 1 {
				t.Fatalf("discovery: %v, %v", failures, discoverErr)
			}

			if !strings.Contains(failures[0].FailureOutput, output) {
				t.Fatalf("whole runtime evidence missing: got %d bytes", len(failures[0].FailureOutput))
			}
		})
	}
}
