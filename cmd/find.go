package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/devsy-org/devsy-provider-dockerless/pkg/dockerless"
	"github.com/devsy-org/devsy-provider-dockerless/pkg/options"
	"github.com/devsy-org/devsy/pkg/log"
	"github.com/spf13/cobra"
)

// FindCmd holds the cmd flags.
type FindCmd struct{}

// NewFindCmd defines a command.
func NewFindCmd() *cobra.Command {
	cmd := &FindCmd{}
	findCmd := &cobra.Command{
		Use:   "find",
		Short: "Find a container",
		RunE: func(_ *cobra.Command, args []string) error {
			options, err := options.FromEnv()
			if err != nil {
				return err
			}

			return cmd.Run(context.Background(), options)
		},
	}

	return findCmd
}

// Run runs the command logic.
func (cmd *FindCmd) Run(ctx context.Context, options *options.Options) error {
	dockerlessProvider, err := dockerless.NewProvider(ctx, options)
	if err != nil {
		return err
	}

	containerDetails, err := dockerlessProvider.Find(ctx, options.DevContainerID)
	if err != nil {
		log.Error(err)
	}

	if containerDetails == nil {
		return nil
	}

	out, err := json.Marshal(containerDetails)
	if err != nil {
		return fmt.Errorf("error marshalling container details: %w", err)
	}

	_, err = os.Stdout.WriteString(string(out) + "\n")

	return err
}
