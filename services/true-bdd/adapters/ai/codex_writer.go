package ai

import (
	"context"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/cli/gotool"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
)

// codexWriterHome places every turn beneath the subtree denied to all writers.
func codexWriterHome(req Request) (string, error) {
	root, err := filepath.EvalSymlinks(req.WorkDir)
	if err != nil {
		return "", fmt.Errorf("resolve codex workspace: %w", err)
	}

	scratch := req.TmpDir
	if !filepath.IsAbs(scratch) {
		scratch = filepath.Join(root, scratch)
	}

	_, err = codexInsideRoot(root, scratch)
	if err != nil {
		return "", err
	}

	home := filepath.Join(root, "tmp", "codex-homes", filepath.Base(req.TmpDir),
		filepath.Base(artifactPath(req, "codex-home")))

	return codexInsideRoot(root, home)
}

func prepareCodexWriter(ctx context.Context, req Request, home string) ([]string, error) {
	err := disk.Dir(req.TmpDir, disk.Private)
	if err != nil {
		return nil, fmt.Errorf("create codex artifacts: %w", err)
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

	config, err := codexWriterConfig(req, paths)
	if err != nil {
		return nil, err
	}

	err = disk.Copy(filepath.Join(home, "auth.json"), filepath.Join(authHome, "auth.json"), disk.Private)
	if err != nil {
		return nil, fmt.Errorf("copy codex authentication: %w", err)
	}

	err = disk.Write(filepath.Join(home, "config.toml"), []byte(config), disk.Private)
	if err != nil {
		return nil, fmt.Errorf("write codex configuration: %w", err)
	}

	return append(os.Environ(), "CODEX_HOME="+home), nil
}

func codexWriterConfig(req Request, paths map[string]string) (string, error) {
	root, err := filepath.EvalSymlinks(req.WorkDir)
	if err != nil {
		return "", fmt.Errorf("resolve codex workspace: %w", err)
	}

	temporary, err := codexInsideRoot(root, filepath.Join(root, "tmp"))
	if err != nil {
		return "", err
	}

	env, err := codexScratchEnvironment(root, temporary, paths)
	if err != nil {
		return "", err
	}

	var config strings.Builder
	config.WriteString(codexWriterDefaults)
	config.WriteString(codexRuntimeProfile)

	for _, path := range []string{root, paths["GOROOT"]} {
		fmt.Fprintf(&config, "%s = \"read\"\n", strconv.Quote(path))
	}

	fmt.Fprintf(&config, "%s = \"deny\"\n", strconv.Quote(paths["GOMODCACHE"]))

	for _, path := range req.Mode.LibraryReadRoots {
		fmt.Fprintf(&config, "%s = \"read\"\n", strconv.Quote(path))
	}

	err = codexRuntimeGrants(&config, env, root)
	if err != nil {
		return "", err
	}

	err = codexWritableRoots(&config, root, codexWritePatterns(req.Mode, temporary))
	if err != nil {
		return "", err
	}

	for _, path := range []string{filepath.Join(temporary, "library-download"), filepath.Join(temporary, "codex-homes"),
		"/private/tmp", "/private/var/tmp",
		filepath.Join(paths["GOMODCACHE"], "cache/vcs")} {
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

func codexInsideRoot(root, path string) (string, error) {
	resolved, err := resolveWriterPath(path)
	if err != nil {
		return "", err
	}

	relative, err := filepath.Rel(root, resolved)
	if err != nil || relative == ".." || strings.HasPrefix(relative, "../") || relative == "." {
		return "", fmt.Errorf("codex write root %q: %w", path, os.ErrPermission)
	}

	return resolved, nil
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

func codexScratchEnvironment(root, temporary string, paths map[string]string) (map[string]string, error) {
	env := codexCommandEnvironment(temporary, paths)
	for _, key := range []string{"HOME", "TMPDIR", codexCacheEnv} {
		resolved, err := codexInsideRoot(root, env[key])
		if err != nil {
			return nil, err
		}

		relative, err := filepath.Rel(root, resolved)
		if err != nil {
			return nil, fmt.Errorf("locate codex scratch: %w", err)
		}

		env[key] = resolved

		_, err = disk.DirWithin(root, relative, disk.Private)
		if err != nil {
			return nil, fmt.Errorf("create codex scratch: %w", err)
		}
	}

	return env, nil
}
