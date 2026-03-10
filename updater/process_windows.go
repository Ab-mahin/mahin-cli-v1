//go:build windows

// process_windows.go — Windows implementation of setSysProcAttr.
// On Windows, CREATE_NEW_PROCESS_GROUP detaches the child from the parent's
// console so it is not killed when the parent exits.
package updater

import (
	"os/exec"
	"syscall"
)

func setSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}