package cli_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/cli"
	"github.com/ondatra-ai/true-bdd/pkg/cli/spec"
	proxy "github.com/ondatra-ai/true-bdd/pkg/proxy/cli"
)

const replayMode = "replay"

type observedRun struct {
	env       []string
	directory string
}

func (r *observedRun) SetEnv(values ...string)      { r.env = append(r.env, values...) }
func (r *observedRun) ObservationDirectory() string { return r.directory }

type observationFiles struct{ directory, dummy, link, outside string }

func TestPathObservationModeContainsOnlyPermissionBits(t *testing.T) {
	shims, realDir := installProbe(t, localUtility(t), proxy.Recording{}, "#!/bin/sh\nexit 0\n")
	directory := t.TempDir()

	err := os.Chmod(directory, 0o700)
	if err != nil {
		t.Fatal(err)
	}

	observed := &observedRun{directory: t.TempDir()}

	err = proxy.Observe(observed, "probe", []proxy.PathSpec{{Path: directory}})
	if err != nil {
		t.Fatal(err)
	}

	environment := probeEnvironment(t, "observe", shims, realDir, t.TempDir(), observed.directory)
	environment = append(environment, observed.env...)

	result, err := spec.Run([]string{filepath.Join(shims, "probe")}, cli.Options{
		Dir: t.TempDir(), Env: cli.Inherit().Exact(environment), Timeout: time.Minute,
	})
	if err != nil || result.Code != 0 {
		t.Fatalf("spawn: %v, %+v", err, result)
	}

	spawns, err := proxy.Spawns(observed)
	if err != nil || len(spawns) != 1 {
		t.Fatalf("observations: %v %v", spawns, err)
	}

	if got := spawns[0].Paths[directory]; got.Mode != 0o700 || got.Type != "directory" {
		t.Fatalf("directory mode must exclude type bits: %+v", got)
	}
}

func TestObserveRecordsSpawnInEveryModeWithoutReadingSymlinks(t *testing.T) {
	shims, realDir := installProbe(t, localUtility(t), proxy.Recording{}, "#!/bin/sh\nprintf 'done\\n'\n")
	files := observationInputs(t)

	shelf := t.TempDir()
	for _, mode := range []string{"record", replayMode, "observe"} {
		t.Run(mode, func(t *testing.T) { observeProbe(t, mode, shims, realDir, shelf, files) })
	}
}

func observationInputs(t *testing.T) observationFiles {
	t.Helper()
	directory := t.TempDir()
	files := observationFiles{directory: directory, dummy: filepath.Join(directory, "dummy.txt"),
		link: filepath.Join(directory, "link"), outside: filepath.Join(t.TempDir(), "outside.txt")}
	writeProbeFile(t, files.dummy, "fixture\n", 0o600)
	writeProbeFile(t, files.outside, "outside", 0o600)

	err := os.Symlink(files.dummy, files.link)
	if err != nil {
		t.Fatal(err)
	}

	return files
}

func observeProbe(t *testing.T, mode, shims, realDir, shelf string, files observationFiles) {
	t.Helper()
	observed := &observedRun{directory: t.TempDir()}

	err := proxy.Observe(observed, "probe", []proxy.PathSpec{
		{Path: files.directory}, {Path: files.dummy, Contents: true}, {Path: files.link, Contents: true},
		{Path: files.outside, Contents: true, ContentRoot: files.directory},
	})
	if err != nil {
		t.Fatal(err)
	}

	environment := probeEnvironment(t, mode, shims, realDir, shelf, observed.directory)
	environment = append(environment, "PROBE_MARKER=declared")
	environment = append(environment, observed.env...)

	result, err := spec.Run([]string{filepath.Join(shims, "probe"), "argument"}, cli.Options{
		Dir: t.TempDir(), Env: cli.Inherit().Exact(environment), Timeout: time.Minute,
	})
	if err != nil || result.Code != 0 {
		t.Fatalf("spawn: %v, %d: %s", err, result.Code, result.Stderr)
	}

	spawns, err := proxy.Spawns(observed)
	if err != nil || len(spawns) != 1 {
		t.Fatalf("observations: %v %v", spawns, err)
	}

	spawn := spawns[0]
	if spawn.Environment["PROBE_MARKER"] != "declared" || !reflect.DeepEqual(spawn.Args, []string{"argument"}) {
		t.Fatal("lost spawn inputs")
	}

	assertObservedFiles(t, spawn, files)
}

func assertObservedFiles(t *testing.T, spawn proxy.Spawn, files observationFiles) {
	t.Helper()

	if got := spawn.Paths[files.outside]; len(got.Content) != 0 || got.Error == "" {
		t.Fatalf("content escaped: %+v", got)
	}

	if got := spawn.Paths[files.dummy]; got.Mode != 0o600 || got.Type != "file" || string(got.Content) != "fixture\n" {
		t.Fatalf("file: %+v", got)
	}

	if got := spawn.Paths[files.link]; !got.Symlink || len(got.Content) != 0 {
		t.Fatalf("symlink followed: %+v", got)
	}

	if got := spawn.Paths[files.directory]; got.Type != "directory" ||
		!reflect.DeepEqual(got.Entries, []string{"dummy.txt", "link"}) {
		t.Fatalf("directory: %+v", got)
	}
}

func installProbe(t *testing.T, utility *proxy.Utility, rules proxy.Recording, script string) (string, string) {
	t.Helper()
	shims, realDir := t.TempDir(), t.TempDir()

	err := utility.Substitute(shims, []proxy.Dependency{{Name: "probe", Recording: rules}})
	if err != nil {
		t.Fatal(err)
	}

	writeProbeFile(t, filepath.Join(realDir, "probe"), script, 0o700)

	return shims, realDir
}

func writeProbeFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()

	err := os.WriteFile(path, []byte(content), mode)
	if err != nil {
		t.Fatal(err)
	}
}

func probeEnvironment(t *testing.T, mode, shims, realDir, shelf, state string) []string {
	t.Helper()

	environment := proxy.Environment(mode, shims, shelf, state, []string{shims})
	path := strings.Join([]string{shims, realDir, os.Getenv("PATH")}, string(os.PathListSeparator))

	return append(environment, "PATH="+path, "HOME="+t.TempDir(), "GOTELEMETRY=off", "GOENV=off")
}
