package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"mahin-cli/internal/version"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show current version",
	Run: func(cmd *cobra.Command, args []string) {

		v := version.Get()

		fmt.Println("Current version:", v)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}