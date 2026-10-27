package codex_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/cli/codex"
)

const (
	reportFile   = "report.md"
	largestModel = "gpt-6-astra"
)

func stub(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	script := `#!/bin/sh
if [ "$1" = login ]; then echo 'Logged in using ChatGPT'; exit 0; fi
printf '%s\n' "$@"
cat
`

	err := os.WriteFile(filepath.Join(dir, "codex"), []byte(script), 0o755)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	return dir
}

func TestOperations(t *testing.T) {
	dir := stub(t)

	status := codex.LoginStatus(t.Context(), dir)
	if status != "" {
		t.Fatalf("login: %s", status)
	}

	for _, writer := range []bool{false, true} {
		var output bytes.Buffer

		opts := codex.Options{Dir: dir, Final: reportFile, Writer: writer, Output: &output}

		result, err := codex.StartRun(t.Context(), "brief", opts)
		if err != nil || result.Code != 0 {
			t.Fatalf("start: %v %v", result, err)
		}

		want := "exec\n-s\nread-only\n-c\napproval_policy=\"never\"\n" +
			"-c\nweb_search=\"disabled\"\n-C\n" + dir + "\n-o\nreport.md\n-\nbrief"
		if writer {
			want = strings.Replace(want, "read-only", "danger-full-access", 1)
			want = strings.Replace(want, `approval_policy="never"`,
				"approval_policy=\"never\"\n-c\nmodel_reasoning_effort=\"high\"", 1)
		}

		if output.String() != want {
			t.Fatalf("argv/stdin: %q, want %q", output.String(), want)
		}
	}
}

func TestResume(t *testing.T) {
	dir := stub(t)

	var output bytes.Buffer

	_, err := codex.ResumeRun(t.Context(), "session-175", "retry",
		codex.Options{Dir: dir, Final: reportFile, Search: true, Output: &output})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.HasPrefix(output.String(), "--search\nexec\n") ||
		!strings.Contains(output.String(), "resume\nsession-175\n-o\n") ||
		strings.Contains(output.String(), "web_search") {
		t.Fatal(output.String())
	}
}

func TestModelAndEffort(t *testing.T) {
	dir := stub(t)

	for _, test := range []struct {
		name   string
		writer bool
		model  string
		effort string
		flags  string
	}{
		{name: "reader-default"},
		{name: "reader-model", model: "arbitrary/model:slug", flags: "-m\narbitrary/model:slug\n"},
		{name: "reader-effort", effort: "xhigh", flags: "-c\nmodel_reasoning_effort=\"xhigh\"\n"},
		{name: "reader-both", model: largestModel, effort: "ultra",
			flags: "-m\ngpt-6-astra\n-c\nmodel_reasoning_effort=\"ultra\"\n"},
		{name: "writer-default", writer: true, flags: "-c\nmodel_reasoning_effort=\"high\"\n"},
		{name: "writer-model", writer: true, model: largestModel,
			flags: "-m\ngpt-6-astra\n-c\nmodel_reasoning_effort=\"high\"\n"},
		{name: "writer-effort", writer: true, effort: "low", flags: "-c\nmodel_reasoning_effort=\"low\"\n"},
		{name: "writer-both", writer: true, model: largestModel, effort: "max",
			flags: "-m\ngpt-6-astra\n-c\nmodel_reasoning_effort=\"max\"\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, session := range []string{"", "session-175"} {
				var output bytes.Buffer

				opts := codex.Options{Dir: dir, Final: reportFile, Writer: test.writer,
					Model: test.model, Effort: test.effort, Output: &output}

				_, err := codex.ResumeRun(t.Context(), session, "brief", opts)
				if err != nil {
					t.Fatal(err)
				}

				sandbox := "read-only"
				if test.writer {
					sandbox = "danger-full-access"
				}

				want := "exec\n-s\n" + sandbox + "\n-c\napproval_policy=\"never\"\n" + test.flags +
					"-c\nweb_search=\"disabled\"\n-C\n" + dir + "\n"
				if session != "" {
					want += "resume\n" + session + "\n"
				}

				want += "-o\nreport.md\n-\nbrief"
				if output.String() != want {
					t.Fatalf("session %q argv/stdin: %q, want %q", session, output.String(), want)
				}
			}
		})
	}
}
