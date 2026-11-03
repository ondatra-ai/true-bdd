package claude_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/cli/claude"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
)

func TestClaudeReceivesPromptLargerThanArgumentLimit(t *testing.T) {
	installPromptReceiver(t, "stdin")

	// Over 4 MiB: exceeds macOS ARG_MAX and Linux's single-argument limit.
	prompt := `"` + strings.Repeat(`λ\tline\n`, 500000) + `tail"`

	got, err := claude.Run(prompt, claude.Options{Minimal: true})
	if err != nil {
		t.Fatal(err)
	}

	if got != prompt {
		t.Fatalf("received %d bytes, want all %d bytes unchanged", len(got), len(prompt))
	}
}

func TestClaudePromptTransportForEveryTurnMode(t *testing.T) {
	for _, form := range []string{"argv", "stdin"} {
		for _, mode := range []string{"json", "schema", "stream", "stream schema", "writer"} {
			t.Run(form+"/"+mode, func(t *testing.T) {
				installPromptReceiver(t, form)

				prompt := `"a small prompt with λ and escaped whitespace\n\t"`
				if form == "stdin" {
					prompt = `"` + strings.Repeat(`λ\tline\n`, 500000) + `tail"`
				}

				opts := claude.Options{Minimal: true, SystemPrompt: "system context", Model: "test-model",
					PermissionMode: "acceptEdits", AllowedTools: "Read(**)", DisallowedTools: []string{"Bash"}}
				if strings.Contains(mode, "schema") {
					opts.Schema = `{"type":"string"}`
				}

				if mode == "writer" {
					opts.Writer = &claude.WriterOptions{Settings: "settings.json", SessionID: "test-session"}
				}

				var (
					answer claude.Answer
					err    error
				)
				if mode == "json" || mode == "schema" {
					answer, err = claude.RunJSON(prompt, opts)
				} else {
					answer, err = claude.RunStreamContext(t.Context(), prompt, opts, func(claude.Event) {})
				}

				if err != nil {
					t.Fatal(err)
				}

				if string(answer.Data) != prompt {
					t.Fatalf("received %d bytes, want all %d bytes unchanged", len(answer.Data), len(prompt))
				}
			})
		}
	}
}

func TestClaudePromptBudgetIncludesEnvironmentAndFlags(t *testing.T) {
	for _, source := range []string{"environment", "flags"} {
		t.Run(source, func(t *testing.T) {
			installPromptReceiver(t, "stdin")

			// Individually small strings exceed the total only together.
			opts := claude.Options{}

			if source == "environment" {
				for index := range 18 {
					t.Setenv(fmt.Sprintf("TRUE_BDD_TEST_PADDING_%d", index), strings.Repeat("e", 4096))
				}
			} else {
				opts.SystemPrompt = strings.Repeat("s", 72*1024)
			}

			prompt := strings.Repeat("p", 48*1024)

			got, err := claude.Run(prompt, opts)
			if err != nil {
				t.Fatal(err)
			}

			if got != prompt {
				t.Fatalf("received %d bytes, want %d", len(got), len(prompt))
			}
		})
	}
}

func TestClaudeSmallRawPromptKeepsArgumentForm(t *testing.T) {
	installPromptReceiver(t, "argv")

	// An option-looking prompt and trailing whitespace must remain literal.
	prompt := "-p\nλ\t \n"

	got, err := claude.Run(prompt, claude.Options{})
	if err != nil {
		t.Fatal(err)
	}

	if got != prompt {
		t.Fatalf("received %q, want %q", got, prompt)
	}
}

func installPromptReceiver(t *testing.T, form string) {
	t.Helper()

	root := t.TempDir()
	script := `#!/bin/sh
envelope=no
while [ "$#" -gt 0 ]; do
  case "$1" in
    --output-format) envelope=yes; shift ;;
    -p) shift; break ;;
  esac
  shift
done
form=stdin
if [ "$#" -gt 0 ]; then form=argv; fi
if [ "$form" != "$TRUE_BDD_TEST_PROMPT_FORM" ]; then exit 7; fi
if [ "$envelope" = yes ]; then printf '{"type":"result","structured_output":'; fi
if [ "$form" = argv ]; then printf '%s' "$1"; else /bin/cat; fi
if [ "$envelope" = yes ]; then printf '}\n'; fi
`

	path := filepath.Join(root, "claude")

	err := disk.Write(path, []byte(script), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	err = disk.Executable(path)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("TRUE_BDD_TEST_PROMPT_FORM", form)
	t.Setenv("PATH", root)
}
