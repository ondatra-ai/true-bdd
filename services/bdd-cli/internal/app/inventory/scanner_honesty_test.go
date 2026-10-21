package inventory_test

import (
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/inventory"
)

// The honesty probes pin loader-mirroring chip behavior: the registry and
// architecture chips read a document the way the engine's loaders do.

// honestyRegPath is the canonical registry location under a tmpdir host.
const honestyRegPath = "docs/scenarios.yaml"

// writeInto writes content at root/rel, creating parent directories.
func writeInto(t *testing.T, root, rel, content string) {
	t.Helper()

	path := filepath.Join(root, rel)

	err := disk.Dir(filepath.Dir(path), disk.Shared)
	if err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}

	err = disk.Write(path, []byte(content), disk.Shared)
	if err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

// honestyTree materializes an in-code host folder from a rel->content map
// and returns its root.
func honestyTree(t *testing.T, files map[string]string) string {
	t.Helper()

	root := t.TempDir()
	for rel, content := range files {
		writeInto(t, root, rel, content)
	}

	return root
}

func TestHonestyRegistryChipStates(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		files  map[string]string
		status string
	}{
		{"missing", map[string]string{}, inventory.StatusMissing},
		{"invalid", map[string]string{honestyRegPath: "scenarios:\n  broken: [unclosed\n"}, inventory.StatusInvalid},
		{"present_empty", map[string]string{honestyRegPath: "scenarios: {}\n"}, inventory.StatusPresentEmpty},
		{
			"present",
			map[string]string{honestyRegPath: "scenarios:\n  E2E-001:\n    user_stories: []\n"},
			inventory.StatusPresent,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			wantDoc(t, inventory.Scan(honestyTree(t, testCase.files)), chipRegistry, testCase.status)
		})
	}
}

func TestHonestyArchitectureMirrorsBuildCodeLoader(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		content string
		status  string
	}{
		{"no_services", "architecture:\n  services: []\n", inventory.StatusInvalid},
		{"absent_services", "architecture:\n  vocabulary: {}\n", inventory.StatusInvalid},
		{
			"with_services",
			"architecture:\n  services:\n    - name: svc\n      path: services/svc\n",
			inventory.StatusPresent,
		},
		// The workspace file-as-source schema (task workspace-overview-design-parity,
		// R3): a top-level `services:` map, no `architecture:` wrapper, mirroring
		// derive.ts's deriveArchitecture — same chip serves both UI surfaces.
		{"workspace_schema_no_services", "services: {}\n", inventory.StatusInvalid},
		{
			"workspace_schema_with_services",
			"services:\n  mcp-service:\n    type: custom\n",
			inventory.StatusPresent,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			folder := honestyTree(t, map[string]string{
				"docs/architecture/architecture.yaml": testCase.content,
			})
			wantDoc(t, inventory.Scan(folder), "architecture", testCase.status)
		})
	}
}
