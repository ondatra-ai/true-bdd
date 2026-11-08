package commands_test

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"log/slog"
	"maps"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/commands"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
)

const scenarioRunBareMode = "bare"

const scenarioRunPlainMode = "plain"

const scenarioRunFixMode = "fix"
const scenarioRunStateFile = "tests/calc/steps/state.go"
const scenarioRunArchitecture = "docs/architecture/architecture.yaml"

func TestBuildStepsEnsuresScenarioRunWithoutConceptItems(t *testing.T) {
	for _, mode := range []string{scenarioRunFixMode, scenarioRunBareMode, "present", scenarioRunPlainMode} {
		t.Run(mode, func(t *testing.T) {
			deps, reply := scenarioRunHost(t, mode)
			before := scenarioRunHostFiles(t)
			log := captureScenarioRunLog(t)

			err := commands.RunBuildSteps(t.Context(), deps, "docs/scenarios.yaml", scenarioRunArchitecture,
				mode == scenarioRunFixMode)
			if mode == scenarioRunBareMode {
				if err == nil || !strings.Contains(err.Error(), "tests/calc") {
					t.Fatalf("missing suite failure: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}

			assertScenarioRunFiles(t, before, mode)

			if len(reply.requests) != 0 {
				t.Fatalf("unexpected AI turns: %v", reply.requests)
			}

			asked := strings.Contains(log.String(), `check=scenario-run`)
			if asked != (mode != scenarioRunPlainMode) {
				t.Fatalf("suite Item checks: %s", log.String())
			}

			if mode == scenarioRunFixMode && strings.Count(log.String(), "check=scenario-run") != 4 {
				t.Fatalf("expected failure, recheck, restart and outer walk: %s", log.String())
			}
		})
	}
}

func scenarioRunHost(t *testing.T, mode string) (commands.BuildStepsDeps, *conceptReplies) {
	t.Helper()

	repository, err := filepath.Abs("../../../../..")
	if err != nil {
		t.Fatal(err)
	}

	reply := &conceptReplies{}
	deps := conceptHost(t, reply)

	err = disk.CopyTree(".", filepath.Join(repository, "tests/true-bdd/fixtures/build-steps-harness-wiring/input"),
		disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	if mode != "present" {
		source := "package steps\ntype State struct { Program string; Value int }\n" +
			"func (s *State) Reset() { s.Value = 0 }\n" +
			"type Registrar interface { Step(string, func(*State, []string) error) }\n"

		err = disk.Write(scenarioRunStateFile, []byte(source), disk.Shared)
		if err != nil {
			t.Fatal(err)
		}
	}

	if mode == scenarioRunPlainMode {
		replaceScenarioRunFile(t, scenarioRunArchitecture, "    testkit: {}\n", "")
	}

	return deps, reply
}

func replaceScenarioRunFile(t *testing.T, path, old, replacement string) {
	t.Helper()

	source, err := disk.Read(path)
	if err != nil {
		t.Fatal(err)
	}

	err = disk.Write(path, []byte(strings.Replace(string(source), old, replacement, 1)), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}
}

func captureScenarioRunLog(t *testing.T) *bytes.Buffer {
	t.Helper()

	var log bytes.Buffer

	previous := slog.Default()

	slog.SetDefault(slog.New(slog.NewTextHandler(&log, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	return &log
}

func assertScenarioRunFiles(t *testing.T, before map[string]string, mode string) {
	t.Helper()

	after := scenarioRunHostFiles(t)
	if mode == scenarioRunFixMode {
		source := after[scenarioRunStateFile]
		for _, want := range []string{"*testkit.ScenarioRun", "Program string", "Value", "Reset()"} {
			if !strings.Contains(source, want) {
				t.Fatalf("missing %s in %s", want, source)
			}
		}

		delete(before, scenarioRunStateFile)
		delete(after, scenarioRunStateFile)
	}

	if !maps.Equal(before, after) {
		t.Fatal("unexpected file change")
	}
}

func scenarioRunHostFiles(t *testing.T) map[string]string {
	t.Helper()

	files := make(map[string]string)

	err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() {
			if path == "tmp" {
				return filepath.SkipDir
			}

			return nil
		}

		source, err := disk.Read(path)
		if err != nil {
			return err
		}

		files[path] = string(source)

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	return files
}

func TestBuildStepsChecksEveryDistinctHarnessSuite(t *testing.T) {
	deps, reply := scenarioRunHost(t, "present")
	replaceScenarioRunFile(t, "docs/scenarios.yaml", "tests/calc/calc_test.go", "tests/other/other_test.go")

	replaceScenarioRunFile(t, "docs/scenarios.yaml", "service: calc", "service: other")
	replaceScenarioRunFile(t, scenarioRunArchitecture, "  testing:",
		"  - name: other\n    path: services/other\n    language: go\n"+
			"    public_interface:\n      kind: cli\n      document: docs/architecture/calc-cli.yaml\n  testing:")

	err := disk.CopyTree("tests/other/steps", "tests/calc/steps", disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	before := scenarioRunHostFiles(t)
	log := captureScenarioRunLog(t)

	err = commands.RunBuildSteps(t.Context(), deps, "docs/scenarios.yaml", scenarioRunArchitecture, false)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Count(log.String(), "check=scenario-run") != 2 || len(reply.requests) != 0 {
		t.Fatalf("want two suite Items without AI: %s; turns=%v", log.String(), reply.requests)
	}

	assertScenarioRunFiles(t, before, "present")
}

func TestBuildStepsAsksConceptAndSuiteItemsOnlyTheirChecklistItems(t *testing.T) {
	reply := &verificationReplies{conceptReplies: conceptReplies{
		proposals: []string{patternAnswer(`^Press Halve once for (\w+)\.$`, true)},
	}}
	deps := conceptHost(t, reply)
	prepareVerificationWalk(t, &deps, reply)
	declareScenarioRunForConceptHost(t)
	log := captureScenarioRunLog(t)

	err := commands.RunBuildSteps(t.Context(), deps, "docs/scenarios.yaml", scenarioRunArchitecture, true)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Join(reply.turns, ",") != "pattern,fix,apply,verify,verify" {
		t.Fatalf("unexpected model turns: %v", reply.turns)
	}

	for _, line := range strings.Split(log.String(), "\n") {
		if !strings.Contains(line, `msg="Check answered"`) {
			continue
		}

		suite := strings.Contains(line, "subjectID=tests/calc/steps-scenario-run")

		scenario := strings.Contains(line, "check=scenario-run")
		if suite != scenario {
			t.Fatalf("wrong Item asked: %s", line)
		}
	}

	if strings.Count(log.String(), "check=scenario-run") != 2 {
		t.Fatalf("suite Item missing from both walks: %s", log.String())
	}
}

func declareScenarioRunForConceptHost(t *testing.T) {
	t.Helper()

	source, err := disk.Read(scenarioRunArchitecture)
	if err != nil {
		t.Fatal(err)
	}

	var document map[string]map[string]any

	err = json.Unmarshal(source, &document)
	if err != nil {
		t.Fatal(err)
	}

	testingConfig, ok := document["architecture"]["testing"].(map[string]any)
	if !ok {
		t.Fatal("missing testing configuration")
	}

	testingConfig["testkit"] = map[string]any{}

	encoded, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}

	files := map[string]string{
		scenarioRunArchitecture: string(encoded),
		"go.mod": "module fixture/buildtests\n\ngo 1.25.0\nrequire github.com/ondatra-ai/true-bdd/pkg v0.0.0\n" +
			"replace github.com/ondatra-ai/true-bdd/pkg => ./local-library\n",
		"local-library/go.mod":             "module github.com/ondatra-ai/true-bdd/pkg\ngo 1.25.0\n",
		"local-library/testkit/run.go":     "package testkit\ntype ScenarioRun struct{}\n",
		"local-library/proxy/cli/proxy.go": "package cli\ntype Proxy struct{}\n",
	}
	for path, content := range files {
		err = disk.Write(path, []byte(content), disk.Shared)
		if err != nil {
			t.Fatal(err)
		}
	}
}
