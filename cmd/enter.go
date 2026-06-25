package cmd

import (
	"context"

	"github.com/devsy-org/devsy-provider-dockerless/pkg/dockerless"
	"github.com/devsy-org/devsy-provider-dockerless/pkg/options"
	"github.com/spf13/cobra"
)

// EnterCmd holds the cmd flags.
type EnterCmd struct{}

// NewEnterCmd defines a command.
func NewEnterCmd() *cobra.Command {
	cmd := &EnterCmd{}
	enterCmd := &cobra.Command{
		Use:   "enter",
		Short: "Enter a container",
		RunE: func(_ *cobra.Command, args []string) error {
			options, err := options.FromEnv()
			if err != nil {
				return err
			}

			return cmd.Run(context.Background(), options)
		},
	}

	return enterCmd
}

// Run runs the command logic.
func (cmd *EnterCmd) Run(ctx context.Context, options *options.Options) error {
	dockerlessProvider, err := dockerless.NewProvider(ctx, options)
	if err != nil {
		return err
	}

	return dockerlessProvider.Enter(ctx, options.DevContainerID)
}
