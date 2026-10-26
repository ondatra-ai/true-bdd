package release

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/cli"
	"github.com/ondatra-ai/true-bdd/pkg/cli/spec"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"gopkg.in/yaml.v3"
)

func evaluateHost(host, name string, result cli.Result, live bool) (HostResult, error) {
	outcome := newHostResult(name)
	outcome.Stage = configurationStage
	outcome.ExitCode = result.Code
	outcome.Log = filepath.Join(host, "tmp/true-bdd.log.json")
	outcome.Transcript = filepath.Join(host, "rehearsal-output.txt")

	transcript := result.Stdout + result.Stderr

	err := disk.Write(outcome.Transcript, []byte(transcript), disk.Private)
	if err != nil {
		return outcome, fmt.Errorf("saving engine transcript: %w", err)
	}

	data, err := disk.Read(outcome.Log)
	if err != nil {
		return outcome, fmt.Errorf("configuration or Specification: reading engine log: %w", err)
	}

	evidence, err := readEvidence(data, &outcome)
	if err != nil {
		return outcome, err
	}

	outcome.Stage = evidence.stage
	if live {
		outcome.Stage = "live build"
		outcome.Passed = result.Code == 0 && strings.Contains(transcript, "BUILD CODE COMPLETE")
	} else {
		sentinel := strings.Contains(transcript, "TRUE_BDD_RELEASE_AI_SENTINEL")
		outcome.Passed = len(outcome.Turns) > 0 && sentinel && result.Code > 0
	}

	if missingCalculatorDownload(name, outcome) {
		outcome.Passed = false

		return outcome, fmt.Errorf("%w: no engine library download before first AI turn; see %s", errRefused, outcome.Log)
	}

	if !outcome.Passed {
		return outcome, fmt.Errorf("%s: %w: engine exit %d; see %s", evidence.stage,
			errRefused, result.Code, outcome.Transcript)
	}

	return outcome, nil
}

func runHostTests(host, name string, opt cli.Options) error {
	data, err := disk.Read(filepath.Join(host, "docs/architecture/architecture.yaml"))
	if err != nil {
		return fmt.Errorf("reading host test command: %w", err)
	}

	var document struct {
		Architecture struct {
			Testing struct {
				Commands struct {
					Replay string `yaml:"replay"`
				} `yaml:"commands"`
			} `yaml:"testing"`
		} `yaml:"architecture"`
	}

	err = yaml.Unmarshal(data, &document)
	if err != nil {
		return fmt.Errorf("decoding host test command: %w", err)
	}

	command := document.Architecture.Testing.Commands.Replay
	if command == "" {
		return fmt.Errorf("%w: missing host test command", errRefused)
	}

	if name == jestHost {
		opt.Dir = filepath.Join(host, "tests")
	}

	phase := spec.Phase{Name: "host tests", Dir: opt.Dir, Env: opt.Env, Budget: opt.Timeout}

	err = phase.Run([]string{command})
	if err != nil {
		return fmt.Errorf("running host tests: %w", err)
	}

	return nil
}

func missingCalculatorDownload(name string, outcome HostResult) bool {
	return name == "calculator" && len(outcome.LibraryDownloads) == 0
}
