// main.go — entry point.
//
// It has ONE extra responsibility beyond calling cmd.Execute():
// detecting whether this process was spawned as the background updater child.
//
// When the user runs `mahin update`, the parent process spawns a child with:
//   mahin --internal-updater <execPath> <binaryURL> <checksumURL> <assetName> <newVersion>
//
// We intercept that flag HERE, before cobra sees os.Args, because cobra would
// reject an unknown flag and exit with an error.
package main

import (
	"os"

	"github.com/mahin/mahin-cli-v1/cmd"
	"github.com/mahin/mahin-cli-v1/updater"
)

func main() {
	// Check if this process is the background updater child.
	// The flag is always the first argument when the parent spawns us.
	if len(os.Args) > 1 && os.Args[1] == updater.InternalUpdaterFlag {
		// os.Args[2:] contains: execPath binaryURL checksumURL assetName newVersion
		updater.RunChild(os.Args[2:])
		return
	}

	// Normal CLI mode — hand off to cobra.
	cmd.Execute()
}