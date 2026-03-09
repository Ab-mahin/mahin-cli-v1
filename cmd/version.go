package cmd

import (
	"fmt"
	"mahin-cli-v1/config"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of mahin",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("mahin version %s\n", config.AppVersion)
	},
}