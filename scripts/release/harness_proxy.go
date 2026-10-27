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

const harnessModule = "github.com/ondatra-ai/true-bdd/pkg"

func localHarnessProxy(ctx context.Context, root, source, folder, ref string) (string, string, error) {
	source = filepath.Join(source, "pkg")

	repository := git.Repository{Dir: root}

	commit, err := repository.ResolveCommit(ctx, ref)
	if err != nil {
		return "", "", fmt.Errorf("resolve harness commit: %w", err)
	}

	stamp, err := repository.CommitTime(ctx, commit)
	if err != nil {
		return "", "", fmt.Errorf("resolve harness time: %w", err)
	}

	const shortHash = 12

	version := "v0.0.0-" + stamp.Format("20060102150405") + "-" + commit[:shortHash]
	proxy := filepath.Join(folder, "module-proxy")

	archive, err := zipHarness(source, version)
	if err != nil {
		return "", "", err
	}

	manifest, err := disk.Read(filepath.Join(source, "go.mod"))
	if err != nil {
		return "", "", fmt.Errorf("read harness manifest: %w", err)
	}

	info, err := json.Marshal(map[string]string{"Version": version, "Time": stamp.Format("2006-01-02T15:04:05Z")})
	if err != nil {
		return "", "", fmt.Errorf("encode module info: %w", err)
	}

	for extension, data := range map[string][]byte{".mod": manifest, ".info": info, ".zip": archive} {
		err = disk.Write(filepath.Join(proxy, harnessModule, "@v", version+extension), data, disk.Private)
		if err != nil {
			return "", "", fmt.Errorf("write module proxy: %w", err)
		}
	}

	location := url.URL{Scheme: "file", Path: proxy}

	return version, location.String(), nil
}

func zipHarness(source, version string) ([]byte, error) {
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

		writer, err := archive.Create(harnessModule + "@" + version + "/" + filepath.ToSlash(relative))
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
		return nil, fmt.Errorf("pack harness module: %w", err)
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
			"GONOSUMDB="+harnessModule, "GONOPROXY=none")
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
