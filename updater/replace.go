// replace.go — atomically replaces the running executable with a new binary.
// Only job: swap old binary → new binary safely, with rollback on failure.
//
// WHY THIS IS NON-TRIVIAL
// A running process cannot safely overwrite itself — especially on Windows
// where the OS file-locks the executable. The solution used here:
//   1. Rename old → old.bak   (releases the filename)
//   2. Copy  new → old path   (works across devices/filesystems)
//   3. chmod 0755             (make it executable)
//   4. Remove old.bak         (cleanup)
//
// If step 2 or 3 fails, old.bak is renamed back so the user is never left
// with a broken installation.
package updater

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// verifyBinary runs `<newBinary> version` and confirms the output contains
// the expected version tag. This proves the binary is executable and correct
// BEFORE we replace anything on disk.
func verifyBinary(binaryPath, expectedVersion string) error {
	out, err := exec.Command(binaryPath, "version").Output()
	if err != nil {
		return fmt.Errorf("new binary failed to run: %w", err)
	}
	if !strings.Contains(string(out), strings.TrimPrefix(expectedVersion, "v")) {
		return fmt.Errorf("version mismatch in output: %q", strings.TrimSpace(string(out)))
	}
	return nil
}

// replaceExecutable atomically swaps oldPath with newPath.
// See the package-level comment for the full strategy.
func replaceExecutable(oldPath, newPath string) error {
	backupPath := oldPath + ".bak"

	// Step 1: back up the old binary.
	if err := os.Rename(oldPath, backupPath); err != nil {
		return fmt.Errorf("cannot back up old binary: %w", err)
	}

	// Step 2: copy new binary into the old path.
	if err := copyFile(newPath, oldPath); err != nil {
		_ = os.Rename(backupPath, oldPath) // rollback
		return fmt.Errorf("cannot write new binary: %w", err)
	}

	// Step 3: make it executable.
	if err := os.Chmod(oldPath, 0755); err != nil {
		_ = os.Rename(backupPath, oldPath) // rollback
		return fmt.Errorf("cannot chmod new binary: %w", err)
	}

	// Step 4: remove backup.
	os.Remove(backupPath)
	return nil
}

// copyFile copies the contents of src into dst, creating dst if needed.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}