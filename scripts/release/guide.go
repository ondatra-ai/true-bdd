package release

import (
	"bytes"
	"fmt"
	"io"
	"runtime"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/cli"
	"github.com/ondatra-ai/true-bdd/pkg/cli/spec"
)

// The install guide's commands, as README.md quotes them between the guide markers.
// The rehearsal runs each one as written, its placeholders filled; only the first run goes
// through its typed wrapper, which answers the fix menu.
const (
	guideVerify         = `grep " true-bdd_vX.Y.Z_<os>_<arch>.tar.gz$" checksums.txt | shasum -a 256 -c -`
	guideUnpack         = `tar -xzf true-bdd_vX.Y.Z_<os>_<arch>.tar.gz true-bdd`
	guideOnPath         = `mkdir -p "$HOME/.local/bin" && mv true-bdd "$HOME/.local/bin/"`
	guideVersion        = `true-bdd version`
	guideGoInstall      = `go install github.com/ondatra-ai/true-bdd/services/true-bdd@vX.Y.Z`
	guideInit           = `true-bdd init`
	guideHarness        = `go get github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo@<commit>`
	guideHarnessVersion = `go list -m github.com/ondatra-ai/true-bdd/pkg`
	guideGitInit        = `git init`
	guideFirstRun       = `true-bdd build code --fix`
)

// guideCommands lists the guide's commands in the order README.md gives them.
func guideCommands() []string {
	return []string{guideVerify, guideUnpack, guideOnPath, guideVersion, guideGoInstall,
		guideInit, guideHarness, guideHarnessVersion, guideGitInit, guideFirstRun}
}

// fillGuide puts a release's version and commit, and this machine's platform, into a guide command.
func fillGuide(command, version, commit string) string {
	return strings.NewReplacer(versionPlaceholder, version, commitPlaceholder, commit,
		"<os>", runtime.GOOS, "<arch>", runtime.GOARCH).Replace(command)
}

// runGuide runs guide commands as a person types them, in dir; output, when set, receives
// their stdout and stderr. A failure carries the tail of what they printed.
func runGuide(dir string, env cli.Env, output io.Writer, commands ...string) error {
	var printed bytes.Buffer

	sink := io.Writer(&printed)
	if output != nil {
		sink = io.MultiWriter(&printed, output)
	}

	phase := spec.Phase{Name: "install guide", Dir: dir, Env: env, Budget: installBudget,
		Output: func(int) (cli.Sink, func()) { return cli.To(sink), func() {} }}

	err := phase.Run(commands)
	if err != nil {
		const tail = 2048

		text := printed.String()
		if len(text) > tail {
			text = text[len(text)-tail:]
		}

		return fmt.Errorf("following the install guide: %w: %s", err, strings.TrimSpace(text))
	}

	return nil
}
