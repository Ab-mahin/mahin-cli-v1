// movie_rename.go — mahin movie rename
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/mahin/mahin-cli-v1/cleaner"
	"github.com/mahin/mahin-cli-v1/db"
)

var movieRenameDryRun bool

var movieRenameCmd = &cobra.Command{
	Use:   "rename [folder]",
	Short: "Rename files to clean names",
	Long: `Automatically renames messy filenames to clean format.
Example: Scream.2022.1080p.WEBRip.x264-RARBG.mkv → Scream (2022).mkv`,
	Args: cobra.MaximumNArgs(1),
	Run:  runMovieRename,
}

func init() {
	movieRenameCmd.Flags().BoolVar(&movieRenameDryRun, "dry-run", false, "Preview rename operations without changing files")
}

func runMovieRename(cmd *cobra.Command, args []string) {
	database, err := db.Open()
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Database error: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	folder := ""
	if len(args) > 0 {
		folder = args[0]
	} else {
		folder, _ = database.GetConfig("scan_dir")
		if folder == "" {
			folder = "."
		}
	}

	home, _ := os.UserHomeDir()
	folder = expandHome(folder, home)

	info, err := os.Stat(folder)
	if err != nil || !info.IsDir() {
		fmt.Fprintf(os.Stderr, "❌ Folder not found: %s\n", folder)
		os.Exit(1)
	}

	type renameItem struct {
		oldPath string
		newPath string
		oldName string
		newName string
	}

	entries, err := os.ReadDir(folder)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Cannot read folder: %v\n", err)
		os.Exit(1)
	}

	var items []renameItem
	for _, entry := range entries {
		if entry.IsDir() || !cleaner.IsVideoFile(entry.Name()) {
			continue
		}

		oldName := entry.Name()
		parsed := cleaner.Clean(oldName)
		if parsed.CleanTitle == "" {
			continue
		}

		newName := cleaner.ToCleanFileName(parsed.CleanTitle, parsed.Year, parsed.Extension)
		oldPath := filepath.Join(folder, oldName)
		newPath := filepath.Join(folder, newName)

		if oldName != newName {
			items = append(items, renameItem{
				oldPath: oldPath,
				newPath: newPath,
				oldName: oldName,
				newName: newName,
			})
		}
	}

	if len(items) == 0 {
		fmt.Println("✅ All files already have clean names!")
		return
	}

	fmt.Printf("📝 Found %d files to rename in %s:\n\n", len(items), folder)
	for i, item := range items {
		fmt.Printf("  %d. %s\n", i+1, item.oldName)
		fmt.Printf("     → %s\n\n", item.newName)
	}

	if movieRenameDryRun {
		fmt.Println("🧪 Dry run complete. No files were renamed.")
		return
	}

	success := 0
	for _, item := range items {
		if _, err := os.Stat(item.newPath); err == nil {
			fmt.Fprintf(os.Stderr, "  ⚠️  Skipped (target exists): %s\n", item.newName)
			continue
		}

		if err := os.Rename(item.oldPath, item.newPath); err != nil {
			fmt.Fprintf(os.Stderr, "  ❌ Failed: %s → %v\n", item.oldName, err)
			continue
		}
		_, _ = database.Exec(
			"UPDATE media SET current_file_path = ?, updated_at = CURRENT_TIMESTAMP WHERE current_file_path = ?",
			item.newPath,
			item.oldPath,
		)
		fmt.Printf("  ✅ %s → %s\n", item.oldName, item.newName)
		success++
	}

	fmt.Printf("\n✅ Renamed %d/%d files.\n", success, len(items))
}
