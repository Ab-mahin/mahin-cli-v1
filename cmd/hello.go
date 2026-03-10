// hello.go — implements the `mahin hello` command.
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mahin/mahin-cli-v1/version"
)

var helloCmd = &cobra.Command{
	Use:   "hello",
	Short: "Print a greeting",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("👋 Hello from mahin-cli-v1!")
		fmt.Printf("   Running version: %s\n", version.Short())
	},
}
