package cmd

import (
	"context"
	"os"

	"github.com/devsy-org/devsy-provider-dockerless/pkg/dockerless"
	"github.com/devsy-org/devsy-provider-dockerless/pkg/options"
	"github.com/devsy-org/log"
	"github.com/spf13/cobra"
)

// CommandCmd holds the cmd flags.
type CommandCmd struct{}

// NewCommandCmd defines a command.
func NewCommandCmd() *cobra.Command {
	cmd := &CommandCmd{}
	commandCmd := &cobra.Command{
		Use:   "command",
		Short: "Command a container",
		RunE: func(_ *cobra.Command, args []string) error {
			options, err := options.FromEnv()
			if err != nil {
				return err
			}

			return cmd.Run(context.Background(), options, log.Default)
		},
	}

	return commandCmd
}

// Run runs the command logic.
func (cmd *CommandCmd) Run(ctx context.Context, options *options.Options, log log.Logger) error {
	dockerlessProvider, err := dockerless.NewProvider(ctx, options, log)
	if err != nil {
		return err
	}

	return dockerlessProvider.ExecuteCommand(ctx, dockerless.ExecOptions{
		WorkspaceID: options.DevContainerID,
		User:        os.Getenv("DEVCONTAINER_USER"),
		Command:     os.Getenv("DEVCONTAINER_COMMAND"),
		Stdin:       os.Stdin,
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
	})
}
