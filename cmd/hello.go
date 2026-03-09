package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var helloCmd = &cobra.Command{
	Use:   "hello",
	Short: "Print hello message",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("hi")
	},
}

func init() {
	rootCmd.AddCommand(helloCmd)
}