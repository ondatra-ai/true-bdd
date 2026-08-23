package steps

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/playwright-community/playwright-go"
)

// npmInstallTimeout caps fetching playwright-core.
const npmInstallTimeout = 5 * time.Minute

// ensureDriver assembles a Playwright driver directory from npm instead
// of playwright-go's own downloader — its CDN zip URLs 404 (mirrors) or
// 400 (redirect target) for every version. Cached under tmp/, keyed by version.
func ensureDriver(ctx context.Context, repoRoot string) (string, error) {
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

	err = installDriverPackage(ctx, dir, version)
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
func installDriverPackage(ctx context.Context, dir, version string) error {
	staging := dir + ".staging"

	err := os.RemoveAll(staging)
	if err != nil {
		return fmt.Errorf("clear the driver staging dir: %w", err)
	}

	err = os.MkdirAll(staging, dirPerm)
	if err != nil {
		return fmt.Errorf("create the driver staging dir: %w", err)
	}

	installCtx, cancel := context.WithTimeout(ctx, npmInstallTimeout)
	defer cancel()

	install := exec.CommandContext(installCtx, "npm", "install",
		"--no-save", "--no-package-lock", "--prefix", staging,
		"playwright-core@"+version)
	install.Stdout = os.Stderr
	install.Stderr = os.Stderr

	err = install.Run()
	if err != nil {
		return fmt.Errorf("fetch playwright-core@%s: %w", version, err)
	}

	err = os.RemoveAll(dir)
	if err != nil {
		return fmt.Errorf("clear the driver dir: %w", err)
	}

	err = os.MkdirAll(dir, dirPerm)
	if err != nil {
		return fmt.Errorf("create the driver dir: %w", err)
	}

	err = os.Rename(filepath.Join(staging, "node_modules", "playwright-core"),
		filepath.Join(dir, "package"))
	if err != nil {
		return fmt.Errorf("install the driver package: %w", err)
	}

	err = os.RemoveAll(staging)
	if err != nil {
		return fmt.Errorf("clear the driver staging dir: %w", err)
	}

	return nil
}

// linkNode puts the node the suite is already using where the library
// looks for it. A symlink rather than a copy: it is the same binary, and
// a copy would go stale the next time node is upgraded.
func linkNode(dir string) error {
	node, err := exec.LookPath("node")
	if err != nil {
		return fmt.Errorf("locate node for the driver: %w", err)
	}

	link := filepath.Join(dir, "node")

	err = os.RemoveAll(link)
	if err != nil {
		return fmt.Errorf("clear the driver's node link: %w", err)
	}

	err = os.Symlink(node, link)
	if err != nil {
		return fmt.Errorf("link node into the driver dir: %w", err)
	}

	return nil
}
