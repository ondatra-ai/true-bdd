package ai_test

import (
	"fmt"
	"github.com/pelletier/go-toml/v2"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/adapters/ai"
)

func TestBodyEvidenceReadsExcludeServiceSourceAndSymlinks(t *testing.T) {
	root := t.TempDir()
	suite := filepath.Join(root, "tests/calc")

	service := filepath.Join(root, "services/calc")
	for _, path := range []string{suite, service} {
		err := os.MkdirAll(path, 0750)
		if err != nil {
			t.Fatal(err)
		}
	}

	link := filepath.Join(suite, "service-link")

	err := os.Symlink(service, link)
	if err != nil {
		t.Fatal(err)
	}

	policy := ai.ClaudeGuardPolicy{Replay: ai.ReplayPolicy{ReadRoot: root}, EvidenceRoots: []string{suite}}
	for _, tool := range []string{"Read", "Glob", "Grep"} {
		for _, target := range []struct {
			path    string
			allowed bool
		}{{suite, true}, {service, false}, {link, false}, {root, false}} {
			payload := fmt.Sprintf(`{"tool_name":%q,"cwd":%q,"tool_input":{"path":%q,"pattern":"**/*.go"}}`,
				tool, root, target.path)

			allowed, reason := policy.DecidePayload([]byte(payload))
			if allowed != target.allowed {
				t.Errorf("%s %s: %v (%s)", tool, target.path, allowed, reason)
			}
		}
	}
}

func TestConfinedSchemaTurnCanReturnStructuredOutput(t *testing.T) {
	for _, schema := range []bool{false, true} {
		policy := ai.ClaudeGuardPolicy{Replay: ai.ReplayPolicy{ReadRoot: "/host"}, AllowStructuredOutput: schema}

		allowed, _ := policy.DecidePayload([]byte(`{"tool_name":"StructuredOutput","cwd":"/host","tool_input":{}}`))
		if allowed != schema {
			t.Fatalf("schema=%v: allowed=%v", schema, allowed)
		}
	}
}

func TestCodexBodyProfileGrantsEvidenceWithoutHostSource(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	home := filepath.Join(root, "original-home")
	putCodexFile(t, filepath.Join(home, "auth.json"), `{"fixture":"dummy"}`)
	t.Setenv("CODEX_HOME", home)

	bin := filepath.Join(root, "bin")
	putCodexFile(t, filepath.Join(bin, "codex"), `#!/bin/sh
cp "$CODEX_HOME/config.toml" profile.txt
while [ "$1" != '-o' ]; do shift; done
printf done > "$2"
`)
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))

	suite := filepath.Join(root, "tests/calc")
	spec := filepath.Join(root, "docs/product.yaml")

	_, err = ai.NewCodexProvider().Execute(t.Context(), ai.Request{
		WorkDir: root, TmpDir: filepath.Join(root, "tmp/run"), Label: "body", CodexApply: true,
		Mode: ai.ExecutionMode{ReadRoots: []string{suite, spec}, AllowedTools: []string{
			"Write(./tests/calc/**)", "Write(./tmp/**)"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	profile := readCodexFile(t, filepath.Join(root, "profile.txt"))

	var decoded struct {
		Shell struct {
			Set map[string]string `toml:"set"`
		} `toml:"shell_environment_policy"`
	}

	err = toml.Unmarshal([]byte(profile), &decoded)
	if err != nil {
		t.Fatalf("invalid profile: %v", err)
	}

	if decoded.Shell.Set["PWD"] != root {
		t.Fatalf("confined tools need their working directory: %v", decoded.Shell.Set["PWD"])
	}

	for _, grant := range []string{strconv.Quote(suite) + ` = "write"`, strconv.Quote(spec) + ` = "read"`} {
		if !strings.Contains(profile, grant) {
			t.Errorf("missing evidence grant: %s", grant)
		}
	}

	if strings.Contains(profile, strconv.Quote(root)+` = "read"`) {
		t.Fatal("body profile grants the whole host")
	}
}
