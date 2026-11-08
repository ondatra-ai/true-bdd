package gates_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/ondatra-ai/true-bdd/pkg/cli"
	"github.com/ondatra-ai/true-bdd/pkg/cli/spec"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
	proxycli "github.com/ondatra-ai/true-bdd/pkg/proxy/cli"
)

func TestRecordingSkipsBuildCachesAndKeepsFixtureScratch(t *testing.T) {
	utility := recordingTestUtility(t)
	for name, rules := range architectureRecordingRules(t) {
		t.Run(name, func(t *testing.T) {
			root, shims, programs, shelf := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()

			err := utility.Substitute(shims, []proxycli.Dependency{{Name: "probe", Recording: rules}})
			if err != nil {
				t.Fatal(err)
			}

			var script strings.Builder
			script.WriteString("#!/bin/sh\nset -eu\n")

			for _, path := range []string{
				"tmp/go-cache", "tmp/go-mod-cache", "tmp/go-tmp", "tmp/calc-module-cache", "tmp/calc-tool-tmp",
				"tmp/codex-homes/session", "tmp/session-codex-home", "tmp/run/codex-go-build",
				"tmp/run/library-download", "tmp/run/codex-shell-home", "tmp/run/codex-tmp",
			} {
				writeRecordingProbe(t, filepath.Join(root, path, ".lock"), "cache")
				// Snapshots reject symlinks; success proves neither snapshot reads these paths.
				err = os.Symlink("unread", filepath.Join(root, path, "before"))
				if err != nil {
					t.Fatal(err)
				}

				script.WriteString("ln -s unread " + path + "/after\n")
			}

			kept := []string{"tmp/calc-calls.log", "tmp/concept-bodies-extra-first-apply", "tmp/hold-ready"}
			for _, path := range kept {
				script.WriteString("printf observed > " + path + "\n")
			}

			writeRecordingProbe(t, filepath.Join(programs, "probe"), script.String())
			addRecordingGo(t, programs)

			err = os.Chmod(filepath.Join(programs, "probe"), 0o700)
			if err != nil {
				t.Fatal(err)
			}

			t.Setenv("PATH", programs+":/usr/bin:/bin")
			environment := proxycli.Environment("record", shims, shelf, t.TempDir(), []string{shims})

			result, err := spec.Run([]string{filepath.Join(shims, "probe")}, cli.Options{
				Dir: root, Env: cli.Inherit().Set(environment...), Timeout: time.Minute,
			})
			if err != nil || result.Code != 0 {
				t.Fatalf("synthetic recording: %+v, %v", result, err)
			}

			assertRecordingPaths(t, shelf, kept)
		})
	}
}

func assertRecordingPaths(t *testing.T, shelf string, want []string) {
	t.Helper()

	data, err := disk.Read(filepath.Join(shelf, "probe-001/meta.json"))
	if err != nil {
		t.Fatal(err)
	}

	var manifest struct {
		FSDiff struct {
			Created  []string `json:"created"`
			Modified []string `json:"modified"`
			Deleted  []string `json:"deleted"`
		} `json:"fsdiff"`
	}

	err = json.Unmarshal(data, &manifest)
	if err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(manifest.FSDiff.Created, want) || len(manifest.FSDiff.Modified)+len(manifest.FSDiff.Deleted) != 0 {
		t.Fatalf("recorded paths: %+v; want only %v", manifest.FSDiff, want)
	}

	for _, path := range want {
		data, err = disk.Read(filepath.Join(shelf, "probe-001/fsdiff/after", path))
		if err != nil || string(data) != "observed" {
			t.Errorf("fixture output %s: %q, %v", path, data, err)
		}
	}
}

func architectureRecordingRules(t *testing.T) map[string]proxycli.Recording {
	t.Helper()

	data, err := disk.Read("../../docs/architecture/architecture.yaml")
	if err != nil {
		t.Fatal(err)
	}

	var document struct {
		Architecture struct {
			Testing struct {
				Commands struct {
					Record struct {
						Exclude []string `yaml:"exclude"`
					} `yaml:"record"`
				} `yaml:"commands"`
			} `yaml:"testing"`
			Services []struct {
				Dependencies []struct {
					Name      string `yaml:"name"`
					Recording struct {
						Unread []string `yaml:"unread"`
					} `yaml:"recording"`
				} `yaml:"dependencies"`
			} `yaml:"services"`
		} `yaml:"architecture"`
	}

	err = yaml.Unmarshal(data, &document)
	if err != nil {
		t.Fatal(err)
	}

	rules := make(map[string]proxycli.Recording)

	for _, service := range document.Architecture.Services {
		for _, dependency := range service.Dependencies {
			if dependency.Name == "claude" || dependency.Name == "codex" {
				rules[dependency.Name] = proxycli.Recording{Unread: append(dependency.Recording.Unread,
					document.Architecture.Testing.Commands.Record.Exclude...)}
			}
		}
	}

	if len(rules) != 2 {
		t.Fatalf("expected both AI CLI recording rules: %v", rules)
	}

	return rules
}

func recordingTestUtility(t *testing.T) *proxycli.Utility {
	t.Helper()

	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}

	utility, err := proxycli.Build(root, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	return utility
}

func writeRecordingProbe(t *testing.T, path, content string) {
	t.Helper()

	err := disk.Dir(filepath.Dir(path), disk.Private)
	if err != nil {
		t.Fatal(err)
	}

	err = disk.Write(path, []byte(content), disk.Private)
	if err != nil {
		t.Fatal(err)
	}
}

func addRecordingGo(t *testing.T, programs string) {
	t.Helper()

	program, err := cli.Find("go")
	if err != nil {
		t.Fatal(err)
	}

	err = os.Symlink(program, filepath.Join(programs, "go"))
	if err != nil {
		t.Fatal(err)
	}
}
