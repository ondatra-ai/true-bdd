package release

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/cli"
	"github.com/ondatra-ai/true-bdd/pkg/cli/git"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// Options selects the live pass and an optional proxy-served revision of the engine and harness.
type Options struct {
	Live            bool
	HarnessRevision string
	harnessRef      string
	engineVersion   string
	moduleProxy     string
	moduleCache     string
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

	binary, err := installBinary(report.Root, artifacts, metadata)

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

	opt, err = prepareRehearsalLibraries(ctx, root, source, metadata, opt, report)
	if err != nil {
		return err
	}

	err = report.step("", "go install", rehearseGoInstall(report.Root, opt))
	if err != nil {
		return err
	}

	if !opt.Live {
		err = fakeAgents(filepath.Dir(binary))

		err = report.step("", "fake AI CLIs", err)
		if err != nil {
			return err
		}
	}

	return runRehearsalHosts(ctx, binary, source, opt, report)
}

func runRehearsalHosts(ctx context.Context, binary, source string, opt Options, report *Report) error {
	err := rehearseHosts(ctx, binary, source, opt, report)
	if err != nil {
		return err
	}

	if !opt.Live {
		err = rehearseMissingLibrary(ctx, binary, source, opt, report)
		report.Passed = err == nil
	}

	return err
}

// installBinary follows the guide's archive channel: the native archive and checksums.txt,
// copied where a person would download them, checked, unpacked and put on a private PATH.
func installBinary(folder, artifacts string, metadata Metadata) (string, error) {
	download := filepath.Join(folder, "download")
	archive := fillGuide("true-bdd_vX.Y.Z_<os>_<arch>.tar.gz", metadata.Version, metadata.Commit)

	for _, name := range []string{archive, "checksums.txt"} {
		data, err := disk.Read(filepath.Join(artifacts, name))
		if err == nil {
			err = disk.Write(filepath.Join(download, name), data, disk.Private)
		}

		if err != nil {
			return "", fmt.Errorf("install binary: download %s: %w", name, err)
		}
	}

	home := filepath.Join(folder, "home")
	binary := filepath.Join(home, ".local", "bin", "true-bdd")
	env := hostOptions(folder, binary).Env.Set("HOME=" + home)

	err := runGuide(download, env, nil, fillGuide(guideVerify, metadata.Version, metadata.Commit),
		fillGuide(guideUnpack, metadata.Version, metadata.Commit), guideOnPath)
	if err == nil {
		err = checkVersion(folder, env, metadata.Version, metadata.Commit)
	}

	if err != nil {
		return "", fmt.Errorf("install binary: %w", err)
	}

	return binary, nil
}

// rehearseGoInstall follows the guide's `go install` channel from the rehearsal's module proxy.
// A module build carries no VCS metadata, so it reports its version and an unknown commit.
func rehearseGoInstall(folder string, opt Options) error {
	gobin := filepath.Join(folder, "gobin")
	env := rehearsalOptions(folder, filepath.Join(gobin, "true-bdd"), opt, true).Env.Set("GOBIN=" + gobin)

	err := runGuide(folder, env, nil, fillGuide(guideGoInstall, opt.engineVersion, ""))
	if err != nil {
		return err
	}

	version := opt.engineVersion
	if opt.HarnessRevision != "" {
		version = ""
	}

	return checkVersion(folder, env, version, "unknown")
}

// checkVersion runs the guide's `true-bdd version` and holds its answer to the build identity;
// an empty version accepts any.
func checkVersion(dir string, env cli.Env, version, commit string) error {
	var answer bytes.Buffer

	err := runGuide(dir, env, &answer, guideVersion)
	if err != nil {
		return err
	}

	pattern := `^true-bdd version ` + regexp.QuoteMeta(version) + ` commit ` + regexp.QuoteMeta(commit) + "\n$"
	if version == "" {
		pattern = `^true-bdd version \S+ commit ` + regexp.QuoteMeta(commit) + "\n$"
	}

	if !regexp.MustCompile(pattern).Match(answer.Bytes()) {
		return fmt.Errorf("%w: true-bdd version answered %q, want version %q and commit %q",
			errRefused, answer.String(), version, commit)
	}

	return nil
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

func prepareRehearsalLibraries(ctx context.Context, root, source string,
	metadata Metadata, opt Options, report *Report,
) (Options, error) {
	opt.moduleCache = filepath.Join(report.Root, "module-cache")
	report.ModuleCache = opt.moduleCache

	if opt.HarnessRevision != "" {
		opt.harnessRef, opt.engineVersion = opt.HarnessRevision, opt.HarnessRevision
		report.Harness = "module proxy at " + opt.HarnessRevision

		return opt, nil
	}

	version, commit, proxy, err := localModuleProxy(ctx, root, source, report.Root, metadata.Commit)

	err = report.step("", "local module proxy", err)
	if err != nil {
		return opt, err
	}

	opt.harnessRef, opt.engineVersion, opt.moduleProxy = commit, version, proxy
	report.Harness = "local file module proxy at " + version + " (no pushed commit required)"

	return opt, nil
}
