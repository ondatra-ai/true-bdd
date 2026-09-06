package steps

import (
	"context"
	"fmt"
	"os"

	"github.com/ondatra-ai/true-bdd/pkg/cli"
)

// relay is one application process a scenario started for itself: where it
// answers, and the process serving it, so the scenario can point its clauses
// at it and give it back when it ends.
type relay struct {
	// BaseURL is where this relay answers.
	BaseURL string

	bundle  string
	port    int
	env     []string
	process *cli.Process
}

// startScenarioRelay starts another copy of the application under test on a
// free loopback port, under env on top of what every relay needs, and hands
// its lifetime to the scenario: one left running holds its port.
func startScenarioRelay(state *State, env ...string) (*relay, error) {
	ctx := context.Background()

	port, err := freePort(ctx)
	if err != nil {
		return nil, err
	}

	process, err := startNodeServer(ctx, state.Harness.Bundle, port, env)
	if err != nil {
		return nil, err
	}

	started := &relay{
		BaseURL: fmt.Sprintf("http://127.0.0.1:%d", port),
		bundle:  state.Harness.Bundle,
		port:    port,
		env:     env,
		process: process,
	}

	state.T.Cleanup(started.stop)

	return started, waitForRelay(ctx, started.BaseURL)
}

// stop ends the relay. Kill rather than a polite signal: the scenario is over,
// and a relay left running holds its port.
func (r *relay) stop() {
	_ = r.process.Signal(os.Kill)
	_, _ = r.process.Wait()
}

// restart kills this relay and starts another on the same port under the same
// settings: the process dies, the registry it served does not.
func (r *relay) restart(ctx context.Context) error {
	r.stop()

	process, err := startNodeServer(ctx, r.bundle, r.port, r.env)
	if err != nil {
		return err
	}

	r.process = process

	return waitForRelay(ctx, r.BaseURL)
}
