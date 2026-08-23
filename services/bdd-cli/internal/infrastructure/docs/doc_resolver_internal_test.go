package docs

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/config"
)

const (
	productRel    = "docs/product/product.yaml"
	productPath   = "./" + productRel
	archRel       = "docs/architecture/architecture.yaml"
	archPath      = "./" + archRel
	scenariosRel  = "docs/scenarios.yaml"
	scenariosPath = "./" + scenariosRel
)

// newTestResolver builds a Resolver over a throwaway project whose
// `documents:` block is exactly the supplied mapping, then creates every
// file named in `present`.
func newTestResolver(t *testing.T, documents map[string]string, present []string) *Resolver {
	t.Helper()

	root := t.TempDir()

	var body strings.Builder

	body.WriteString("documents:\n")

	for key, path := range documents {
		body.WriteString("  " + key + ": \"" + path + "\"\n")
	}

	writeFile(t, filepath.Join(root, "true-bdd", "true-bdd.yaml"), body.String())

	for _, rel := range present {
		writeFile(t, filepath.Join(root, rel), "content\n")
	}

	t.Chdir(root)

	cfg, err := config.NewViperConfig()
	if err != nil {
		t.Fatalf("NewViperConfig: %v", err)
	}

	return NewResolver(cfg)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	err := os.MkdirAll(filepath.Dir(path), 0o755)
	if err != nil {
		t.Fatalf("MkdirAll %s: %v", path, err)
	}

	err = os.WriteFile(path, []byte(content), 0o644)
	if err != nil {
		t.Fatalf("WriteFile %s: %v", path, err)
	}
}

// TestResolveAcceptsExistingDocument is the happy path: a known key,
// configured, and actually on disk.
func TestResolveAcceptsExistingDocument(t *testing.T) {
	resolver := newTestResolver(t,
		map[string]string{KeyProduct: productPath},
		[]string{productRel},
	)

	path, err := resolver.Resolve(KeyProduct)
	if err != nil {
		t.Fatalf("Resolve(product): unexpected error %v", err)
	}

	if path != productPath {
		t.Errorf("Resolve(product) = %q, want ./docs/product/product.yaml", path)
	}
}

// TestResolveRejectsMissingFile guards a real regression: Resolve used to
// succeed on a configured-but-absent path, handing prompts a dead Read()
// instruction.
func TestResolveRejectsMissingFile(t *testing.T) {
	resolver := newTestResolver(t,
		map[string]string{KeyArchitectureYAML: archPath},
		nil,
	)

	_, err := resolver.Resolve(KeyArchitectureYAML)
	if !errors.Is(err, ErrDocFileMissing) {
		t.Fatalf("Resolve: error = %v, want ErrDocFileMissing", err)
	}

	if !strings.Contains(err.Error(), archPath) {
		t.Errorf("error must name the path it looked for, got %q", err)
	}
}

// TestResolveRejectsDirectory pins that a path which merely exists is not
// enough: os.Stat succeeds on directories too, so Resolve must explicitly
// check for a regular file.
func TestResolveRejectsDirectory(t *testing.T) {
	resolver := newTestResolver(t,
		map[string]string{KeyArchitectureYAML: archPath},
		nil,
	)

	// Create the configured path as a DIRECTORY rather than a file.
	err := os.MkdirAll(archPath, 0o755)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	_, err = resolver.Resolve(KeyArchitectureYAML)
	if !errors.Is(err, ErrDocNotRegularFile) {
		t.Fatalf("Resolve: error = %v, want ErrDocNotRegularFile", err)
	}

	if !strings.Contains(err.Error(), archPath) {
		t.Errorf("error must name the offending path, got %q", err)
	}
}

