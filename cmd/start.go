package cmd

import (
	"context"

	"github.com/devsy-org/devsy-provider-dockerless/pkg/dockerless"
	"github.com/devsy-org/devsy-provider-dockerless/pkg/options"
	"github.com/spf13/cobra"
)

// StartCmd holds the cmd flags.
type StartCmd struct{}

// NewStartCmd defines a command.
func NewStartCmd() *cobra.Command {
	cmd := &StartCmd{}
	startCmd := &cobra.Command{
		Use:   "start",
		Short: "Start a container",
		RunE: func(_ *cobra.Command, args []string) error {
			options, err := options.FromEnv()
			if err != nil {
				return err
			}

			return cmd.Run(context.Background(), options)
		},
	}

	return startCmd
}

// Run runs the command logic.
func (cmd *StartCmd) Run(ctx context.Context, options *options.Options) error {
	dockerlessProvider, err := dockerless.NewProvider(ctx, options)
	if err != nil {
		return err
	}

	return dockerlessProvider.Start(ctx, options.DevContainerID)
}
