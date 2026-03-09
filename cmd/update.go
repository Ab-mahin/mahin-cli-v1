package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"mahin-cli-v1/config" // Make sure this matches your module name!

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(updateCmd)
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Downloads latest code, compiles, and updates the CLI",
	Run: func(cmd *cobra.Command, args []string) {
		// 1. Remember the current version before doing anything
		oldVersion := config.AppVersion

		fmt.Println("Starting update process...")

		// 👇 CHANGE THIS to your actual GitHub repository URL
		repoURL := "https://github.com/Ab-mahin/mahin-cli-v1"

		tempDir, err := os.MkdirTemp("", "mahin-update-*")
		if err != nil {
			fmt.Printf("❌ Failed to create temp directory: %v\n", err)
			return
		}
		defer os.RemoveAll(tempDir)

		fmt.Println("\n[1/4] Downloading latest code from GitHub...")
		gitCmd := exec.Command("git", "clone", repoURL, tempDir)
		if err := gitCmd.Run(); err != nil {
			fmt.Printf("❌ Failed to clone repository: %v\n", err)
			return
		}
		fmt.Println("✅ Download complete.")

		newExeName := "mahin_new"
		fmt.Println("\n[2/4] Compiling new binary...")
		buildCmd := exec.Command("go", "build", "-o", newExeName, ".")
		buildCmd.Dir = tempDir
		if err := buildCmd.Run(); err != nil {
			fmt.Printf("❌ Build failed: %v\n", err)
			return
		}
		fmt.Println("✅ Compilation complete.")

		fmt.Println("\n[3/4] Deploying new binary...")
		currentExePath, err := os.Executable()
		if err != nil {
			fmt.Printf("❌ Could not find current executable path: %v\n", err)
			return
		}

		currentDir := filepath.Dir(currentExePath)
		oldExePath := filepath.Join(currentDir, "mahin.old")
		newExeBuiltPath := filepath.Join(tempDir, newExeName)

		os.Remove(oldExePath)

		if err := os.Rename(currentExePath, oldExePath); err != nil {
			fmt.Printf("❌ Failed to rename current binary: %v\n", err)
			return
		}

		if err := os.Rename(newExeBuiltPath, currentExePath); err != nil {
			os.Rename(oldExePath, currentExePath)
			fmt.Printf("❌ Failed to deploy new binary: %v\n", err)
			return
		}

		os.Chmod(currentExePath, 0755)
		fmt.Println("✅ Deploy complete.")

		fmt.Println("\n[4/4] Cleanup")
		fmt.Println("-> Temp source files removed.")
		fmt.Println("-> Previous binary kept as mahin.old")

		// --- PRINT THE VERSION SUMMARY ---

		// Run the newly installed binary to ask it what its version is
		newVerCmd := exec.Command(currentExePath, "version")
		newVerOut, _ := newVerCmd.Output()

		// Clean up the output text
		newVersionStr := strings.TrimSpace(string(newVerOut))

		fmt.Println("\nOK All done!\n")
		fmt.Printf("Version before:   mahin %s\n", oldVersion)
		fmt.Printf("Version deployed: %s\n", newVersionStr)
		fmt.Println("[OK] Active binary matches deployed version.")
	},
}
