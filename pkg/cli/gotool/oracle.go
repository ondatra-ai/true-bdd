package gotool

import (
	"context"
	"io"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/cli/git"
	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

// OracleOptions supervises build and replay with the same run identity.
type OracleOptions struct {
	Directory string
	Marker    string
	Output    io.Writer
	Limit     time.Duration
	Grace     time.Duration
}

// BuildOracle compiles the engine a cut produced.
func BuildOracle(ctx context.Context, binary string, opt OracleOptions) (int, error) {
	return oracleCommand(ctx, []string{buildVerb, "-trimpath", "-o", binary, "./services/bdd-cli"}, opt)
}

// ReplayOracle exercises the original suite against the rebuilt binary.
func ReplayOracle(ctx context.Context, binary string, opt OracleOptions) (int, error) {
	return oracleCommand(ctx,
		[]string{"test",
			"-tags",
			"bdd",
			jsonOutputFlag,
			"-count=1",
			"-timeout=40m",
			"./tests/bdd-cli/",
			"-mode=services:replay,tests:replay",
			"-engine-bin=" + binary},
		opt)
}

func oracleCommand(ctx context.Context, args []string, opt OracleOptions) (int, error) {
	result,
		err := shell.Watch(ctx,
		append([]string{Bin},
			args...),
		shell.Options{Dir: opt.Directory,
			Session: true,
			Env: git.Environment().Strip("OLDPWD").Set("PWD="+opt.Directory,
				"TRUE_BDD_DRILL_RUN="+opt.Marker),
			Output: shell.To(opt.Output)},
		shell.Watchdog{Limit: opt.Limit,
			Grace: opt.Grace})

	return result.Code, err
}
