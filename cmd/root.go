package cmd

import (
	"os"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "mahin",
	Short: "Mahin is a custom self-updating CLI",
	Long:  `A custom command line interface that can say hello, check its version, and self-update from a Git repository.`,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}