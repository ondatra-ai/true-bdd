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

// ensureDriver returns a Playwright driver directory playwright-go can
// run, assembling it from npm instead of letting the library download
// its driver zip.
//
// playwright-go fetches `playwright-<version>-<platform>.zip` from
// Microsoft's CDN. Those URLs are gone — every mirror the library knows
// answers 404, and the host they redirect to answers 400 for every
// version, current ones included. What still works is npm, which is
// where Playwright publishes the same driver as `playwright-core`.
//
// So this builds the layout the library expects — `<dir>/package` (the
// npm package, holding cli.js) beside a `node` executable — and hands
// the directory over. playwright-go then finds an up-to-date driver,
// skips the download entirely, and installs browsers through that very
// cli.js, which reaches a CDN that does answer.
//
// Assembled under tmp/ and keyed by version, so it survives between runs
// and is rebuilt when the library's expected version moves.
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
