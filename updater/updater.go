// updater.go — orchestrates the full self-update flow.
//
// This file is the conductor. It calls the other files in the right order:
//   github.go   → fetch latest release info
//   platform.go → detect OS/arch, build asset name
//   download.go → download binary + verify checksum
//   replace.go  → verify binary + swap executable on disk
//
// THE SPAWNED PROCESS PATTERN
// A running binary cannot safely replace itself (Windows locks it; even on
// Linux it is unsafe). The solution is:
//
//   Parent process (mahin update)
//     │
//     ├─ does all pre-checks (version compare, platform detect, asset lookup)
//     ├─ spawns a CHILD process: mahin --internal-updater <args>
//     └─ exits immediately
//
//   Child process (mahin --internal-updater ...)
//     │  (parent is now gone, file lock is released)
//     ├─ waits briefly to ensure parent has fully exited
//     ├─ creates temp workspace
//     ├─ downloads binary
//     ├─ verifies checksum
//     ├─ verifies binary
//     ├─ replaces executable
//     └─ cleans up temp files
//
// The child is the same binary invoked with a hidden flag so no extra
// executable needs to be bundled or downloaded.
package updater

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mahin/mahin-cli-v1/version"
)

// InternalUpdaterFlag is the hidden flag the parent passes to the child
// process. It is intentionally unexported from cobra so users never see it.
const InternalUpdaterFlag = "--internal-updater"

// childArgs are the values the parent encodes for the child.
type childArgs struct {
	execPath    string // absolute path of the binary to replace
	binaryURL   string // download URL of the new binary
	checksumURL string // download URL of the .sha256 file (may be empty)
	assetName   string // filename of the asset e.g. "mahin-linux-amd64"
	newVersion  string // expected version tag e.g. "v1.2.0"
}

// ─────────────────────────────────────────────────────────────────────────────
// PARENT — called by `cmd/update.go` when the user runs `mahin update`
// ─────────────────────────────────────────────────────────────────────────────

// Result is returned to cmd/update.go to print the final message.
type Result struct {
	AlreadyLatest   bool
	PreviousVersion string
	UpdatedTo       string
}

// Run is the entry point for the parent process.
// It does all the checks and then hands off to the child process.
func Run() (*Result, error) {
	// ── Step 1: get the absolute path of the running executable ──────────────
	execPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("cannot determine executable path: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath) // follow symlinks to real file
	if err != nil {
		return nil, fmt.Errorf("cannot resolve symlinks: %w", err)
	}
	fmt.Printf("📍 Current binary  : %s\n", execPath)

	// ── Step 2: read the current version baked into this binary ──────────────
	currentVersion := version.Short() // e.g. "v1.0.0"
	fmt.Printf("📦 Current version : %s\n", currentVersion)

	currentSemver, err := parseSemver(currentVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid current version %q: %w", currentVersion, err)
	}

	// ── Step 3: query the GitHub Releases API ────────────────────────────────
	fmt.Println("🔍 Checking for updates...")
	rel, err := fetchLatestRelease()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch release info: %w", err)
	}
	fmt.Printf("🏷️  Latest release  : %s\n", rel.TagName)

	// ── Step 4: compare versions ─────────────────────────────────────────────
	latestSemver, err := parseSemver(rel.TagName)
	if err != nil {
		return nil, fmt.Errorf("invalid remote version %q: %w", rel.TagName, err)
	}
	if !isNewer(currentSemver, latestSemver) {
		fmt.Println("✅ Already up to date!")
		return &Result{AlreadyLatest: true, PreviousVersion: currentVersion}, nil
	}
	fmt.Printf("🆕 New version      : %s → %s\n", currentVersion, rel.TagName)

	// ── Step 5: detect platform and find the right asset ─────────────────────
	plat := detect()
	fmt.Printf("🖥️  Platform         : %s/%s\n", plat.OS, plat.Arch)

	binaryURL, checksumURL, err := findAssetURLs(
		rel.Assets,
		plat.binaryAssetName(),
		plat.checksumAssetName(),
	)
	if err != nil {
		return nil, err
	}

	// ── Step 6: launch the updater child process ──────────────────────────────
	// We pass all the information the child needs as CLI arguments so it can
	// work independently after the parent exits.
	fmt.Println("🚀 Launching updater process...")
	args := childArgs{
		execPath:    execPath,
		binaryURL:   binaryURL,
		checksumURL: checksumURL,
		assetName:   plat.binaryAssetName(),
		newVersion:  rel.TagName,
	}
	if err := spawnChild(args); err != nil {
		return nil, fmt.Errorf("failed to launch updater process: %w", err)
	}

	// ── Parent exits here ─────────────────────────────────────────────────────
	// The child is now running. Exiting releases any OS file lock on the
	// current executable so the child can safely replace it.
	fmt.Println("⏳ Updater is running in the background...")
	os.Exit(0)

	// unreachable — os.Exit above always terminates
	return &Result{PreviousVersion: currentVersion, UpdatedTo: rel.TagName}, nil
}