// TestResolveAcceptsScenariosRegistry pins the scenario registry as a
// first-class documents.* key: the cmd layer resolves the registry path
// through here, with no hardcoded fallback anywhere in the engine.
func TestResolveAcceptsScenariosRegistry(t *testing.T) {
	resolver := newTestResolver(t,
		map[string]string{KeyScenariosYAML: scenariosPath},
		[]string{scenariosRel},
	)

	path, err := resolver.Resolve(KeyScenariosYAML)
	if err != nil {
		t.Fatalf("Resolve(scenarios_yaml): unexpected error %v", err)
	}

	if path != scenariosPath {
		t.Errorf("Resolve(scenarios_yaml) = %q, want %q", path, scenariosPath)
	}
}

// TestResolveScenariosRegistryNotConfigured is the breaking-change
// contract: a host config without documents.scenarios_yaml refuses to
// resolve — never a silent fall back to docs/scenarios.yaml.
func TestResolveScenariosRegistryNotConfigured(t *testing.T) {
	resolver := newTestResolver(t,
		map[string]string{KeyProduct: productPath},
		[]string{productRel, scenariosRel},
	)

	_, err := resolver.Resolve(KeyScenariosYAML)
	if !errors.Is(err, ErrDocPathNotConfigured) {
		t.Fatalf("Resolve: error = %v, want ErrDocPathNotConfigured", err)
	}

	if !strings.Contains(err.Error(), "documents.scenarios_yaml") {
		t.Errorf("error must name the config key to set, got %q", err)
	}
}

func TestResolveRejectsUnknownKey(t *testing.T) {
	resolver := newTestResolver(t, map[string]string{KeyProduct: productPath},
		[]string{productRel})

	_, err := resolver.Resolve("no_such_doc")
	if !errors.Is(err, ErrUnknownDocKey) {
		t.Fatalf("Resolve: error = %v, want ErrUnknownDocKey", err)
	}
}

func TestResolveRejectsUnconfiguredPath(t *testing.T) {
	resolver := newTestResolver(t, map[string]string{KeyProduct: ""}, nil)

	_, err := resolver.Resolve(KeyProduct)
	if !errors.Is(err, ErrDocPathNotConfigured) {
		t.Fatalf("Resolve: error = %v, want ErrDocPathNotConfigured", err)
	}
}

// TestResolveAllReportsEveryProblem is the aggregation contract: two
// broken documents must both be named by one run, so a single startup
// failure tells you everything you have to fix.
func TestResolveAllReportsEveryProblem(t *testing.T) {
	resolver := newTestResolver(t, map[string]string{
		KeyProduct:          productPath,
		KeyArchitectureYAML: archPath,
	}, nil)

	_, err := resolver.ResolveAll([]string{KeyProduct, KeyArchitectureYAML})
	if err == nil {
		t.Fatal("ResolveAll: expected an error, got nil")
	}

	for _, want := range []string{productPath, archPath} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("aggregated error must name %s, got %q", want, err)
		}
	}
}

// TestResolveAllDeduplicates proves a key declared by many prompts is
// only reported once — the runner passes the raw union, one entry per
// prompt.
func TestResolveAllDeduplicates(t *testing.T) {
	resolver := newTestResolver(t,
		map[string]string{KeyProduct: productPath},
		nil,
	)

	_, err := resolver.ResolveAll([]string{KeyProduct, KeyProduct, KeyProduct})
	if err == nil {
		t.Fatal("ResolveAll: expected an error, got nil")
	}

	if strings.Count(err.Error(), productPath) != 1 {
		t.Errorf("duplicate keys must be reported once, got %q", err)
	}
}

func TestResolveAllAcceptsSatisfiedSet(t *testing.T) {
	resolver := newTestResolver(t, map[string]string{
		KeyProduct:          productPath,
		KeyArchitectureYAML: archPath,
	}, []string{productRel, archRel})

	paths, err := resolver.ResolveAll([]string{KeyProduct, KeyArchitectureYAML})
	if err != nil {
		t.Fatalf("ResolveAll: unexpected error %v", err)
	}

	if len(paths) != 2 {
		t.Errorf("ResolveAll returned %d paths, want 2", len(paths))
	}
}
