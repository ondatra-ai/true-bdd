package docs

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/src/internal/infrastructure/config"
)

const (
	prdRel   = "docs/prd/prd.yaml"
	prdPath  = "./" + prdRel
	archRel  = "docs/architecture/architecture.yaml"
	archPath = "./" + archRel
)

// newTestResolver builds a Resolver over a throwaway project whose
// `documents:` block is exactly the supplied mapping, then creates
// every file named in `present`. ViperConfig always reads
// ./true-bdd/true-bdd.yaml, so the test chdirs into the temp project.
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
		map[string]string{KeyPRD: prdPath},
		[]string{prdRel},
	)

	path, err := resolver.Resolve(KeyPRD)
	if err != nil {
		t.Fatalf("Resolve(prd): unexpected error %v", err)
	}

	if path != prdPath {
		t.Errorf("Resolve(prd) = %q, want ./docs/prd/prd.yaml", path)
	}
}

// TestResolveRejectsMissingFile covers the defect this type exists
// for: the key is known and configured, but nothing is at the path. It
// used to resolve happily and put a dead Read() instruction into every
// prompt that declared it.
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

// TestResolveRejectsDirectory pins that a path which merely EXISTS is
// not good enough. os.Stat succeeds on a directory, so without an
// explicit regular-file check the run passes preflight and then hands a
// model a Read instruction for a directory — the same silent mid-run
// failure the up-front validation was built to stop.
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

func TestResolveRejectsUnknownKey(t *testing.T) {
	resolver := newTestResolver(t, map[string]string{KeyPRD: prdPath},
		[]string{prdRel})

	_, err := resolver.Resolve("no_such_doc")
	if !errors.Is(err, ErrUnknownDocKey) {
		t.Fatalf("Resolve: error = %v, want ErrUnknownDocKey", err)
	}
}

func TestResolveRejectsUnconfiguredPath(t *testing.T) {
	resolver := newTestResolver(t, map[string]string{KeyPRD: ""}, nil)

	_, err := resolver.Resolve(KeyPRD)
	if !errors.Is(err, ErrDocPathNotConfigured) {
		t.Fatalf("Resolve: error = %v, want ErrDocPathNotConfigured", err)
	}
}

// TestResolveAllReportsEveryProblem is the aggregation contract: two
// broken documents must both be named by one run, so a single startup
// failure tells you everything you have to fix.
func TestResolveAllReportsEveryProblem(t *testing.T) {
	resolver := newTestResolver(t, map[string]string{
		KeyPRD:              prdPath,
		KeyArchitectureYAML: archPath,
	}, nil)

	_, err := resolver.ResolveAll([]string{KeyPRD, KeyArchitectureYAML})
	if err == nil {
		t.Fatal("ResolveAll: expected an error, got nil")
	}

	for _, want := range []string{prdPath, archPath} {
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
		map[string]string{KeyPRD: prdPath},
		nil,
	)

	_, err := resolver.ResolveAll([]string{KeyPRD, KeyPRD, KeyPRD})
	if err == nil {
		t.Fatal("ResolveAll: expected an error, got nil")
	}

	if strings.Count(err.Error(), prdPath) != 1 {
		t.Errorf("duplicate keys must be reported once, got %q", err)
	}
}

func TestResolveAllAcceptsSatisfiedSet(t *testing.T) {
	resolver := newTestResolver(t, map[string]string{
		KeyPRD:              prdPath,
		KeyArchitectureYAML: archPath,
	}, []string{prdRel, archRel})

	paths, err := resolver.ResolveAll([]string{KeyPRD, KeyArchitectureYAML})
	if err != nil {
		t.Fatalf("ResolveAll: unexpected error %v", err)
	}

	if len(paths) != 2 {
		t.Errorf("ResolveAll returned %d paths, want 2", len(paths))
	}
}