// spawnChild launches a new process (same binary, hidden flag) that will
// carry out the actual download and replacement after the parent exits.
func spawnChild(args childArgs) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}

	// Pass all needed values as positional arguments after the flag.
	// Format: mahin --internal-updater <execPath> <binaryURL> <checksumURL> <assetName> <newVersion>
	cmd := buildCommand(self,
		InternalUpdaterFlag,
		args.execPath,
		args.binaryURL,
		args.checksumURL,
		args.assetName,
		args.newVersion,
	)

	// Start the child in a detached state so it outlives the parent.
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Start() // Start (not Run) — parent does NOT wait
}

// ─────────────────────────────────────────────────────────────────────────────
// CHILD — called by main.go when it detects --internal-updater
// ─────────────────────────────────────────────────────────────────────────────

// RunChild is the entry point for the child (updater) process.
// It is called from main.go when os.Args contains InternalUpdaterFlag.
//
// Expected args (after the flag):
//
//	[0] execPath    — path of the binary to replace
//	[1] binaryURL   — download URL
//	[2] checksumURL — checksum URL (empty string if none)
//	[3] assetName   — filename used when saving the download
//	[4] newVersion  — expected version tag
func RunChild(args []string) {
	if len(args) < 5 {
		fmt.Fprintln(os.Stderr, "❌ Internal updater: wrong number of arguments")
		os.Exit(1)
	}

	execPath    := args[0]
	binaryURL   := args[1]
	checksumURL := args[2]
	assetName   := args[3]
	newVersion  := args[4]

	// Wait for the parent to fully exit and release its file lock.
	fmt.Println("⏳ Waiting for parent process to exit...")
	time.Sleep(500 * time.Millisecond)

	if err := runChildUpdate(execPath, binaryURL, checksumURL, assetName, newVersion); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Update failed: %v\n", err)
		os.Exit(1)
	}
}

// runChildUpdate carries out all the download/verify/replace steps.
func runChildUpdate(execPath, binaryURL, checksumURL, assetName, newVersion string) error {
	// ── Step 7: create a temporary workspace ─────────────────────────────────
	tmpDir, err := os.MkdirTemp("", "mahin-update-*")
	if err != nil {
		return fmt.Errorf("cannot create temp dir: %w", err)
	}
	defer func() {
		// ── Step 12: clean up temp files ─────────────────────────────────────
		fmt.Println("🧹 Cleaning up temp files...")
		os.RemoveAll(tmpDir)
	}()
	fmt.Printf("📁 Temp workspace   : %s\n", tmpDir)

	// ── Step 8: download the new binary ──────────────────────────────────────
	newBinaryPath := filepath.Join(tmpDir, assetName)
	fmt.Printf("⬇️  Downloading %s...\n", assetName)
	if err := downloadFile(binaryURL, newBinaryPath); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	// ── Step 9: verify checksum ───────────────────────────────────────────────
	if checksumURL != "" {
		fmt.Println("🔐 Verifying checksum...")
		checksumPath := filepath.Join(tmpDir, assetName+".sha256")
		if err := downloadFile(checksumURL, checksumPath); err != nil {
			return fmt.Errorf("checksum download failed: %w", err)
		}
		if err := verifyChecksum(newBinaryPath, checksumPath); err != nil {
			return fmt.Errorf("checksum mismatch: %w", err)
		}
		fmt.Println("✅ Checksum verified.")
	} else {
		fmt.Println("⚠️  No checksum file — skipping verification.")
	}

	// Make executable before verification run.
	if err := os.Chmod(newBinaryPath, 0755); err != nil {
		return fmt.Errorf("cannot chmod: %w", err)
	}

	// ── Step 10: verify the new binary runs and reports the right version ─────
	fmt.Println("🔬 Verifying new binary...")
	if err := verifyBinary(newBinaryPath, newVersion); err != nil {
		return fmt.Errorf("binary verification failed: %w", err)
	}

	// ── Step 11: replace the existing executable ──────────────────────────────
	fmt.Println("🔄 Replacing executable...")
	if err := replaceExecutable(execPath, newBinaryPath); err != nil {
		return fmt.Errorf("replace failed: %w", err)
	}

	// ── Done ──────────────────────────────────────────────────────────────────
	fmt.Printf("\n🎉 Updated to %s — please re-run your command.\n", newVersion)
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Semver helpers (kept here to avoid a separate file for 3 small functions)
// ─────────────────────────────────────────────────────────────────────────────

type semver struct{ Major, Minor, Patch int }

func parseSemver(v string) (semver, error) {
	v = strings.TrimPrefix(v, "v")
	var s semver
	_, err := fmt.Sscanf(v, "%d.%d.%d", &s.Major, &s.Minor, &s.Patch)
	return s, err
}

func isNewer(current, candidate semver) bool {
	if candidate.Major != current.Major {
		return candidate.Major > current.Major
	}
	if candidate.Minor != current.Minor {
		return candidate.Minor > current.Minor
	}
	return candidate.Patch > current.Patch
}