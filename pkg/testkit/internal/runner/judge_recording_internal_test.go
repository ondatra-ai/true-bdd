package runner

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"

	proxycli "github.com/ondatra-ai/true-bdd/pkg/proxy/cli"
)

func TestJudgeRecordingIgnoresAnotherRunsConcurrentWrite(t *testing.T) {
	root, own, other, shelf := judgeRecordingProbe(t)
	t.Chdir(root)

	done := make(chan error, 1)

	go func() {
		outcome, err := (&ClaudeJudge{model: "offline-probe"}).Verdict(t.Context(), JudgeRequest{
			Dir: own, Cmd: "probe", Clauses: []string{"the declared output was written"},
		})
		if err == nil && !outcome.Pass {
			err = ErrJudgeMalformedResponse
		}

		done <- err
	}()

	awaitJudgeProbe(t, filepath.Join(shelf, "ready"))
	writeJudgeProbe(t, filepath.Join(other, "concurrent.txt"), "another run", 0o600)
	writeJudgeProbe(t, filepath.Join(shelf, "resume"), "continue", 0o600)

	err := <-done
 if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(shelf, "claude-001/meta.json"))
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
		t.Fatalf("judge cassette must contain only its own write, got %+v", diff)
	}
}

func judgeRecordingProbe(t *testing.T) (string, string, string, string) {
	t.Helper()

	library, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}

	host, shims, programs, shelf := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()
	writeJudgeProbe(t, filepath.Join(host, "go.mod"),
		"module probe\n\ngo 1.25.0\nrequire github.com/ondatra-ai/true-bdd/pkg v0.0.0\n"+
			"replace github.com/ondatra-ai/true-bdd/pkg => "+library+"\n", 0o600)

	utility, err := proxycli.Build(host, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	err = utility.Substitute(shims, []proxycli.Dependency{{Name: "claude"}})
	if err != nil {
		t.Fatal(err)
	}

	script := `#!/bin/sh
set -eu
printf own > judge-created.txt
touch "$PROBE_SHELF/ready"
while [ ! -f "$PROBE_SHELF/resume" ]; do sleep 0.01; done
printf '%s\n' '{"type":"result","structured_output":{"verdicts":[{"rule":1,"verdict":"pass","reason":""}]}}'
`
	writeJudgeProbe(t, filepath.Join(programs, "claude"), script, 0o700)

	goPath, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}

	err = os.Symlink(goPath, filepath.Join(programs, "go"))
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", programs+":/usr/bin:/bin")
	t.Setenv("PROBE_SHELF", shelf)
	restore := ArmProcess(proxycli.Environment(ProxyModeRecord, shims, shelf, t.TempDir(), []string{shims}))
	t.Cleanup(restore)
	root := t.TempDir()

	own, other := filepath.Join(root, "own"), filepath.Join(root, "other")
	for _, directory := range []string{own, other} {
		err = os.Mkdir(directory, 0o700)
		if err != nil {
			t.Fatal(err)
		}
	}

	return root, own, other, shelf
}

func writeJudgeProbe(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()

	err := os.WriteFile(path, []byte(content), mode)
	if err != nil {
		t.Fatal(err)
	}
}

func awaitJudgeProbe(t *testing.T, path string) {
	t.Helper()

	deadline := time.After(10 * time.Second)

	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			t.Fatal("fake judge did not reach the snapshot barrier")
		case <-ticker.C:
			_, err := os.Stat(path)
 if err == nil {
				return
			}
		}
	}
}
