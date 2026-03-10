// download.go — handles downloading files from the internet and verifying them.
// Only job: fetch a URL to disk, then confirm its SHA-256 matches the expected hash.
package updater

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// downloadFile streams a remote URL directly to a local file.
// Streaming means the entire binary is never loaded into memory at once.
func downloadFile(url, destPath string) error {
	client := &http.Client{Timeout: 5 * time.Minute} // generous timeout for large binaries

	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned HTTP %d", resp.StatusCode)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("cannot create file: %w", err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

// verifyChecksum reads the expected SHA-256 hash from checksumFile and
// compares it against the actual SHA-256 of binaryFile.
//
// Checksum file format (standard sha256sum output):
//
//	<hex-hash>  <filename>
//
// If they do not match the download is considered corrupt or tampered with.
func verifyChecksum(binaryFile, checksumFile string) error {
	// Read expected hash from the .sha256 file.
	data, err := os.ReadFile(checksumFile)
	if err != nil {
		return fmt.Errorf("cannot read checksum file: %w", err)
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return fmt.Errorf("checksum file is empty")
	}
	expected := strings.ToLower(fields[0])

	// Compute actual SHA-256 of the downloaded binary.
	f, err := os.Open(binaryFile)
	if err != nil {
		return fmt.Errorf("cannot open binary for hashing: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("hashing failed: %w", err)
	}
	actual := hex.EncodeToString(h.Sum(nil))

	if actual != expected {
		return fmt.Errorf("checksum mismatch\n  expected: %s\n  actual:   %s", expected, actual)
	}
	return nil
}