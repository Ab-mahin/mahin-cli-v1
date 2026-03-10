//go:build !windows

// process_unix.go — Unix implementation of setSysProcAttr.
// On Linux and macOS a child process already outlives its parent by default,
// so no special flags are needed.
package updater

import "os/exec"

func setSysProcAttr(cmd *exec.Cmd) {
	// Nothing extra needed on Unix — the child survives parent exit naturally.
}