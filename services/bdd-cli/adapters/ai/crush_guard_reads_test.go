package ai_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/adapters/ai"
)

func TestCrushGuardLocalReadsStayInsideProject(t *testing.T) {
	t.Parallel()

	policy := thinkPolicy()

	for _, tool := range []string{"view", "ls", "grep", "glob", "diagnostics"} {
		t.Run(tool, func(t *testing.T) {
			t.Parallel()

			for _, target := range []string{"", "/repo", "/repo/docs/spec.yaml", "docs/spec.yaml", "./docs/../spec.yaml"} {
				if allowed, reason := policy.Decide(tool, target); !allowed {
					t.Errorf("read %q denied: %s", target, reason)
				}
			}

			for _, target := range []string{"/neighbour/spec.yaml", "../neighbour/spec.yaml", "/repo-old/spec.yaml"} {
				allowed, reason := policy.Decide(tool, target)
				if allowed || !strings.Contains(reason, "outside read root /repo") {
					t.Errorf("read %q = %v, %q; want denial naming read root", target, allowed, reason)
				}
			}
		})
	}
}

func TestCrushGuardNetworkReadersKeepTheirAllowance(t *testing.T) {
	t.Parallel()

	policy := thinkPolicy()
	for _, tool := range []string{"fetch", "sourcegraph", "context7_query", "mcp_context7_query"} {
		if allowed, reason := policy.Decide(tool, "/neighbour/spec.yaml"); !allowed {
			t.Errorf("%s denied: %s", tool, reason)
		}
	}
}

func TestCrushGuardResolvesReadRootAndTargetLinks(t *testing.T) {
	t.Parallel()
	project := t.TempDir()

	alias := filepath.Join(t.TempDir(), "project")

	err := os.Symlink(project, alias)
	if err != nil {
		t.Fatal(err)
	}

	outside := t.TempDir()

	escape := filepath.Join(project, "escape")

	err = os.Symlink(outside, escape)
	if err != nil {
		t.Fatal(err)
	}

	policy := ai.NewCrushGuardPolicy(ai.ExecutionMode{}, alias)
	for _, target := range []string{filepath.Join(project, "spec.yaml"), filepath.Join(alias, "spec.yaml"), "spec.yaml"} {
		if allowed, reason := policy.Decide("view", target); !allowed {
			t.Errorf("project read %q denied: %s", target, reason)
		}
	}

	for _, target := range []string{
		filepath.Join(escape, "spec.yaml"), "escape/spec.yaml",
		escape + "/../spec.yaml", "escape/../spec.yaml",
	} {
		if allowed, reason := policy.Decide("view", target); allowed {
			t.Errorf("escaped read %q allowed: %s", target, reason)
		}
	}
}
