package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"mahin-cli/internal/version"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update version",
	Run: func(cmd *cobra.Command, args []string) {

		v := version.Update()

		fmt.Println("Updated version:", v)
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}