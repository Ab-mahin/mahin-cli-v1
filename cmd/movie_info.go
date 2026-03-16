// movie_info.go — mahin movie info <id>
package cmd

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mahin/mahin-cli-v1/db"
	"github.com/mahin/mahin-cli-v1/tmdb"
)

var movieInfoCmd = &cobra.Command{
	Use:   "info [id-or-title]",
	Short: "Show detailed info for a movie or TV show",
	Long: `Display full metadata for a media item by ID number or title.
If title is used, it searches TMDb and saves the item to local database only when new.`,
	Args: cobra.MinimumNArgs(1),
	Run:  runMovieInfo,
}

func runMovieInfo(cmd *cobra.Command, args []string) {
	database, err := db.Open()
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Database error: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	query := strings.TrimSpace(strings.Join(args, " "))

	// ID path: read local library record directly.
	if id, err := strconv.ParseInt(query, 10, 64); err == nil {
		m, getErr := database.GetMediaByID(id)
		if getErr != nil {
			fmt.Fprintf(os.Stderr, "❌ Media not found for ID %d: %v\n", id, getErr)
			os.Exit(1)
		}
		printMediaDetail(m)
		return
	}

	// Title path: search TMDb and save only if not already present.
	apiKey, _ := database.GetConfig("tmdb_api_key")
	if apiKey == "" {
		apiKey = os.Getenv("TMDB_API_KEY")
	}
	if apiKey == "" {
		apiKey = tmdb.DefaultAPIKey
	}

	client := tmdb.NewClient(apiKey)
	results, err := client.SearchMulti(query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ TMDb search error: %v\n", err)
		os.Exit(1)
	}
	if len(results) == 0 {
		fmt.Println("📭 No TMDb result found. Nothing was saved.")
		return
	}

	best := results[0]
	m, err := buildMediaFromTMDB(database, client, best)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to build TMDb metadata: %v\n", err)
		os.Exit(1)
	}

	metadataPath, err := storeSearchMetadataByType(database, m, query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to store metadata file: %v\n", err)
		os.Exit(1)
	}

	if existing, getErr := database.GetMediaByTmdbID(best.ID); getErr == nil && existing != nil {
		fmt.Println("ℹ️  Already in database. No new record created.")
		fmt.Printf("📁 Metadata stored at: %s\n\n", metadataPath)

		if existing.OriginalFileName != "" {
			m.OriginalFileName = existing.OriginalFileName
		}
		if existing.OriginalFilePath != "" {
			m.OriginalFilePath = existing.OriginalFilePath
		}
		if existing.CurrentFilePath != "" {
			m.CurrentFilePath = existing.CurrentFilePath
		}
		if existing.FileExtension != "" {
			m.FileExtension = existing.FileExtension
		}
		if existing.FileSize > 0 {
			m.FileSize = existing.FileSize
		}

		fmt.Println()
		printMediaDetail(m)
		return
	} else if getErr != nil && !errors.Is(getErr, sql.ErrNoRows) {
		fmt.Fprintf(os.Stderr, "❌ Failed to check existing media: %v\n", getErr)
		os.Exit(1)
	}

	m.OriginalFileName = filepath.Base(metadataPath)
	m.OriginalFilePath = metadataPath
	m.CurrentFilePath = metadataPath
	m.FileExtension = filepath.Ext(metadataPath)
	if fi, statErr := os.Stat(metadataPath); statErr == nil {
		m.FileSize = fi.Size()
	}

	if _, err := database.InsertMedia(m); err != nil {
		if existing, getErr := database.GetMediaByTmdbID(best.ID); getErr == nil && existing != nil {
			fmt.Println("ℹ️  Already in database. No new record created.")
			fmt.Printf("📁 Metadata stored at: %s\n", metadataPath)
			fmt.Println()
			printMediaDetail(existing)
			return
		}
		fmt.Fprintf(os.Stderr, "❌ Failed to save TMDb result: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Saved TMDb result to database.")
	fmt.Printf("📁 Metadata stored at: %s\n", metadataPath)
	fmt.Println()
	printMediaDetail(m)
}
