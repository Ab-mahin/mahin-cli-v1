// platform.go — detects the current OS and CPU architecture at runtime.
// Only job: figure out which release asset to download for this machine.
package updater

import (
	"fmt"
	"runtime"

	"github.com/mahin/mahin-cli-v1/config"
)

// platform holds the OS and architecture of the machine running this binary.
type platform struct {
	OS   string // "linux" | "darwin" | "windows"
	Arch string // "amd64" | "arm64" | "386"
}

// detect reads runtime.GOOS and runtime.GOARCH — both are set automatically
// by the Go runtime for the machine that is actually running the binary.
// No user input or environment variables are needed.
func detect() platform {
	os := runtime.GOOS   // e.g. "linux", "darwin", "windows"
	arch := runtime.GOARCH // e.g. "amd64", "arm64"

	// Normalise uncommon aliases that some systems report.
	switch arch {
	case "x86_64":
		arch = "amd64"
	case "aarch64":
		arch = "arm64"
	}

	return platform{OS: os, Arch: arch}
}

// binaryAssetName returns the expected filename of the release binary for
// this platform, e.g.:
//
//	linux  / amd64 → "mahin-linux-amd64"
//	darwin / arm64 → "mahin-darwin-arm64"
//	windows/ amd64 → "mahin-windows-amd64.exe"
func (p platform) binaryAssetName() string {
	name := fmt.Sprintf("%s-%s-%s", config.BinaryName, p.OS, p.Arch)
	if p.OS == "windows" {
		name += ".exe"
	}
	return name
}

// checksumAssetName returns the expected filename of the SHA-256 checksum
// file for this platform, e.g. "mahin-linux-amd64.sha256".
func (p platform) checksumAssetName() string {
	return p.binaryAssetName() + ".sha256"
}