package release

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/cli"
	"github.com/ondatra-ai/true-bdd/pkg/cli/truebdd"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

const missingLibrary = "example.invalid/true-bdd-missing@v0.0.0"

func rehearseMissingLibrary(ctx context.Context, binary, source string, opt Options, report *Report) error {
	const name = "missing-library"

	host, command, err := prepareMissingLibrary(ctx, binary, source, opt, report)
	if err != nil {
		return report.step(name, "configuration", err)
	}

	result, err := truebdd.RehearseCode(ctx, binary, false, command)
	if err != nil {
		return report.step(name, "missing library", err)
	}

	outcome, evidenceErr := evaluateHost(host, name, result, false)
	outcome.Stage = "missing library refused before AI"
	outcome.Passed = missingLibraryRefused(evidenceErr, result, outcome)

	report.LibraryFailure = &outcome
	if !outcome.Passed {
		return report.step(name, outcome.Stage, fmt.Errorf("%w: expected named library failure without AI; see %s: %w",
			errRefused, outcome.Transcript, evidenceErr))
	}

	return report.step(name, outcome.Stage, nil)
}

func missingLibraryRefused(evidenceErr error, result cli.Result, outcome HostResult) bool {
	return errors.Is(evidenceErr, errRefused) && result.Code > 0 && len(outcome.Turns) == 0 &&
		strings.Contains(result.Stdout, "library download failed") && strings.Contains(result.Stdout, missingLibrary) &&
		!strings.Contains(result.Stdout+result.Stderr, "TRUE_BDD_RELEASE_AI_SENTINEL")
}

func prepareMissingLibrary(ctx context.Context, binary, source string, opt Options,
	report *Report,
) (string, cli.Options, error) {
	const name = "missing-library"

	host := filepath.Join(report.Root, name)
	sample := sampleHost{name: name, fixture: "build-code-spec-scenario", service: "calc"}

	err := prepareSample(source, host, sample)
	if err != nil {
		return "", cli.Options{}, err
	}

	command := rehearsalOptions(host, binary, opt, false)

	result, err := truebdd.InitHost(ctx, binary, command)
	if err == nil {
		err = result.Err()
	}

	if err == nil {
		err = declareTestingLibrary(host, harnessModule, report.HarnessVersion)
	}

	if err == nil {
		err = declareTestingLibrary(host, "example.invalid/true-bdd-missing", "v0.0.0")
	}

	if err != nil {
		return "", cli.Options{}, err
	}

	empty := filepath.Join(report.Root, "empty-proxy")

	err = disk.Dir(empty, disk.Private)
	if err != nil {
		return "", cli.Options{}, fmt.Errorf("prepare empty proxy: %w", err)
	}

	command.Env = command.Env.Set("GOPROXY=file://"+empty, "GOSUMDB=off")

	return host, command, nil
}
