// process.go — builds the OS-appropriate command to spawn the child updater.
// Only job: handle the difference between Unix and Windows process spawning.
//
// On Unix  → exec.Command is enough; child outlives parent naturally.
// On Windows → SysProcAttr with CreationFlags is needed to detach the child
//              so it is not killed when the parent console closes.
package updater

import (
	"os/exec"
	"runtime"
)

// buildCommand creates an *exec.Cmd configured to run detached from the
// parent process on both Unix and Windows.
func buildCommand(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	setSysProcAttr(cmd) // platform-specific, defined in process_unix.go / process_windows.go
	return cmd
}

// currentOS is a helper used in tests.
func currentOS() string {
	return runtime.GOOS
}