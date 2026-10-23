// Package drill launches the drill worker binary for repository tooling.
package drill

import (
	"context"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

// StartSupervisor detaches a drill worker without inherited terminal descriptors.
func StartSupervisor(ctx context.Context, binary, iteration, tag, marker string) (int, error) {
	process,
		err := shell.Start(context.WithoutCancel(ctx),
		[]string{binary,
			"supervise",
			"--iteration",
			iteration,
			tag},
		shell.Options{
			Session: true, Stdin: strings.NewReader(""), Output: shell.Discard(),
			Env: shell.Inherit().Strip("TRUE_BDD_DRILL_RUN").Set("TRUE_BDD_DRILL_WORKER=" + marker),
		})
	if err != nil {
		return 0, err
	}

	go func() { _, _ = process.Wait() }()

	return process.Pid(), nil
}
