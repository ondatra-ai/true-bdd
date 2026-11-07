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

func TestDeclaredRecordingContractSurvivesRelocation(t *testing.T) {
	rules := proxy.Recording{
		Placeholders: []proxy.Placeholder{{Token: "{{JOB}}", Pattern: `job-\d+`, Directory: "runs"}},
		Unrecorded:   []string{"private/**"}, Unread: []string{"unread/**"},
		ReadOnlyWhen: map[string]string{"REQUEST_ROLE": "read"},
	}
	script := "#!/bin/sh\nset -eu\nmkdir -p private unread\nprintf unrecorded > private/state\n" +
		"ln -s ignored unread/link\n" +
		"printf '%s' \"$2\" > \"$2\"\nprintf scratch > \"$2.scratch\"\nprintf '%s\\n' \"$2\"\n"

	shims, realDir := installProbe(t, localUtility(t), rules, script)
	for _, readOnly := range []bool{false, true} {
		t.Run(map[bool]string{false: "mutable", true: "read-only"}[readOnly], func(t *testing.T) {
			shelf := t.TempDir()
			for _, mode := range []string{"record", replayMode} {
				root, resultPath := runRecordingProbe(t, mode, shims, realDir, shelf, readOnly)
				if mode == replayMode {
					assertOmittedPaths(t, root, resultPath, readOnly)
				}
			}
		})
	}
}

func runRecordingProbe(t *testing.T, mode, shims, realDir, shelf string, readOnly bool) (string, string) {
	t.Helper()

	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	job := "job-11"
	if mode == replayMode {
		job = "job-22"
	}

	resultPath := filepath.Join(root, "runs", job, "answer.txt")

	err = os.MkdirAll(filepath.Dir(resultPath), 0o700)
	if err != nil {
		t.Fatal(err)
	}

	environment := probeEnvironment(t, mode, shims, realDir, shelf, t.TempDir())
	if readOnly {
		environment = append(environment, "REQUEST_ROLE=read")
	}

	result, err := spec.Run([]string{filepath.Join(shims, "probe"), "-o", resultPath}, cli.Options{
		Dir: root, Env: cli.Inherit().Exact(environment), Timeout: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}

	assertRecordingResult(t, result, resultPath)

	return root, resultPath
}

func assertRecordingResult(t *testing.T, result cli.Result, resultPath string) {
	t.Helper()

	if result.Code != 0 {
		t.Fatalf("spawn: %d: %s", result.Code, result.Stderr)
	}

	data, err := os.ReadFile(resultPath)
	if err != nil || string(data) != resultPath || strings.TrimSpace(result.Stdout) != resultPath {
		t.Fatalf("relocation: %q %q %v", data, result.Stdout, err)
	}
}

func assertOmittedPaths(t *testing.T, root, resultPath string, readOnly bool) {
	t.Helper()

	for _, name := range []string{"private/state", "unread/link"} {
		_, err := os.Lstat(filepath.Join(root, name))
		if !os.IsNotExist(err) {
			t.Fatalf("omitted path replayed: %s", name)
		}
	}

	_, err := os.Stat(resultPath + ".scratch")
	if os.IsNotExist(err) != readOnly {
		t.Fatalf("read-only=%v, scratch: %v", readOnly, err)
	}
}

func localUtility(t *testing.T) *proxy.Utility {
	t.Helper()

	library, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}

	host := t.TempDir()
	manifest := "module example.com/host\n\ngo 1.25.0\nrequire github.com/ondatra-ai/true-bdd/pkg v0.0.0\n" +
		"replace github.com/ondatra-ai/true-bdd/pkg => " + library + "\n"
	writeProbeFile(t, filepath.Join(host, "go.mod"), manifest, 0o600)

	utility, err := proxy.Build(host, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	return utility
}
