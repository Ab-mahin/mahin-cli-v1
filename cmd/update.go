// update.go — implements the `mahin update` command.
// This file only defines the cobra command and its description.
// All update logic lives in the updater/ package.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/mahin/mahin-cli-v1/updater"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Check GitHub for a newer release and self-update",
	Long: `update queries the GitHub Releases API for the latest release.

If a newer version is found:
  1. Your OS and CPU architecture are detected automatically
  2. The correct binary for your platform is downloaded
  3. The SHA-256 checksum is verified
  4. A child process is spawned to do the replacement (so the running
     binary is not locked when it gets overwritten)
  5. Temporary files are cleaned up`,
	Run: func(cmd *cobra.Command, args []string) {
    result, err := updater.Run()
    if err != nil {
        fmt.Fprintf(os.Stderr, "❌ Update failed: %v\n", err)
        os.Exit(1)
    }

    if result.AlreadyLatest {
        fmt.Println("✔ Already running the latest version")
        return
    }

    fmt.Printf("\n✨ Updated %s → %s\n", result.PreviousVersion, result.UpdatedTo)
},
}