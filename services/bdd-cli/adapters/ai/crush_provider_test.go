package ai_test

import (
	"errors"
	"github.com/ondatra-ai/true-bdd/pkg/console"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/adapters/ai"
	pkgerrors "github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/pkg/errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The provider probes its own executable before spawning. This test binary
// acts as that subprocess boundary and can refuse it independently of crush.
func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "crush-guard" {
		if os.Getenv("TRUE_BDD_TEST_REFUSE_GUARD") == "yes" {
			os.Exit(1)
		}

		_, _ = console.Err().Write([]byte("true-bdd guard: this mode grants no write roots\n"))

		os.Exit(2)
	}

	os.Exit(m.Run())
}

func TestCrushProviderReportsStartedOutcomes(t *testing.T) {
	for _, testCase := range []struct {
		name, script, outcome string
		refuse                bool
	}{
		{name: "cannot start"},
		{name: "exec failure", script: "#!/missing/interpreter\n"},
		{name: "refused before spawn", script: "#!/bin/sh\nexit 0\n", refuse: true},
		{name: "nonzero", script: "#!/bin/sh\nprintf partial\nexit 7\n", outcome: "non-zero exit"},
		{name: "empty", script: "#!/bin/sh\nexit 0\n", outcome: "empty output"},
		{name: "whitespace", script: "#!/bin/sh\nprintf '  \\n'\n", outcome: "empty output"},
		{name: "stall", script: `#!/bin/sh
mkdir -p "$CRUSH_GLOBAL_CONFIG/crush-data/logs"
log="$CRUSH_GLOBAL_CONFIG/crush-data/logs/crush.log"
printf '%s\n' '{"time":"2000-01-01T00:00:00Z","msg":"ModelProvider called"}' > "$log"
sleep 60
`, outcome: "stalled"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			root := t.TempDir()

			installCrush(t, root, testCase.script)

			if testCase.refuse {
				t.Setenv("TRUE_BDD_TEST_REFUSE_GUARD", "yes")
			}

			started := time.Now()

			text, err := ai.NewCrushProvider().Execute(t.Context(), ai.Request{
				WorkDir: root, TmpDir: root, Label: "writer", WatchWriterResponse: true,
			})
			assertPromptStallStop(t, testCase.outcome, started)

			if err == nil {
				t.Fatal("expected failure")
			}

			var failure *pkgerrors.CrushTurnError
			if got := errors.As(err, &failure); got != (testCase.outcome != "") {
				t.Fatalf("Execute = %q, %v; started outcome = %v", text, err, got)
			}

			if failure != nil && failure.Outcome != testCase.outcome {
				t.Fatalf("outcome = %q", failure.Outcome)
			}

			if testCase.name == "nonzero" && !strings.Contains(text, "partial") {
				t.Fatal("lost transcript")
			}
		})
	}
}

func installCrush(t *testing.T, root, script string) {
	t.Helper()

	bin := filepath.Join(root, "bin")

	err := disk.Dir(bin, disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", bin+":/bin:/usr/bin")

	if script == "" {
		t.Setenv("PATH", bin)
	} else {
		path := filepath.Join(bin, "crush")

		err := disk.Write(path, []byte(script), disk.Shared)
		if err != nil {
			t.Fatal(err)
		}

		err = os.Chmod(path, 0o755)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func assertPromptStallStop(t *testing.T, outcome string, started time.Time) {
	t.Helper()

	if outcome == "stalled" && time.Since(started) > 5*time.Second {
		t.Fatal("stall did not stop the process group promptly")
	}
}
