package release

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/cli"
	"github.com/ondatra-ai/true-bdd/pkg/cli/git"
	"github.com/ondatra-ai/true-bdd/pkg/cli/truebdd"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// Options selects the live pass and an optional proxy-served harness revision.
type Options struct {
	Live            bool
	HarnessRevision string
}

// Rehearse installs artifacts in a fresh private user folder and runs sample hosts.
func Rehearse(ctx context.Context, root, artifacts string, opt Options) (Report, error) {
	report := Report{Live: opt.Live}
	err := rehearse(ctx, root, artifacts, opt, &report)

	return report, errors.Join(err, saveReport(artifacts, report))
}

func rehearse(ctx context.Context, root, artifacts string, opt Options, report *Report) error {
	metadata, err := readMetadata(artifacts)
	if err == nil {
		err = verifyArtifacts(artifacts, metadata)
	}

	err = report.step("", "install checksums", err)
	if err != nil {
		return err
	}

	report.Commit = metadata.Commit

	report.Root, err = disk.TempDir("/tmp", "true-bdd-release-*")

	err = report.step("", "private user folder", err)
	if err != nil {
		return err
	}

	binary, err := installBinary(ctx, report.Root, artifacts, metadata)

	err = report.step("", "binary", err)
	if err != nil {
		return err
	}

	source := filepath.Join(report.Root, "source")

	err = exportSource(ctx, root, metadata.Commit, source)

	err = report.step("", "source", err)
	if err != nil {
		return err
	}

	report.Harness = "exported source at " + metadata.Commit + " (no pushed commit required)"
	if opt.HarnessRevision != "" {
		report.Harness = "module proxy at " + opt.HarnessRevision
	}

	if !opt.Live {
		err = fakeAgents(filepath.Dir(binary))

		err = report.step("", "fake AI CLIs", err)
		if err != nil {
			return err
		}
	}

	return rehearseHosts(ctx, binary, source, opt, report)
}

func installBinary(ctx context.Context, folder, artifacts string, metadata Metadata) (string, error) {
	binDir := filepath.Join(folder, "bin")
	name := "true-bdd_" + metadata.Version + "_" + runtime.GOOS + "_" + runtime.GOARCH + ".tar.gz"

	data, err := disk.Read(filepath.Join(artifacts, name))
	if err == nil {
		err = unpack(binDir, data, true)
	}

	if err != nil {
		return "", fmt.Errorf("install binary: %w", err)
	}

	binary := filepath.Join(binDir, "true-bdd")

	result, err := truebdd.ReleaseVersion(ctx, binary, hostOptions(folder, binary))
	if err == nil {
		err = result.Err()
	}

	if err != nil {
		return "", fmt.Errorf("install binary: %w", err)
	}

	return binary, nil
}

func exportSource(ctx context.Context, root, commit, directory string) error {
	repository := git.Repository{Dir: root}

	archive, err := repository.Archive(ctx, commit)
	if err != nil {
		return fmt.Errorf("exporting source: %w", err)
	}

	return unpack(directory, archive, false)
}

const installBudget = 5 * time.Minute

func hostOptions(host, binary string) cli.Options {
	return cli.Options{Dir: host, Env: git.Environment().Strip("CLAUDECODE", "GOFLAGS").Set(
		"PATH="+filepath.Dir(binary)+string(os.PathListSeparator)+os.Getenv("PATH"), "GOWORK=off"),
		Timeout: installBudget, Group: true}
}

func fakeAgents(directory string) error {
	for _, name := range []string{"claude", "codex"} {
		path := filepath.Join(directory, name)

		err := disk.Write(path, []byte("#!/bin/sh\necho TRUE_BDD_RELEASE_AI_SENTINEL >&2\nexit 3\n"), disk.Private)
		if err == nil {
			err = disk.Executable(path)
		}

		if err != nil {
			return fmt.Errorf("installing fake %s: %w", name, err)
		}
	}

	return nil
}
