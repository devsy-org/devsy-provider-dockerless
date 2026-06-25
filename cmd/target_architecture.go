package cmd

import (
	"context"
	"os"
	"runtime"

	"github.com/devsy-org/devsy-provider-dockerless/pkg/options"
	"github.com/devsy-org/log"
	"github.com/spf13/cobra"
)

// TargetArchitectureCmd holds the cmd flags.
type TargetArchitectureCmd struct{}

// NewTargetArchitectureCmd defines a command.
func NewTargetArchitectureCmd() *cobra.Command {
	cmd := &TargetArchitectureCmd{}
	targetArchitectureCmd := &cobra.Command{
		Use:   "target-architecture",
		Short: "TargetArchitecture a container",
		RunE: func(_ *cobra.Command, args []string) error {
			options, err := options.FromEnv()
			if err != nil {
				return err
			}

			return cmd.Run(context.Background(), options, log.Default)
		},
	}

	return targetArchitectureCmd
}

// Run runs the command logic.
func (cmd *TargetArchitectureCmd) Run(
	ctx context.Context,
	options *options.Options,
	log log.Logger,
) error {
	_, err := os.Stdout.WriteString(runtime.GOARCH + "\n")
	return err
}
