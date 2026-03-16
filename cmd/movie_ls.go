// movie_ls.go — mahin movie ls
package cmd

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mahin/mahin-cli-v1/cleaner"
	"github.com/mahin/mahin-cli-v1/db"
)

var (
	movieLsLimit int
	movieLsSort  string
)

var movieLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List movies and TV shows from your library",
	Long: `Lists scanned movies and TV shows with pagination.
Use --limit to control page size and --sort (title|rating|year) to order results.
Press N for next page, P for previous, Q to quit.`,
	Run: runMovieLs,
}

func init() {
	movieLsCmd.Flags().IntVar(&movieLsLimit, "limit", 0, "Items per page (defaults to page_size config or 20)")
	movieLsCmd.Flags().StringVar(&movieLsSort, "sort", "title", "Sort by: title, rating, year")
}

func runMovieLs(cmd *cobra.Command, args []string) {
	database, err := db.Open()
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Database error: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	pageSize := movieLsLimit
	if pageSize <= 0 {
		pageSizeStr, _ := database.GetConfig("page_size")
		pageSize, _ = strconv.Atoi(pageSizeStr)
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	sortBy := strings.ToLower(strings.TrimSpace(movieLsSort))
	switch sortBy {
	case "title", "rating", "year":
	default:
		fmt.Fprintf(os.Stderr, "❌ Invalid --sort value %q. Use: title, rating, year\n", movieLsSort)
		os.Exit(1)
	}

	allMedia, err := database.ListMedia(0, 100000)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Error: %v\n", err)
		os.Exit(1)
	}

	var libraryMedia []db.Media
	for _, m := range allMedia {
		if strings.TrimSpace(m.CurrentFilePath) == "" {
			continue
		}
		if cleaner.IsVideoFile(m.CurrentFilePath) {
			libraryMedia = append(libraryMedia, m)
		}
	}

	total := len(libraryMedia)
	if total == 0 {
		fmt.Println("📭 No local movie/TV files found in your library.")
		fmt.Println("   Run: mahin movie scan <folder>")
		return
	}

	sortMediaList(libraryMedia, sortBy)

	offset := 0
	scanner := bufio.NewScanner(os.Stdin)

	for {
		pageEnd := offset + pageSize
		if pageEnd > total {
			pageEnd = total
		}
		media := libraryMedia[offset:pageEnd]

		// Clear screen
		fmt.Print("\033[H\033[2J")

		page := (offset / pageSize) + 1
		totalPages := (total + pageSize - 1) / pageSize

		fmt.Printf("🎬 Your Library — Page %d/%d (%d total, sorted by %s)\n", page, totalPages, total, sortBy)
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

		for i, m := range media {
			num := offset + i + 1
			yearStr := ""
			if m.Year > 0 {
				yearStr = fmt.Sprintf("(%d)", m.Year)
			}

			rating := "N/A"
			if m.TmdbRating > 0 {
				rating = fmt.Sprintf("%.1f", m.TmdbRating)
			} else if m.ImdbRating > 0 {
				rating = fmt.Sprintf("%.1f", m.ImdbRating)
			}

			typeIcon := "🎬"
			if m.Type == "tv" {
				typeIcon = "📺"
			}

			fmt.Printf("  %3d. %-40s %-6s  ⭐ %-4s  %s %s\n",
				num, m.CleanTitle, yearStr, rating, typeIcon, capitalize(m.Type))
		}

		fmt.Println()
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Print("  [N] Next  [P] Previous  [Q] Quit  [number] View details → ")

		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		switch {
		case input == "n" || input == "N":
			if offset+pageSize < total {
				offset += pageSize
			} else {
				fmt.Println("  ⚠️  Already on last page")
			}
		case input == "p" || input == "P":
			if offset-pageSize >= 0 {
				offset -= pageSize
			} else {
				fmt.Println("  ⚠️  Already on first page")
			}
		case input == "q" || input == "Q":
			fmt.Println("👋 Bye!")
			return
		default:
			if num, err := strconv.Atoi(input); err == nil && num > 0 && num <= total {
				showMediaDetail(database, libraryMedia[num-1].ID)
				fmt.Print("\nPress Enter to continue...")
				scanner.Scan()
			}
		}
	}
}

func sortMediaList(items []db.Media, sortBy string) {
	switch sortBy {
	case "rating":
		sort.SliceStable(items, func(i, j int) bool {
			left := mediaRating(items[i])
			right := mediaRating(items[j])
			if left == right {
				return strings.ToLower(items[i].CleanTitle) < strings.ToLower(items[j].CleanTitle)
			}
			return left > right
		})
	case "year":
		sort.SliceStable(items, func(i, j int) bool {
			if items[i].Year == items[j].Year {
				return strings.ToLower(items[i].CleanTitle) < strings.ToLower(items[j].CleanTitle)
			}
			return items[i].Year > items[j].Year
		})
	default: // title
		sort.SliceStable(items, func(i, j int) bool {
			return strings.ToLower(items[i].CleanTitle) < strings.ToLower(items[j].CleanTitle)
		})
	}
}

func mediaRating(m db.Media) float64 {
	if m.TmdbRating > 0 {
		return m.TmdbRating
	}
	return m.ImdbRating
}

func showMediaDetail(database *db.DB, id int64) {
	m, err := database.GetMediaByID(id)
	if err != nil {
		fmt.Printf("  ❌ Not found: %v\n", err)
		return
	}

	fmt.Print("\033[H\033[2J")
	printMediaDetail(m)
}

func printMediaDetail(m *db.Media) {
	typeIcon := "🎬 Movie"
	if m.Type == "tv" {
		typeIcon = "📺 TV Show"
	}

	fmt.Println("╔══════════════════════════════════════════════════════════╗")
	fmt.Printf("║  %s\n", m.Title)
	fmt.Println("╚══════════════════════════════════════════════════════════╝")
	fmt.Println()

	if m.Year > 0 {
		fmt.Printf("  📅 Year:        %d\n", m.Year)
	}
	fmt.Printf("  🏷️  Type:        %s\n", typeIcon)

	if m.ImdbRating > 0 {
		fmt.Printf("  ⭐ IMDb:        %.1f\n", m.ImdbRating)
	}
	if m.TmdbRating > 0 {
		fmt.Printf("  ⭐ TMDb:        %.1f\n", m.TmdbRating)
	}
	if m.Popularity > 0 {
		fmt.Printf("  📈 Popularity:  %.0f\n", m.Popularity)
	}

	if m.Genre != "" {
		fmt.Printf("  🎭 Genre:       %s\n", m.Genre)
	}
	if m.Director != "" {
		fmt.Printf("  🎬 Director:    %s\n", m.Director)
	}
	if m.CastList != "" {
		fmt.Printf("  👥 Cast:        %s\n", m.CastList)
	}

	if m.Description != "" {
		fmt.Println()
		fmt.Printf("  📝 %s\n", m.Description)
	}

	if m.ThumbnailPath != "" {
		fmt.Println()
		fmt.Printf("  🖼️  Thumbnail:   %s\n", m.ThumbnailPath)
	}

	if m.CurrentFilePath != "" {
		fmt.Println()
		fmt.Printf("  📁 File:        %s\n", m.CurrentFilePath)
	}
}

func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	if s[0] >= 'a' && s[0] <= 'z' {
		return string(s[0]-32) + s[1:]
	}
	return s
}
