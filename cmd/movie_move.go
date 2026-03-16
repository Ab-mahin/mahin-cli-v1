// movie_move.go — mahin movie move
package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mahin/mahin-cli-v1/cleaner"
	"github.com/mahin/mahin-cli-v1/db"
)

var movieMoveCmd = &cobra.Command{
	Use:   "move [id-or-title] [destination]",
	Short: "Move a movie or TV show to a destination folder",
	Long: `Move media files to organized directories by ID/title and destination.
When no args are provided, it falls back to interactive mode.
Configured destination shortcuts (Movies, TV Shows, Archive) are available in interactive mode.
The move is logged for undo support.`,
	Args: cobra.MaximumNArgs(2),
	Run:  runMovieMove,
}

func runMovieMove(cmd *cobra.Command, args []string) {
	database, err := db.Open()
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Database error: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	if len(args) > 0 {
		runMovieMoveDirect(database, args)
		return
	}

	runMovieMoveInteractive(database)
}

func runMovieMoveDirect(database *db.DB, args []string) {
	if len(args) != 2 {
		fmt.Fprintln(os.Stderr, "❌ Usage: mahin movie move <id-or-title> <destination-folder>")
		os.Exit(1)
	}

	home, _ := os.UserHomeDir()
	destDir := expandHome(strings.TrimSpace(args[1]), home)
	if destDir == "" {
		fmt.Fprintln(os.Stderr, "❌ Destination folder cannot be empty.")
		os.Exit(1)
	}

	selected, err := resolveMediaByQuery(database, args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Media not found: %v\n", err)
		os.Exit(1)
	}

	fromPath := selected.CurrentFilePath
	destPath, err := moveMedia(database, selected, destDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Move failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Moved successfully!")
	fmt.Printf("   %s\n", fromPath)
	fmt.Printf("   → %s\n", destPath)
}

func runMovieMoveInteractive(database *db.DB) {

	scanner := bufio.NewScanner(os.Stdin)

	// List media for selection
	total, _ := database.CountMedia("")
	if total == 0 {
		fmt.Println("📭 No media found. Run 'mahin movie scan <folder>' first.")
		return
	}

	media, err := database.ListMedia(0, 50)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Error: %v\n", err)
		return
	}

	fmt.Println("🎬 Select a movie/TV show to move:")
	fmt.Println()

	for i, m := range media {
		yearStr := ""
		if m.Year > 0 {
			yearStr = fmt.Sprintf("(%d)", m.Year)
		}
		typeIcon := "🎬"
		if m.Type == "tv" {
			typeIcon = "📺"
		}
		fmt.Printf("  %2d. %s %s %s\n", i+1, typeIcon, m.CleanTitle, yearStr)
	}

	fmt.Println()
	fmt.Print("  Select [number]: ")

	if !scanner.Scan() {
		return
	}
	choice, err := strconv.Atoi(strings.TrimSpace(scanner.Text()))
	if err != nil || choice < 1 || choice > len(media) {
		fmt.Println("❌ Invalid selection")
		return
	}

	selected := media[choice-1]
	fmt.Printf("\n  Selected: %s\n\n", selected.CleanTitle)

	// Show destination options
	moviesDir, _ := database.GetConfig("movies_dir")
	tvDir, _ := database.GetConfig("tv_dir")
	archiveDir, _ := database.GetConfig("archive_dir")

	// Expand ~
	home, _ := os.UserHomeDir()
	moviesDir = expandHome(moviesDir, home)
	tvDir = expandHome(tvDir, home)
	archiveDir = expandHome(archiveDir, home)

	fmt.Println("  Destination:")
	fmt.Printf("  1. 🎬 Movies     (%s)\n", moviesDir)
	fmt.Printf("  2. 📺 TV Shows   (%s)\n", tvDir)
	fmt.Printf("  3. 📦 Archive    (%s)\n", archiveDir)
	fmt.Println("  4. 📁 Custom path")
	fmt.Println()
	fmt.Print("  Choose [1-4]: ")

	if !scanner.Scan() {
		return
	}

	var destDir string
	destChoice := strings.TrimSpace(scanner.Text())
	switch destChoice {
	case "1":
		destDir = moviesDir
	case "2":
		destDir = tvDir
	case "3":
		destDir = archiveDir
	case "4":
		fmt.Print("  Enter path: ")
		if !scanner.Scan() {
			return
		}
		destDir = expandHome(strings.TrimSpace(scanner.Text()), home)
	default:
		fmt.Println("❌ Invalid choice")
		return
	}

	// Create clean filename
	cleanName := cleaner.ToCleanFileName(selected.CleanTitle, selected.Year, selected.FileExtension)
	destPath := filepath.Join(destDir, cleanName)

	fmt.Println()
	fmt.Println("  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("  📄 From: %s\n", selected.CurrentFilePath)
	fmt.Printf("  📁 To:   %s\n", destPath)
	fmt.Println("  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Print("  Are you sure? [y/N]: ")

	if !scanner.Scan() {
		return
	}
	confirm := strings.ToLower(strings.TrimSpace(scanner.Text()))
	if confirm != "y" && confirm != "yes" {
		fmt.Println("  ❌ Move cancelled.")
		return
	}

	fromPath := selected.CurrentFilePath
	destPath, err = moveMedia(database, &selected, destDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  ❌ Move failed: %v\n", err)
		return
	}

	fmt.Println()
	fmt.Println("  ✅ Moved successfully!")
	fmt.Printf("     %s\n", fromPath)
	fmt.Printf("     → %s\n", destPath)
}

func moveMedia(database *db.DB, selected *db.Media, destDir string) (string, error) {
	if selected == nil {
		return "", fmt.Errorf("no media selected")
	}
	if strings.TrimSpace(selected.CurrentFilePath) == "" {
		return "", fmt.Errorf("media does not have a current file path")
	}

	// Create destination directory
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", fmt.Errorf("cannot create directory: %w", err)
	}

	// Create clean filename
	cleanName := cleaner.ToCleanFileName(selected.CleanTitle, selected.Year, selected.FileExtension)
	destPath := filepath.Join(destDir, cleanName)

	// Move the file
	if err := os.Rename(selected.CurrentFilePath, destPath); err != nil {
		return "", err
	}

	// Log move history
	database.InsertMoveHistory(selected.ID, selected.CurrentFilePath, destPath,
		selected.OriginalFileName, cleanName)

	// Update media path
	database.UpdateMediaPath(selected.ID, destPath)

	// Save JSON history
	saveHistoryLog(database.BasePath, selected.CleanTitle, selected.Year,
		selected.CurrentFilePath, destPath)

	return destPath, nil
}

func expandHome(path, home string) string {
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	if path == "~" {
		return home
	}
	return path
}

func saveHistoryLog(basePath, title string, year int, from, to string) {
	slug := cleaner.ToSlug(title)
	if year > 0 {
		slug += "-" + strconv.Itoa(year)
	}
	histDir := filepath.Join(basePath, "json", "history", slug)
	os.MkdirAll(histDir, 0755)

	logFile := filepath.Join(histDir, "move-log.json")

	// Append to log
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	entry := fmt.Sprintf(`{"from":"%s","to":"%s","timestamp":"%s"}`+"\n",
		from, to, "now")
	f.WriteString(entry)
}
