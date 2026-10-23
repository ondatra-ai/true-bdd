package drill

import (
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/ondatra-ai/true-bdd/pkg/cli/git"
	"github.com/spf13/cobra"
)

// Command is the drill's public command-line interface.
func Command() *cobra.Command {
	command := &cobra.Command{Use: "drill", SilenceUsage: true, SilenceErrors: true}
	command.SetOut(io.Discard)
	command.SetErr(io.Discard)

	iteration := ""
	command.PersistentFlags().StringVar(&iteration, "iteration", "", "private iteration directory")
	command.AddCommand(startCommand(),
		cutCommand(&iteration),
		runCommand(&iteration),
		oracleCommand(&iteration),
		auditCommand(&iteration),
		stopCommand(&iteration),
		supervisorCommand(&iteration))

	return command
}

func startCommand() *cobra.Command {
	base := ""
	command := &cobra.Command{Use: "start --base <sha>",
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command,
			_ []string) error {
			if base == "" {
				return fmt.Errorf("%w: --base is required", errRefused)
			}

			root, err := git.TopLevel()
			if err != nil {
				return err
			}

			_, err = New(root).Start(command.Context(), base)

			return err
		}}
	command.Flags().StringVar(&base, "base", "", "iteration's base commit")

	return command
}

func cutCommand(iteration *string) *cobra.Command {
	return &cobra.Command{Use: "cut <a|b|c|d>",
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command,
			args []string) error {
			_, err := New("").Cut(command.Context(), *iteration, args[0])

			return err
		}}
}

func runCommand(iteration *string) *cobra.Command {
	const minimumArgs = 3

	return &cobra.Command{Use: "run <cut> <tag> -- <engine args>",
		Args: cobra.MinimumNArgs(minimumArgs),
		RunE: func(command *cobra.Command,
			args []string) error {
			const headerArgs = 2
			if command.ArgsLenAtDash() != headerArgs {
				return fmt.Errorf("%w: separate engine arguments with --", errRefused)
			}

			runner := New("")

			run, err := runner.Prepare(command.Context(), *iteration, args[0], args[1], args[2:])
			if err != nil {
				return err
			}

			return launchCommand(command, runner, *iteration, run.Tag)
		}}
}

func oracleCommand(iteration *string) *cobra.Command {
	return &cobra.Command{Use: "oracle <cut>",
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command,
			args []string) error {
			runner := New("")

			run, err := runner.PrepareOracle(command.Context(), *iteration, args[0])
			if err != nil {
				return err
			}

			return launchCommand(command, runner, *iteration, run.Tag)
		}}
}

func auditCommand(iteration *string) *cobra.Command {
	return &cobra.Command{Use: "audit <cut>",
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command,
			args []string) error {
			audit, err := New("").Audit(command.Context(), *iteration, args[0])
			slog.Info("Drill audit recorded", "iteration", *iteration, "cut", args[0], "summary", audit.String())

			return err
		}}
}

func stopCommand(iteration *string) *cobra.Command {
	return &cobra.Command{Use: "stop <tag>",
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command,
			args []string) error {
			return New("").Stop(command.Context(), *iteration, args[0])
		}}
}

func supervisorCommand(iteration *string) *cobra.Command {
	return &cobra.Command{Use: "supervise <tag>",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(command *cobra.Command,
			args []string) error {
			return New("").Supervise(command.Context(), *iteration, args[0])
		}}
}

func launchCommand(command *cobra.Command, runner Runner, iteration, tag string) error {
	executable, err := os.Executable()
	if err != nil {
		return runner.finish(iteration, tag, -1, fmt.Errorf("finding worker: %w", err))
	}

	err = runner.Launch(command.Context(), iteration, tag, executable)
	slog.Info("Drill worker launched", "iteration", iteration, "tag", tag, "error", err)

	return err
}
