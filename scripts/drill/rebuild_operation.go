package drill

import (
	"context"
	"fmt"
	"slices"

	"github.com/ondatra-ai/true-bdd/pkg/cli/truebdd"
)

func rebuildOperation(args []string) (func(context.Context, string, truebdd.Options) (int, error), error) {
	switch {
	case slices.Equal(args, []string{"build", "tests", "--fix"}):
		return truebdd.BuildTests, nil
	case slices.Equal(args, []string{"build", "code", "--fix"}):
		return truebdd.BuildCode, nil
	default:
		return nil, fmt.Errorf("%w: command must be build tests --fix or build code --fix", errRefused)
	}
}
