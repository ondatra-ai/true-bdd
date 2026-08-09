package ai_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/src/adapters/ai"
	pkgerrors "github.com/ondatra-ai/true-bdd/src/internal/pkg/errors"
)

const guardWorkDir = "/repo"

func thinkPolicy() ai.CrushGuardPolicy {
	return ai.NewCrushGuardPolicy(ai.ExecutionMode{
		AllowedTools:    []string{readSpec, writeTmp, globSpec, grepSpec},
		DisallowedTools: []string{bashTool, editAnySpec, multiAny},
	}, guardWorkDir)
}

func TestCrushGuardPolicyDerivesWriteRoots(t *testing.T) {
	t.Parallel()

	policy := thinkPolicy()

	want := filepath.Join(guardWorkDir, "tmp") + string(filepath.Separator)
	if len(policy.WriteRoots) != 1 || policy.WriteRoots[0] != want {
		t.Fatalf("WriteRoots = %v, want [%s]", policy.WriteRoots, want)
	}

	if policy.AllowBash {
		t.Error("AllowBash = true although the mode disallows Bash")
	}
}

func TestCrushGuardPolicyDecide(t *testing.T) {
	t.Parallel()

	policy := thinkPolicy()

	tests := []struct {
		name        string
		tool        string
		targetPath  string
		wantAllowed bool
	}{
		{name: "read-only tool", tool: "view", wantAllowed: true},
		{name: "mcp read-only namespace", tool: "mcp_context7_query", wantAllowed: true},
		{name: "write inside the root", tool: writeTool, targetPath: "/repo/tmp/out.yaml", wantAllowed: true},
		{name: "write outside the root", tool: writeTool, targetPath: "/repo/src/main.go"},
		{
			// A sibling directory sharing the root's prefix must not
			// pass — this is why roots are separator-terminated.
			name:       "sibling directory sharing the prefix",
			tool:       "edit",
			targetPath: "/repo/tmpfoo/out.yaml",
		},
		{name: "relative write target", tool: writeTool, targetPath: "tmp/out.yaml"},
		{name: "write with no target", tool: writeTool},
		{name: "shell is denied when the mode denies it", tool: "bash"},
		{
			// Default-deny: `download` writes files and would slip past
			// a deny-list, so unknown tools are treated as writers.
			name: "unknown tool", tool: "teleport",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			allowed, reason := policy.Decide(testCase.tool, testCase.targetPath)
			if allowed != testCase.wantAllowed {
				t.Fatalf("Decide(%q, %q) = %v (%s), want %v",
					testCase.tool, testCase.targetPath, allowed, reason, testCase.wantAllowed)
			}

			if !allowed && reason == "" {
				t.Error("a denial must explain itself")
			}
		})
	}
}

func TestCrushGuardPolicyEditModeAllowsShellAndWrites(t *testing.T) {
	t.Parallel()

	policy := ai.NewCrushGuardPolicy(ai.ExecutionMode{
		AllowedTools: []string{writeTmp, "Edit(./tmp/**)"},
	}, guardWorkDir)

	if !policy.AllowBash {
		t.Error("AllowBash = false although the mode does not disallow Bash")
	}

	if allowed, reason := policy.Decide("edit", "/repo/tmp/registry.yaml"); !allowed {
		t.Errorf("edit inside the write root denied: %s", reason)
	}
}

// The zero policy is what a stray `crush run` outside the engine sees.
// It must deny everything that could mutate the repo.
func TestCrushGuardZeroPolicyFailsClosed(t *testing.T) {
	t.Parallel()

	var policy ai.CrushGuardPolicy

	for _, tool := range []string{writeTool, "edit", "bash", "download"} {
		if allowed, _ := policy.Decide(tool, "/repo/tmp/out.yaml"); allowed {
			t.Errorf("zero policy allowed %q", tool)
		}
	}
}

func TestLoadCrushGuardPolicyRoundTrip(t *testing.T) {
	original := thinkPolicy()

	entry, err := original.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	prefix := ai.CrushPolicyEnvVar + "="
	if len(entry) <= len(prefix) || entry[:len(prefix)] != prefix {
		t.Fatalf("Encode() = %q, want a %s entry", entry, prefix)
	}

	t.Setenv(ai.CrushPolicyEnvVar, entry[len(prefix):])

	loaded, err := ai.LoadCrushGuardPolicy()
	if err != nil {
		t.Fatalf("LoadCrushGuardPolicy: %v", err)
	}

	if len(loaded.WriteRoots) != 1 || loaded.WriteRoots[0] != original.WriteRoots[0] {
		t.Errorf("round-tripped roots = %v, want %v", loaded.WriteRoots, original.WriteRoots)
	}
}

func TestLoadCrushGuardPolicyRejectsMissingAndMalformed(t *testing.T) {
	t.Setenv(ai.CrushPolicyEnvVar, "")

	_, err := ai.LoadCrushGuardPolicy()
	if !errors.Is(err, pkgerrors.ErrCrushPolicyMissing) {
		t.Fatalf("missing policy error = %v, want ErrCrushPolicyMissing", err)
	}

	t.Setenv(ai.CrushPolicyEnvVar, "{not json")

	_, err = ai.LoadCrushGuardPolicy()
	if !errors.Is(err, pkgerrors.ErrCrushPolicyInvalid) {
		t.Fatalf("malformed policy error = %v, want ErrCrushPolicyInvalid", err)
	}
}
