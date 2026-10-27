package release

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/cli"
	"github.com/ondatra-ai/true-bdd/pkg/cli/gotool"
	"github.com/ondatra-ai/true-bdd/pkg/cli/npm"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

const jestHost = "jest"
const liveBudget = 2 * time.Hour

type sampleHost struct {
	name    string
	fixture string
	service string
}

func rehearseHosts(ctx context.Context, binary, source string, opt Options, report *Report) error {
	hosts := []sampleHost{
		{name: "calculator", fixture: "build-code-spec-scenario", service: "calc"},
		{name: jestHost, fixture: "build-code-jest-failing-test", service: "greeter"},
	}

	var failures []error

	for _, host := range hosts {
		if host.name == jestHost {
			err := cli.Require("node", "npm")
			if err != nil {
				skipped := newHostResult(host.name)
				skipped.Skipped = "node and npm required: " + err.Error()
				report.Hosts = append(report.Hosts, skipped)
				report.Steps = append(report.Steps, Step{Host: host.name, Name: "sample host", Result: "skipped",
					Detail: "node and npm are not both installed"})

				continue
			}
		}

		before := len(report.Hosts)

		err := rehearseHost(ctx, binary, source, host, opt, report)
		if err != nil {
			failures = append(failures, err)

			if len(report.Hosts) == before {
				failed := newHostResult(host.name)
				failed.Stage = report.Steps[len(report.Steps)-1].Name
				report.Hosts = append(report.Hosts, failed)
			}
		}
	}

	report.Passed = len(failures) == 0

	return errors.Join(failures...)
}

func rehearseHost(ctx context.Context, binary, source string, sample sampleHost, opt Options, report *Report) error {
	host := filepath.Join(report.Root, sample.name)

	err := prepareSample(source, host, sample)

	err = report.step(sample.name, "sample host", err)
	if err != nil {
		return err
	}

	commandOptions := rehearsalOptions(host, binary, opt, false)

	err = runGuide(host, commandOptions.Env, nil, guideInit)

	err = report.step(sample.name, configurationStage, err)
	if err != nil {
		return err
	}

	version, err := installHostRequirements(ctx, sample, rehearsalOptions(host, binary, opt, true), opt)
	if sample.name != jestHost && err == nil {
		report.HarnessVersion = version
	}

	step := "harness"
	if sample.name == jestHost {
		step = "test dependencies"
	}

	err = report.step(sample.name, step, err)
	if err != nil {
		return err
	}

	err = runGuide(host, commandOptions.Env, nil, guideGitInit)

	err = report.step(sample.name, "git init", err)
	if err != nil {
		return err
	}

	return runSample(ctx, binary, host, sample, opt, report)
}

func runSample(ctx context.Context, binary, host string, sample sampleHost, opt Options, report *Report) error {
	commandOptions := rehearsalOptions(host, binary, opt, false)

	if opt.Live {
		commandOptions.Timeout = liveBudget
	}

	started := time.Now()
	result, err := rehearseCode(ctx, binary, opt.Live, commandOptions)
	outcome, evidenceErr := evaluateHost(host, sample.name, result, opt.Live)
	outcome.WallTime = time.Since(started).String()
	report.Hosts = append(report.Hosts, outcome)

	err = errors.Join(err, evidenceErr)

	err = report.step(sample.name, outcome.Stage, err)
	if err != nil {
		return err
	}

	if opt.Live {
		err = runHostTests(host, sample.name, commandOptions)
		report.Hosts[len(report.Hosts)-1].WallTime = time.Since(started).String()

		err = report.step(sample.name, "host tests", err)
		if err != nil {
			report.Hosts[len(report.Hosts)-1].Passed = false

			return err
		}
	}

	return nil
}

func prepareSample(source, host string, sample sampleHost) error {
	input := filepath.Join(source, "tests/true-bdd/fixtures", sample.fixture, "input")

	err := disk.CopyTree(host, input, disk.Private)
	if err != nil {
		return fmt.Errorf("copying sample host: %w", err)
	}

	for _, path := range []string{"true-bdd", "templates", filepath.Join("services", sample.service)} {
		err := disk.RemoveTree(filepath.Join(host, path))
		if err != nil {
			return fmt.Errorf("removing sample %s: %w", path, err)
		}
	}

	return nil
}

// installHostRequirements installs a host's test dependencies: a Jest host its own npm packages,
// a Go host the test library by the guide, at the release's commit, pinned in testing.libraries.
func installHostRequirements(ctx context.Context, sample sampleHost,
	commandOptions cli.Options, opt Options,
) (string, error) {
	if sample.name == jestHost {
		err := npm.InstallTests(ctx, commandOptions)
		if err != nil {
			return "", fmt.Errorf("install dependencies: %w", err)
		}

		return "", nil
	}

	err := gotool.DropHarness(ctx, commandOptions)
	if err == nil {
		err = dropTestingLibrary(commandOptions.Dir, repositoryModules)
	}

	if err == nil {
		err = runGuide(commandOptions.Dir, commandOptions.Env, nil,
			fillGuide(guideHarness, "", opt.harnessRef))
	}

	var listed bytes.Buffer

	if err == nil {
		err = runGuide(commandOptions.Dir, commandOptions.Env, &listed, guideHarnessVersion)
	}

	if err != nil {
		return "", fmt.Errorf("install harness: %w", err)
	}

	version, err := listedVersion(listed.String(), harnessModule)
	if err != nil {
		return "", err
	}

	err = declareTestingLibrary(commandOptions.Dir, harnessModule, version)

	return version, err
}

// listedVersion reads a module's version from `go list -m <module>` output.
func listedVersion(listed, module string) (string, error) {
	for line := range strings.SplitSeq(listed, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == module {
			return fields[1], nil
		}
	}

	return "", fmt.Errorf("%w: go list -m %s printed no version: %q", errRefused, module, listed)
}
