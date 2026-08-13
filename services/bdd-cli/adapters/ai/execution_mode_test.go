package ai_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/adapters/ai"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/config"
)

// Tool specs and globs reused across the ai_test package.
const (
	readSpec    = "Read(**)"
	grepSpec    = "Grep(**)"
	globSpec    = "Glob(**)"
	writeTmp    = "Write(./tmp/**)"
	editAnySpec = "Edit(**)"
	multiAny    = "MultiEdit(**)"
	bashTool    = "Bash"
	tmpGlob     = "./tmp/**"
	writeTool   = "write"
)

// ExecutionMode is the single permission source for all three CLIs, so
// the projection helpers crush and codex depend on need direct cover.
func TestExecutionModeWriteGlobs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mode ai.ExecutionMode
		want []string
	}{
		{
			name: "think mode grants only the tmp glob",
			mode: ai.ExecutionMode{
				AllowedTools:    []string{readSpec, writeTmp, globSpec, grepSpec},
				DisallowedTools: []string{bashTool, editAnySpec, multiAny},
			},
			want: []string{tmpGlob},
		},
		{
			name: "edit mode dedupes the repeated glob across write tools",
			mode: ai.ExecutionMode{
				AllowedTools: []string{
					readSpec, writeTmp, "Edit(./tmp/**)", "MultiEdit(./tmp/**)", grepSpec,
				},
				DisallowedTools: []string{bashTool},
			},
			want: []string{tmpGlob},
		},
		{
			name: "read-only tools grant no writes",
			mode: ai.ExecutionMode{AllowedTools: []string{readSpec, grepSpec}},
			want: []string{},
		},
		{
			name: "the zero mode grants no writes",
			mode: ai.ExecutionMode{},
			want: []string{},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := testCase.mode.WriteGlobs()
			if !slices.Equal(got, testCase.want) {
				t.Errorf("WriteGlobs() = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestExecutionModeAllowsBash(t *testing.T) {
	t.Parallel()

	denied := ai.ExecutionMode{DisallowedTools: []string{bashTool, "Agent", "Task"}}
	if denied.AllowsBash() {
		t.Error("AllowsBash() = true although Bash is disallowed")
	}

	if !(ai.ExecutionMode{}).AllowsBash() {
		t.Error("AllowsBash() = false for a mode with no disallow list")
	}
}

// The bug this covers cost a 30-minute fixture run: the build-code
// applier's system prompt named `services/*` as its allowed root while
// the enforced mode granted only the tmp glob, so the crush guard
// denied every write and the applier reported it could not apply.
// ExecutionMode is what the guard is derived from, so the grant has to
// be here, not in the prompt.
func TestSourceEditModeGrantsProjectRoots(t *testing.T) {
	factory := ai.NewModeFactory(testConfig(t))
	mode := factory.GetSourceEditMode([]string{"services/frontend", "services/backend"})

	globs := mode.WriteGlobs()
	for _, want := range []string{"services/frontend/**", "services/backend/**"} {
		if !slices.Contains(globs, want) {
			t.Errorf("WriteGlobs() = %v, missing %q", globs, want)
		}
	}

	// The applier still writes its own result file under tmp.
	if !slices.Contains(globs, tmpGlob) {
		t.Errorf("WriteGlobs() = %v, want the tmp glob retained", globs)
	}

	// A shell would sidestep the write roots entirely.
	if mode.AllowsBash() {
		t.Error("source edit mode must not allow Bash")
	}
}

// Roots arrive from two sources with two spellings: architecture.yaml
// gives bare paths, host config gives globs. Both must resolve, and an
// empty entry must not widen the grant to the repo root.
func TestSourceEditModeNormalisesRootSpellings(t *testing.T) {
	factory := ai.NewModeFactory(testConfig(t))
	mode := factory.GetSourceEditMode([]string{"services/api/", "./tests/**", "", "   "})

	globs := mode.WriteGlobs()
	for _, want := range []string{"services/api/**", "./tests/**"} {
		if !slices.Contains(globs, want) {
			t.Errorf("WriteGlobs() = %v, missing %q", globs, want)
		}
	}

	for _, glob := range globs {
		if glob == "/**" || glob == "**" {
			t.Errorf("blank root widened the grant: %v", globs)
		}
	}
}

// End of the chain: the grant only matters if the crush guard actually
// resolves it to a writable root.
func TestSourceEditModeReachesTheCrushGuard(t *testing.T) {
	factory := ai.NewModeFactory(testConfig(t))
	mode := factory.GetSourceEditMode([]string{"services/frontend"})
	policy := ai.NewCrushGuardPolicy(mode, "/repo")

	allowed, reason := policy.Decide(writeTool, "/repo/services/frontend/app/page.tsx")
	if !allowed {
		t.Fatalf("guard denied the declared root: %s", reason)
	}

	// The root is a grant, not a blanket: production code only.
	if ok, _ := policy.Decide(writeTool, "/repo/tests/integration/home.spec.ts"); ok {
		t.Error("guard allowed a write outside every declared root")
	}
}

// testConfig builds a ViperConfig from a throwaway host config so mode
// construction is exercised for real without depending on the repo's
// own seed. NewViperConfig resolves ./true-bdd/true-bdd.yaml relative
// to the working directory, hence the chdir (which rules out
// t.Parallel for its callers).
func testConfig(t *testing.T) *config.ViperConfig {
	t.Helper()

	dir := t.TempDir()

	mkdirErr := os.MkdirAll(filepath.Join(dir, "true-bdd"), 0o755)
	if mkdirErr != nil {
		t.Fatalf("mkdir: %v", mkdirErr)
	}

	body := "paths:\n  tmp_glob: \"" + tmpGlob + "\"\n"

	writeErr := os.WriteFile(
		filepath.Join(dir, "true-bdd", "true-bdd.yaml"), []byte(body), 0o600,
	)
	if writeErr != nil {
		t.Fatalf("write config: %v", writeErr)
	}

	t.Chdir(dir)

	cfg, err := config.NewViperConfig()
	if err != nil {
		t.Fatalf("NewViperConfig: %v", err)
	}

	return cfg
}
