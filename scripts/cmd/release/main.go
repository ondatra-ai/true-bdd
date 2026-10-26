// Command release builds and rehearses this repository's release artifacts.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/ondatra-ai/true-bdd/pkg/logging"
	"github.com/ondatra-ai/true-bdd/scripts/release"
)

func main() {
	logging.Install(logging.Stderr, "", "release")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	command := release.Command()
	command.SetArgs(os.Args[1:])
	err := command.ExecuteContext(ctx)

	stop()

	if err != nil {
		slog.Error("Release command failed", "error", err)
		os.Exit(1)
	}
}
