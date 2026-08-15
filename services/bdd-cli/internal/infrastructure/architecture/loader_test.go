package architecture_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/architecture"
)

// specWith renders a one-service architecture document whose e2e layer
// carries the supplied `commands:` body verbatim, so a test can omit a
// key rather than set it empty — those are different YAML and only one
// of them is what a host actually writes.
func specWith(commands string) string {
	return `architecture:
  services:
    - name: calc
      path: services/calc
      language: go
      quality_gate:
        tests:
          e2e:
            path: tests
            framework: go-test
` + commands
}

// writeSpec writes a document to a temp file and returns its path.
func writeSpec(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "architecture.yaml")

	err := os.WriteFile(path, []byte(body), 0o600)
	if err != nil {
		t.Fatalf("write spec: %v", err)
	}

	return path
}

const allThreeCommands = `            commands:
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

	got := arch.Services[0].Tests.E2E.Commands
	if got.Replay != "go test -json ./... -mode=replay" {
		t.Errorf("replay: got %q", got.Replay)
	}

	if got.Record == "" || got.Live == "" {
		t.Errorf("record/live must survive the load even though no command runs them: %+v", got)
	}
}

// A declared layer missing any one of the three modes is refused. All
// three are checked because "the mode we happen to use today is set" is
// exactly the half-declared spec the rule exists to reject.
func TestLoadRejectsEachMissingCommand(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"record": `            commands:
              replay: "go test -json ./..."
              live: "go test -json ./..."
`,
		"replay": `            commands:
              record: "go test -json ./..."
              live: "go test -json ./..."
`,
		"live": `            commands:
              record: "go test -json ./..."
              replay: "go test -json ./..."
`,
		"whole block": "",
	}

	for name, commands := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := architecture.Load(writeSpec(t, specWith(commands)))
			if !errors.Is(err, architecture.ErrMissingLayerCommand) {
				t.Fatalf("want ErrMissingLayerCommand, got %v", err)
			}
		})
	}
}

// The message has to name the service, the layer and the key: a spec
// with two services and two layers apiece has four places this can be
// wrong, and a refusal that names none of them sends the reader hunting.
func TestLoadNamesTheOffendingLayer(t *testing.T) {
	t.Parallel()

	_, err := architecture.Load(writeSpec(t, specWith(`            commands:
              record: "go test -json ./..."
              live: "go test -json ./..."
`)))
	if err == nil {
		t.Fatal("want an error")
	}

	for _, want := range []string{"services[calc]", "quality_gate.tests.e2e", "commands.replay"} {
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

	_, err := architecture.Load(writeSpec(t, specWith(`            commands:
              record: "go test -json ./..."
              replay: "   "
              live: "go test -json ./..."
`)))
	if !errors.Is(err, architecture.ErrMissingLayerCommand) {
		t.Fatalf("want ErrMissingLayerCommand, got %v", err)
	}
}

// A layer with no `framework:` is not declared at all — the walk skips
// it — so the commands rule must not reach it. This is what lets a
// service declare e2e tests and no integration tests.
func TestLoadIgnoresUndeclaredLayer(t *testing.T) {
	t.Parallel()

	_, err := architecture.Load(writeSpec(t, specWith(allThreeCommands)+
		`          integration:
            path: tests/integration
`))
	if err != nil {
		t.Fatalf("an undeclared layer must not require commands: %v", err)
	}
}
