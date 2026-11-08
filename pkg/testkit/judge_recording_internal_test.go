package testkit

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	proxycli "github.com/ondatra-ai/true-bdd/pkg/proxy/cli"
	"github.com/ondatra-ai/true-bdd/pkg/testkit/internal/runner"
)

func TestJudgeRecordingSkipsRecordCommandExclusions(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}

	utility, err := proxycli.Build(root, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	suite := &Suite[struct{}]{options: SuiteOptions{RecordExclude: []string{"tmp/codex-shell-home/**"}}}

	shims, err := suite.installShimNames(t.TempDir(), runner.CallerTests, utility)
	if err != nil {
		t.Fatal(err)
	}

	run, shelf, programs := t.TempDir(), t.TempDir(), t.TempDir()

	private := filepath.Join(run, "tmp/codex-shell-home/runtime-bin")

	err = disk.Dir(private, disk.Private)
	if err != nil {
		t.Fatal(err)
	}

	err = os.Symlink("unread-target", filepath.Join(private, "docker"))
	if err != nil {
		t.Fatal(err)
	}

	installJudgeRecordingProbe(t, programs)
	t.Setenv("PATH", programs+":/usr/bin:/bin")
	t.Cleanup(runner.ArmProcess(proxycli.Environment(runner.ProxyModeRecord, shims, shelf, t.TempDir(), []string{shims})))

	judge, err := runner.NewClaudeJudge(filepath.Join(root, "true-bdd/true-bdd.yaml"), runner.ProxyModeRecord)
	if err != nil {
		t.Fatal(err)
	}

	outcome, err := judge.Verdict(t.Context(), runner.JudgeRequest{
		Dir: run, Cmd: "probe", Clauses: []string{"the declared output was written"},
	})
	if err != nil || !outcome.Pass {
		t.Fatalf("judge recording traversed an excluded directory: %+v, %v", outcome, err)
	}

	assertJudgeRecordingDiff(t, shelf)
}

func installJudgeRecordingProbe(t *testing.T, programs string) {
	t.Helper()

	script := "#!/bin/sh\nset -eu\nprintf private > tmp/codex-shell-home/during\n" +
		"printf observed > judge-created.txt\n" +
		`printf '%s\n' '{"type":"result","structured_output":{"verdicts":[{"rule":1,"verdict":"pass","reason":""}]}}'` + "\n"

	err := disk.Write(filepath.Join(programs, "claude"), []byte(script), disk.Private)
	if err != nil {
		t.Fatal(err)
	}

	err = disk.Executable(filepath.Join(programs, "claude"))
	if err != nil {
		t.Fatal(err)
	}

	goPath, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}

	err = os.Symlink(goPath, filepath.Join(programs, "go"))
	if err != nil {
		t.Fatal(err)
	}
}

func assertJudgeRecordingDiff(t *testing.T, shelf string) {
	t.Helper()

	data, err := disk.Read(filepath.Join(shelf, "claude-001/meta.json"))
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

	diff := manifest.FSDiff
	if !slices.Equal(diff.Created, []string{"judge-created.txt"}) || len(diff.Modified)+len(diff.Deleted) != 0 {
		t.Fatalf("judge cassette must omit the excluded directory: %+v", diff)
	}
}
