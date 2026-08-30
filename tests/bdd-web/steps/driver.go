package steps

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/cli"
	"github.com/ondatra-ai/true-bdd/pkg/cli/npm"
	"github.com/ondatra-ai/true-bdd/pkg/console"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/playwright-community/playwright-go"
)

// npmInstallTimeout caps fetching playwright-core.
const npmInstallTimeout = 5 * time.Minute

// ensureDriver assembles a Playwright driver directory from npm instead
// of playwright-go's own downloader — its CDN zip URLs 404 (mirrors) or
// 400 (redirect target) for every version. Cached under tmp/, keyed by version.
func ensureDriver(repoRoot string) (string, error) {
	probe, err := playwright.NewDriver(&playwright.RunOptions{})
	if err != nil {
		return "", fmt.Errorf("resolve the driver version: %w", err)
	}

	version := probe.Version
	dir := filepath.Join(repoRoot, "tmp", "playwright-driver", version)

	_, err = os.Stat(filepath.Join(dir, "package", "cli.js"))
	if err == nil {
		return dir, nil
	}

	err = installDriverPackage(dir, version)
	if err != nil {
		return "", err
	}

	err = linkNode(dir)
	if err != nil {
		return "", err
	}

	return dir, nil
}

// installDriverPackage fetches playwright-core at the exact version the
// library expects and moves it into place as `<dir>/package`.
func installDriverPackage(dir, version string) error {
	staging := dir + ".staging"

	err := disk.RemoveTree(staging)
	if err != nil {
		return fmt.Errorf("clear the driver staging dir: %w", err)
	}

	err = disk.Dir(staging, disk.Shared)
	if err != nil {
		return fmt.Errorf("create the driver staging dir: %w", err)
	}

	installed, err := npm.Install(staging, "playwright-core@"+version,
		cli.Options{
			Timeout: npmInstallTimeout,
			Output:  cli.Streams(console.Err(), console.Err()),
		})
	if err == nil {
		err = installed.Err()
	}

	if err != nil {
		return fmt.Errorf("fetch playwright-core@%s: %w", version, err)
	}

	err = disk.RemoveTree(dir)
	if err != nil {
		return fmt.Errorf("clear the driver dir: %w", err)
	}

	err = disk.Dir(dir, disk.Shared)
	if err != nil {
		return fmt.Errorf("create the driver dir: %w", err)
	}

	//nolint:forbidigo // a directory publish, not a file write: pkg/disk's
	// verbs are about one file's bytes and cannot express this.
	err = os.Rename(filepath.Join(staging, "node_modules", "playwright-core"),
		filepath.Join(dir, "package"))
	if err != nil {
		return fmt.Errorf("install the driver package: %w", err)
	}

	err = disk.RemoveTree(staging)
	if err != nil {
		return fmt.Errorf("clear the driver staging dir: %w", err)
	}

	return nil
}

// linkNode puts the node the suite is already using where the library
// looks for it. A symlink rather than a copy: it is the same binary, and
// a copy would go stale the next time node is upgraded.
func linkNode(dir string) error {
	node, err := npm.NodePath()
	if err != nil {
		return fmt.Errorf("locate node for the driver: %w", err)
	}

	link := filepath.Join(dir, "node")

	err = disk.RemoveTree(link)
	if err != nil {
		return fmt.Errorf("clear the driver's node link: %w", err)
	}

	err = os.Symlink(node, link)
	if err != nil {
		return fmt.Errorf("link node into the driver dir: %w", err)
	}

	return nil
}
