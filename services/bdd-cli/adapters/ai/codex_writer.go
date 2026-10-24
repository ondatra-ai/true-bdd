package ai

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/cli/gotool"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// prepareCodexWriter generates credentials and policy; commands cannot read this home.
func prepareCodexWriter(ctx context.Context, req Request) ([]string, error) {
	home, err := filepath.Abs(artifactPath(req, "codex-home"))
	if err != nil {
		return nil, fmt.Errorf("locate codex home: %w", err)
	}

	paths, err := gotool.Environment(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve codex toolchain: %w", err)
	}

	userHome, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("locate authentication home: %w", err)
	}

	authHome := os.Getenv("CODEX_HOME")
	if authHome == "" {
		authHome = filepath.Join(userHome, ".codex")
	}

	err = disk.Copy(filepath.Join(home, "auth.json"), filepath.Join(authHome, "auth.json"), disk.Private)
	if err != nil {
		return nil, fmt.Errorf("copy codex authentication: %w", err)
	}

	config, err := codexWriterConfig(req, home, paths)
	if err != nil {
		return nil, err
	}

	err = disk.Write(filepath.Join(home, "config.toml"), []byte(config), disk.Private)
	if err != nil {
		return nil, fmt.Errorf("write codex configuration: %w", err)
	}

	return append(os.Environ(), "CODEX_HOME="+home), nil
}

func codexWriterConfig(req Request, home string, paths map[string]string) (string, error) {
	root, err := filepath.EvalSymlinks(req.WorkDir)
	if err != nil {
		return "", fmt.Errorf("resolve codex workspace: %w", err)
	}

	temporary, err := codexInsideRoot(root, filepath.Join(root, "tmp"))
	if err != nil {
		return "", err
	}

	env := codexCommandEnvironment(temporary, paths)
	for _, key := range []string{"HOME", "TMPDIR", codexCacheEnv} {
		err := disk.Dir(env[key], disk.Private)
		if err != nil {
			return "", fmt.Errorf("create codex scratch: %w", err)
		}
	}

	var config strings.Builder
	config.WriteString(codexWriterDefaults)
	config.WriteString(codexRuntimeProfile)

	for _, path := range []string{root, paths["GOROOT"], paths["GOMODCACHE"]} {
		fmt.Fprintf(&config, "%s = \"read\"\n", strconv.Quote(path))
	}

	err = codexWritableRoots(&config, root, append([]string{temporary}, req.Mode.SourceWriteRoots...))
	if err != nil {
		return "", err
	}

	for _, path := range []string{home, "/private/tmp", "/private/var/tmp",
		filepath.Join(paths["GOMODCACHE"], "github.com/ondatra-ai"),
		filepath.Join(paths["GOMODCACHE"], "cache/download/github.com/ondatra-ai")} {
		fmt.Fprintf(&config, "%s = \"deny\"\n", strconv.Quote(path))
	}

	config.WriteString(`
[permissions.writer.network]
enabled = false
[shell_environment_policy]
inherit = "none"
[shell_environment_policy.set]
`)

	for _, key := range slices.Sorted(maps.Keys(env)) {
		fmt.Fprintf(&config, "%s = %s\n", key, strconv.Quote(env[key]))
	}

	return config.String(), nil
}

func codexWritableRoots(config *strings.Builder, root string, paths []string) error {
	for _, path := range paths {
		path = strings.TrimSuffix(path, "/**")
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, path)
		}

		resolved, err := codexInsideRoot(root, path)
		if err != nil {
			return err
		}

		fmt.Fprintf(config, "%s = \"write\"\n", strconv.Quote(resolved))
	}

	return nil
}

func codexInsideRoot(root, path string) (string, error) {
	resolved, err := resolveCodexPath(path)
	if err != nil {
		return "", err
	}

	relative, err := filepath.Rel(root, resolved)
	if err != nil || relative == ".." || strings.HasPrefix(relative, "../") || relative == "." {
		return "", fmt.Errorf("codex write root %q: %w", path, os.ErrPermission)
	}

	return resolved, nil
}

func resolveCodexPath(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return resolved, nil
	}

	if !errors.Is(err, os.ErrNotExist) || filepath.Dir(path) == path {
		return "", fmt.Errorf("resolve codex path: %w", err)
	}

	parent, err := resolveCodexPath(filepath.Dir(path))
	if err != nil {
		return "", err
	}

	return filepath.Join(parent, filepath.Base(path)), nil
}

func codexCommandEnvironment(temporary string, paths map[string]string) map[string]string {
	return map[string]string{
		"PATH": filepath.Join(paths["GOROOT"], "bin") + ":/usr/bin:/bin:/usr/sbin:/sbin",
		"HOME": filepath.Join(temporary, "codex-shell-home"), "TMPDIR": filepath.Join(temporary, "codex-tmp"),
		codexCacheEnv: filepath.Join(temporary, "codex-go-build"), "GOROOT": paths["GOROOT"],
		"GOMODCACHE": paths["GOMODCACHE"], "GOPROXY": codexDisabled, "GOSUMDB": codexDisabled, "GOTOOLCHAIN": "local",
		"GOFLAGS": "-trimpath -buildvcs=false", "GOTELEMETRY": codexDisabled, "CGO_ENABLED": "0",
	}
}

const codexCacheEnv = "GOCACHE"
const codexDisabled = "off"

const codexWriterDefaults = `default_permissions = "writer"
approval_policy = "never"
model_reasoning_effort = "high"
model_context_window = 1050000
web_search = "disabled"
project_doc_max_bytes = 0
allow_login_shell = false
cli_auth_credentials_store = "file"
[history]
persistence = "none"
[features]
apps = false
plugins = false
multi_agent = false
memories = false
shell_snapshot = false
skip_host_skill_discovery = true
`

const codexRuntimeProfile = `
[permissions.writer.filesystem]
# :minimal is the codex 0.162.1 OS runtime baseline (ADR-0021):
#    /bin /sbin /usr/bin /usr/sbin /usr/libexec /usr/lib /usr/share
#    /opt/homebrew/lib /usr/local/lib /Library/Apple
#    /Library/Filesystems/NetFSPlugins /Library/Preferences /Library/Preferences/Logging
#    /var/db /private/var/db (directory records, timezone, eligibility)
#    /etc /private/etc (passwd, master.passwd, protocols, services, localtime)
#    /System/Library/{Frameworks,PrivateFrameworks,SubFrameworks,Extensions}
#    /System/iOSSupport/System/Library/{Frameworks,PrivateFrameworks,SubFrameworks}
#    /Library/Apple/System/Library/{Frameworks,PrivateFrameworks} /Library/Apple/usr/lib
#    /System/Library/CoreServices (directory and version plists)
#    Root/firmlink metadata, /dev metadata, random/urandom, autofs_nowait,
#    null/zero, fd/terminal handles and dtracehelper; OS runtime IPC/syslog.
# Its implicit scratch grants are explicitly denied below.
":minimal" = "read"
`
