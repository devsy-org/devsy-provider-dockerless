package cmd

import (
	"os"
	"os/exec"

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
			cfg := log.Config{Verbosity: 1}
			if os.Getenv("DEVSY_DEBUG") == "true" {
				cfg.Debug = true
			}
			log.Init(cfg)

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
