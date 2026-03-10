package updater

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	maxBinarySize   = 200 * 1024 * 1024 // 200MB safety limit
	downloadTimeout = 5 * time.Minute
)

// ─────────────────────────────────────────────────────────
// URL validation
// ─────────────────────────────────────────────────────────

func validateURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	if u.Scheme != "https" {
		return fmt.Errorf("insecure URL scheme: %s (https required)", u.Scheme)
	}

	return nil
}

//
// ─────────────────────────────────────────────────────────
// Secure file download
// ─────────────────────────────────────────────────────────
//

func downloadFile(fileURL, destPath string) error {

	if err := validateURL(fileURL); err != nil {
		return err
	}

	client := &http.Client{
		Timeout: downloadTimeout,
	}

	resp, err := client.Get(fileURL)
	if err != nil {
		return fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned HTTP %d", resp.StatusCode)
	}

	if resp.ContentLength > maxBinarySize {
		return fmt.Errorf("download too large (%d bytes)", resp.ContentLength)
	}

	tmp := destPath + ".tmp"

	out, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("cannot create file: %w", err)
	}
	defer out.Close()

	written, err := io.Copy(out, resp.Body)
	if err != nil {
		os.Remove(tmp)
		return fmt.Errorf("download failed: %w", err)
	}

	if written == 0 {
		os.Remove(tmp)
		return fmt.Errorf("download produced empty file")
	}

	if err := os.Rename(tmp, destPath); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("file rename failed: %w", err)
	}

	return nil
}

//
// ─────────────────────────────────────────────────────────
// Checksum verification
// ─────────────────────────────────────────────────────────
//

func verifyChecksum(binaryFile, checksumFile string) error {

	data, err := os.ReadFile(checksumFile)
	if err != nil {
		return fmt.Errorf("cannot read checksum file: %w", err)
	}

	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return fmt.Errorf("checksum file empty")
	}

	expected := strings.ToLower(fields[0])

	if len(expected) != 64 {
		return fmt.Errorf("invalid SHA256 checksum length")
	}

	file, err := os.Open(binaryFile)
	if err != nil {
		return fmt.Errorf("cannot open binary: %w", err)
	}
	defer file.Close()

	hash := sha256.New()

	if _, err := io.Copy(hash, file); err != nil {
		return fmt.Errorf("hash computation failed: %w", err)
	}

	actual := hex.EncodeToString(hash.Sum(nil))

	if actual != expected {
		return fmt.Errorf(
			"checksum mismatch\nexpected: %s\nactual:   %s",
			expected,
			actual,
		)
	}

	return nil
}

//
// ─────────────────────────────────────────────────────────
// Binary verification
// ─────────────────────────────────────────────────────────
//

func verifyBinary(binaryPath, expectedVersion string) error {

	cmd := exec.Command(binaryPath, "version")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("binary execution failed: %w", err)
	}

	out := strings.TrimSpace(string(output))

	if !strings.Contains(out, expectedVersion) {
		return fmt.Errorf(
			"version mismatch\nexpected: %s\nbinary reported: %s",
			expectedVersion,
			out,
		)
	}

	return nil
}

//
// ─────────────────────────────────────────────────────────
// Safe executable replacement
// ─────────────────────────────────────────────────────────
//

func replaceExecutable(execPath, newBinary string) error {

	backup := execPath + ".old"

	// Retry loop for Windows file locking
	for i := 0; i < 5; i++ {

		err := os.Rename(execPath, backup)
		if err != nil {
			time.Sleep(300 * time.Millisecond)
			continue
		}

		err = os.Rename(newBinary, execPath)
		if err != nil {

			// rollback
			_ = os.Rename(backup, execPath)

			return fmt.Errorf("install failed, rollback completed: %w", err)
		}

		os.Remove(backup)

		return nil
	}

	return fmt.Errorf("unable to replace executable after retries")
}

//
// ─────────────────────────────────────────────────────────
// Child update logic
// ─────────────────────────────────────────────────────────
//

func runChildUpdate(execPath, binaryURL, checksumURL, assetName, newVersion string) error {

	baseDir := filepath.Dir(execPath)

	tmpDir, err := os.MkdirTemp(baseDir, "mahin-update-*")
	if err != nil {
		return fmt.Errorf("cannot create temp directory: %w", err)
	}

	defer func() {
		fmt.Println("🧹 Cleaning temporary files...")
		os.RemoveAll(tmpDir)
	}()

	fmt.Printf("📁 Temp workspace: %s\n", tmpDir)

	newBinaryPath := filepath.Join(tmpDir, assetName)

	fmt.Println("⬇ Downloading binary...")
	if err := downloadFile(binaryURL, newBinaryPath); err != nil {
		return fmt.Errorf("binary download failed: %w", err)
	}

	if checksumURL != "" {

		fmt.Println("🔐 Verifying checksum...")

		checksumPath := filepath.Join(tmpDir, assetName+".sha256")

		if err := downloadFile(checksumURL, checksumPath); err != nil {
			return fmt.Errorf("checksum download failed: %w", err)
		}

		if err := verifyChecksum(newBinaryPath, checksumPath); err != nil {
			return fmt.Errorf("checksum verification failed: %w", err)
		}

		fmt.Println("✅ Checksum OK")
	}

	info, err := os.Stat(newBinaryPath)
	if err != nil {
		return fmt.Errorf("cannot stat binary: %w", err)
	}

	if info.Size() == 0 {
		return fmt.Errorf("binary file empty")
	}

	if err := os.Chmod(newBinaryPath, 0755); err != nil {
		return fmt.Errorf("chmod failed: %w", err)
	}

	fmt.Println("🔬 Verifying binary...")
	if err := verifyBinary(newBinaryPath, newVersion); err != nil {
		return fmt.Errorf("binary verification failed: %w", err)
	}

	fmt.Println("🔄 Installing update...")
	if err := replaceExecutable(execPath, newBinaryPath); err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}

	fmt.Printf("\n🎉 Updated successfully to %s\n", newVersion)

	return nil
}