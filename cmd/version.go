// version.go — implements the `mahin version` command.
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mahin/mahin-cli-v1/version"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show current version, commit, and build date",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(version.Full())
	},
}