package release

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"

	"github.com/ondatra-ai/true-bdd/pkg/cli"
	"github.com/ondatra-ai/true-bdd/pkg/cli/git"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

const (
	harnessModule = "github.com/ondatra-ai/true-bdd/pkg"
	engineModule  = "github.com/ondatra-ai/true-bdd/services/true-bdd"
	// repositoryModules covers both, for GONOSUMDB: neither is published to the checksum database.
	repositoryModules = "github.com/ondatra-ai/true-bdd"
)

// localModuleProxy serves the test library and the engine from the exported source at ref's
// commit-time pseudo-version, so the guide's `go get …@<commit>` and `go install …@<version>`
// resolve without a pushed commit. It returns that version, the commit and the proxy's URL.
func localModuleProxy(ctx context.Context, root, source, folder, ref string) (string, string, string, error) {
	repository := git.Repository{Dir: root}

	commit, err := repository.ResolveCommit(ctx, ref)
	if err != nil {
		return "", "", "", fmt.Errorf("resolve module commit: %w", err)
	}

	stamp, err := repository.CommitTime(ctx, commit)
	if err != nil {
		return "", "", "", fmt.Errorf("resolve module time: %w", err)
	}

	const shortHash = 12

	version := "v0.0.0-" + stamp.Format("20060102150405") + "-" + commit[:shortHash]
	proxy := filepath.Join(folder, "module-proxy")

	info, err := json.Marshal(map[string]string{"Version": version, "Time": stamp.Format("2006-01-02T15:04:05Z")})
	if err != nil {
		return "", "", "", fmt.Errorf("encode module info: %w", err)
	}

	for module, directory := range map[string]string{harnessModule: "pkg", engineModule: "services/true-bdd"} {
		err = serveModule(proxy, module, filepath.Join(source, directory), version, commit, info)
		if err != nil {
			return "", "", "", err
		}
	}

	location := url.URL{Scheme: "file", Path: proxy}

	return version, commit, location.String(), nil
}

// serveModule writes one module version in the GOPROXY file layout. `latest` and the version
// list answer `go install`'s deprecation query, and `<commit>.info` resolves a commit query.
func serveModule(proxy, module, source, version, commit string, info []byte) error {
	archive, err := zipModule(source, module, version)
	if err != nil {
		return err
	}

	manifest, err := disk.Read(filepath.Join(source, "go.mod"))
	if err != nil {
		return fmt.Errorf("read %s manifest: %w", module, err)
	}

	base := filepath.Join(proxy, module)

	for path, data := range map[string][]byte{
		filepath.Join("@v", version+".mod"):  manifest,
		filepath.Join("@v", version+".info"): info,
		filepath.Join("@v", version+".zip"):  archive,
		filepath.Join("@v", commit+".info"):  info,
		filepath.Join("@v", "list"):          []byte(version + "\n"),
		"@latest":                            info,
	} {
		err = disk.Write(filepath.Join(base, path), data, disk.Private)
		if err != nil {
			return fmt.Errorf("write module proxy: %w", err)
		}
	}

	return nil
}

func zipModule(source, module, version string) ([]byte, error) {
	var buffer bytes.Buffer

	archive := zip.NewWriter(&buffer)

	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if entry.IsDir() {
			return skipNestedModule(source, path, entry.Name())
		}

		if !entry.Type().IsRegular() {
			return nil
		}

		relative, err := filepath.Rel(source, path)
		if err != nil {
			return fmt.Errorf("module path: %w", err)
		}

		data, err := disk.Read(path)
		if err != nil {
			return fmt.Errorf("module source: %w", err)
		}

		writer, err := archive.Create(module + "@" + version + "/" + filepath.ToSlash(relative))
		if err != nil {
			return fmt.Errorf("module zip entry: %w", err)
		}

		_, err = writer.Write(data)
		if err != nil {
			return fmt.Errorf("module zip contents: %w", err)
		}

		return nil
	})
	if err == nil {
		err = archive.Close()
	}

	if err != nil {
		return nil, fmt.Errorf("pack module %s: %w", module, err)
	}

	return buffer.Bytes(), nil
}

func rehearsalOptions(host, binary string, opt Options, install bool) cli.Options {
	command := hostOptions(host, binary)

	cache := opt.moduleCache
	if install {
		cache += "-install"
	}

	command.Env = command.Env.Set("GOMODCACHE=" + cache)
	if opt.moduleProxy != "" {
		command.Env = command.Env.Set("GOPROXY="+opt.moduleProxy+",https://proxy.golang.org",
			"GONOSUMDB="+repositoryModules, "GONOPROXY=none")
	}

	return command
}

func skipNestedModule(source, path, name string) error {
	if path == source {
		return nil
	}

	_, err := os.Stat(filepath.Join(path, "go.mod"))
	if err == nil || name == "vendor" {
		return filepath.SkipDir
	}

	if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("check nested module: %w", err)
	}

	return nil
}
