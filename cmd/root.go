package cmd

import (
	"errors"
	"os"
	"os/exec"

	"github.com/devsy-org/devsy/logging"
	"github.com/devsy-org/devsy/pkg/log"
	"github.com/spf13/cobra"
)

// NewRootCmd returns a new root command.
func NewRootCmd() *cobra.Command {
	dockerlessCmd := &cobra.Command{
		Use:           "devsy-provider-dockerless",
		Short:         "Dockerless Provider commands",
		SilenceErrors: true,
		SilenceUsage:  true,

		PersistentPreRunE: func(cobraCmd *cobra.Command, args []string) error {
			config, invalid := logging.ResolveProvider(
				os.Getenv("DEVSY_LOG_LEVEL"),
				os.Getenv("DEVSY_LOG_FORMAT"),
			)
			var configErr error
			if invalid.Level {
				configErr = errors.New("invalid DEVSY_LOG_LEVEL")
			} else if invalid.Format {
				configErr = errors.New("invalid DEVSY_LOG_FORMAT")
			}
			log.Init(
				log.Config{
					Level:        logging.LevelName(config.Level),
					DefaultLevel: log.LevelInfoName,
					Format:       string(config.Format),
				},
			)
			if configErr != nil {
				return configErr
			}

			return nil
		},
	}

	return dockerlessCmd
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	// build the root command
	rootCmd := BuildRoot()

	// execute command
	err := rootCmd.Execute()
	if err != nil {
		//nolint: errorlint
		exitErr, ok := err.(*exec.ExitError)
		if ok {
			if len(exitErr.Stderr) > 0 {
				log.Error(string(exitErr.Stderr))
			}

			os.Exit(exitErr.ExitCode())
		}
		log.Fatal(err)
	}
}

// BuildRoot creates a new root command from the provider.
func BuildRoot() *cobra.Command {
	rootCmd := NewRootCmd()

	rootCmd.AddCommand(NewEnterCmd())
	rootCmd.AddCommand(NewFindCmd())
	rootCmd.AddCommand(NewDeleteCmd())
	rootCmd.AddCommand(NewStartCmd())
	rootCmd.AddCommand(NewRunCmd())
	rootCmd.AddCommand(NewCommandCmd())
	rootCmd.AddCommand(NewStopCmd())
	rootCmd.AddCommand(NewTargetArchitectureCmd())
	return rootCmd
}
