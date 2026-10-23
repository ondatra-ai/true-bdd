// Command drill runs the rebuild experiment through private iteration records.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/ondatra-ai/true-bdd/pkg/logging"
	"github.com/ondatra-ai/true-bdd/scripts/drill"
)

func main() {
	logging.Install(logging.Stderr, "", "drill")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	command := drill.Command()
	command.SetArgs(os.Args[1:])
	err := command.ExecuteContext(ctx)

	stop()

	if err != nil {
		slog.Error("Drill command failed", "error", err)
		os.Exit(1)
	}
}
