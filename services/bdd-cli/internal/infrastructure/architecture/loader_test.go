package architecture_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/architecture"
)

// specWith renders a one-suite, one-service architecture document, splicing
// the given commands: body in verbatim so a test can omit a key instead of
// setting it empty.
func specWith(commands string) string {
	return `architecture:
  testing:
    suites:
      - name: calc
        service: calc
        path: tests
        framework: go-test
` + commands + `  services:
    - name: calc
      path: services/calc
      language: go
`
}

// writeSpec writes a document to a temp file and returns its path.
func writeSpec(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "architecture.yaml")

	err := disk.Write(path, []byte(body), disk.Shared)
	if err != nil {
		t.Fatalf("write spec: %v", err)
	}

	return path
}

const allThreeCommands = `        commands:
          record: "go test -json ./... -mode=record"
          replay: "go test -json ./... -mode=replay"
          live: "go test -json ./..."
`

func TestLoadKeepsAllThreeCommands(t *testing.T) {
	t.Parallel()

	arch, err := architecture.Load(writeSpec(t, specWith(allThreeCommands)))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	got := arch.Suites[0].Commands
	if got.Replay != "go test -json ./... -mode=replay" {
		t.Errorf("replay: got %q", got.Replay)
	}

	if got.Record == "" || got.Live == "" {
		t.Errorf("record/live must survive the load even though no command runs them: %+v", got)
	}
}

// A declared suite missing any one of the three modes is refused. All
// three are checked because "the mode we happen to use today is set" is
// exactly the half-declared spec the rule exists to reject.
func TestLoadRejectsEachMissingCommand(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"record": `        commands:
          replay: "go test -json ./..."
          live: "go test -json ./..."
`,
		"replay": `        commands:
          record: "go test -json ./..."
          live: "go test -json ./..."
`,
		"live": `        commands:
          record: "go test -json ./..."
          replay: "go test -json ./..."
`,
		"whole block": "",
	}

	for name, commands := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := architecture.Load(writeSpec(t, specWith(commands)))
			if !errors.Is(err, architecture.ErrMissingSuiteCommand) {
				t.Fatalf("want ErrMissingSuiteCommand, got %v", err)
			}
		})
	}
}

// The message has to name the suite and the key: a spec with several
// suites has several places this can be wrong, and a refusal that names
// none of them sends the reader hunting.
func TestLoadNamesTheOffendingSuite(t *testing.T) {
	t.Parallel()

	_, err := architecture.Load(writeSpec(t, specWith(`        commands:
          record: "go test -json ./..."
          live: "go test -json ./..."
`)))
	if err == nil {
		t.Fatal("want an error")
	}

	for _, want := range []string{"architecture.testing.suites[calc]", "commands.replay"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}
}

// A whitespace-only command is empty in every sense that matters — it
// splits to no argv — so it is refused like an absent one rather than
// carried to the runner to fail there.
func TestLoadRejectsBlankCommand(t *testing.T) {
	t.Parallel()

	_, err := architecture.Load(writeSpec(t, specWith(`        commands:
          record: "go test -json ./..."
          replay: "   "
          live: "go test -json ./..."
`)))
	if !errors.Is(err, architecture.ErrMissingSuiteCommand) {
		t.Fatalf("want ErrMissingSuiteCommand, got %v", err)
	}
}

// A suite's `service:` is what decides which source root a fix may
// write. A name that matches no declared service grants nothing, so it
// is refused at load rather than discovered as a fix that never lands.
func TestLoadRejectsSuiteNamingUnknownService(t *testing.T) {
	t.Parallel()

	spec := `architecture:
  testing:
    suites:
      - name: calc
        service: typo
        path: tests
        framework: go-test
` + allThreeCommands + `  services:
    - name: calc
      path: services/calc
      language: go
`

	_, err := architecture.Load(writeSpec(t, spec))
	if !errors.Is(err, architecture.ErrUnknownSuiteService) {
		t.Fatalf("want ErrUnknownSuiteService, got %v", err)
	}
}

// A spec that declares services but never says how their tests run
// describes a project no build pipeline can act on.
func TestLoadRejectsSpecWithoutSuites(t *testing.T) {
	t.Parallel()

	spec := `architecture:
  services:
    - name: calc
      path: services/calc
      language: go
`

	_, err := architecture.Load(writeSpec(t, spec))
	if !errors.Is(err, architecture.ErrNoTestSuites) {
		t.Fatalf("want ErrNoTestSuites, got %v", err)
	}
}

// Several suites load, sorted by name, each keeping its own commands —
// the shape a repository with a CLI suite and a web suite writes.
func TestLoadKeepsEverySuite(t *testing.T) {
	t.Parallel()

	spec := `architecture:
  testing:
    suites:
      - name: web
        service: web
        path: tests/web
        framework: go-test
        commands:
          record: "go test -json ./tests/web/ -mode=record"
          replay: "go test -json ./tests/web/ -mode=replay"
          live: "go test -json ./tests/web/"
      - name: calc
        service: calc
        path: tests/calc
        framework: go-test
` + allThreeCommands + `  services:
    - name: calc
      path: services/calc
      language: go
    - name: web
      path: services/web
      language: typescript
`

	arch, err := architecture.Load(writeSpec(t, spec))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(arch.Suites) != 2 {
		t.Fatalf("want 2 suites, got %d", len(arch.Suites))
	}

	if arch.Suites[0].Name != "calc" || arch.Suites[1].Name != "web" {
		t.Errorf("suites must load sorted by name, got %q then %q",
			arch.Suites[0].Name, arch.Suites[1].Name)
	}

	path, ok := arch.ServicePath("web")
	if !ok || path != "services/web" {
		t.Errorf("ServicePath(web) = %q, %v", path, ok)
	}
}
