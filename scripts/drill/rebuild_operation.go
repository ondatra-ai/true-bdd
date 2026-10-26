package drill

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/cli/truebdd"
)

func rebuildOperation(args []string) (func(context.Context, string, truebdd.Options) (int, error), error) {
	const baseArgs = 3
	if len(args) < baseArgs {
		return nil, fmt.Errorf("%w: command must be build tests --fix or build code --fix", errRefused)
	}

	models, err := rebuildModels(args[baseArgs:])
	if err != nil {
		return nil, err
	}

	var operation func(context.Context, string, truebdd.Options) (int, error)

	switch {
	case slices.Equal(args[:baseArgs], []string{"build", "tests", "--fix"}):
		operation = truebdd.BuildTests
	case slices.Equal(args[:baseArgs], []string{"build", "code", "--fix"}):
		operation = truebdd.BuildCode
	default:
		return nil, fmt.Errorf("%w: command must be build tests --fix or build code --fix", errRefused)
	}

	return func(ctx context.Context, binary string, opt truebdd.Options) (int, error) {
		opt.ModelOverrides = models

		return operation(ctx, binary, opt)
	}, nil
}

func rebuildModels(args []string) ([]truebdd.ModelOverride, error) {
	const flagArgs = 2

	models := make([]truebdd.ModelOverride, 0, len(args)/flagArgs)
	for len(args) > 0 {
		if len(args) < flagArgs || args[0] != "--model" || args[1] == "" || strings.HasPrefix(args[1], "-") {
			return nil, fmt.Errorf("%w: only repeated --model <tier>=<cli>:<model> may follow --fix", errRefused)
		}

		models = append(models, truebdd.ModelOverride(args[1]))
		args = args[flagArgs:]
	}

	return models, nil
}
