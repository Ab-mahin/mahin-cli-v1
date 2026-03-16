// movie_play.go — mahin movie play <id>
package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/mahin/mahin-cli-v1/db"
)

var moviePlayCmd = &cobra.Command{
	Use:   "play [id-or-title]",
	Short: "Play a movie or TV show with the default player",
	Long:  `Opens a media file with the system's default video player by ID or title.`,
	Args:  cobra.ExactArgs(1),
	Run:   runMoviePlay,
}

func runMoviePlay(cmd *cobra.Command, args []string) {
	database, err := db.Open()
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Database error: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	m, err := resolveMediaByQuery(database, args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Media not found: %v\n", err)
		os.Exit(1)
	}

	filePath := m.CurrentFilePath
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "❌ File not found: %s\n", filePath)
		os.Exit(1)
	}

	fmt.Printf("▶️  Playing: %s", m.CleanTitle)
	if m.Year > 0 {
		fmt.Printf(" (%d)", m.Year)
	}
	fmt.Println()

	var openCmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		openCmd = exec.Command("open", filePath)
	case "linux":
		openCmd = exec.Command("xdg-open", filePath)
	case "windows":
		openCmd = exec.Command("cmd", "/c", "start", "", filePath)
	default:
		fmt.Fprintf(os.Stderr, "❌ Unsupported OS: %s\n", runtime.GOOS)
		os.Exit(1)
	}

	if err := openCmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Cannot open player: %v\n", err)
		os.Exit(1)
	}
}
