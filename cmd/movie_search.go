// movie_search.go — mahin movie search <name>
package cmd

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mahin/mahin-cli-v1/cleaner"
	"github.com/mahin/mahin-cli-v1/db"
	"github.com/mahin/mahin-cli-v1/tmdb"
)

var movieSearchCmd = &cobra.Command{
	Use:   "search [name]",
	Short: "Search TMDb and save media metadata",
	Long: `Searches TMDb for a movie or TV show and saves the result to the local database.
If no TMDb match is found, nothing is saved.
Metadata is also stored in your configured Movies/TV folder based on media type.`,
	Args: cobra.MinimumNArgs(1),
	Run:  runMovieSearch,
}

func runMovieSearch(cmd *cobra.Command, args []string) {
	database, err := db.Open()
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Database error: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	query := strings.TrimSpace(strings.Join(args, " "))
	fmt.Printf("🔎 Searching TMDb for: %s\n\n", query)

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

	m, err := buildMediaFromTMDB(database, client, results[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to build media metadata: %v\n", err)
		os.Exit(1)
	}

	metadataPath, err := storeSearchMetadataByType(database, m, query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to store metadata file: %v\n", err)
		os.Exit(1)
	}

	if existing, err := database.GetMediaByTmdbID(m.TmdbID); err == nil && existing != nil {
		m.ID = existing.ID
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
	} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
		fmt.Fprintf(os.Stderr, "❌ Failed to check existing records: %v\n", err)
		os.Exit(1)
	} else {
		m.OriginalFileName = filepath.Base(metadataPath)
		m.OriginalFilePath = metadataPath
		m.CurrentFilePath = metadataPath
	}

	action := "saved"
	if _, err := database.InsertMedia(m); err != nil {
		if m.TmdbID > 0 {
			if updateErr := database.UpdateMediaByTmdbID(m); updateErr != nil {
				fmt.Fprintf(os.Stderr, "❌ Failed to save search result: %v\n", updateErr)
				os.Exit(1)
			}
			action = "updated"
		} else {
			fmt.Fprintf(os.Stderr, "❌ Failed to save search result: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Printf("✅ TMDb result %s in database as %s.\n", action, m.Type)
	fmt.Printf("📁 Metadata stored at: %s\n\n", metadataPath)
	printMediaDetail(m)
}

func buildMediaFromTMDB(database *db.DB, client *tmdb.Client, result tmdb.SearchResult) (*db.Media, error) {
	if result.ID <= 0 {
		return nil, fmt.Errorf("invalid TMDb result")
	}

	mediaType := "movie"
	if result.MediaType == "tv" {
		mediaType = "tv"
	}

	title := strings.TrimSpace(result.GetDisplayTitle())
	if title == "" {
		title = strings.TrimSpace(result.Title)
	}
	if title == "" {
		title = strings.TrimSpace(result.Name)
	}

	year, _ := strconv.Atoi(result.GetYear())
	m := &db.Media{
		Title:       title,
		CleanTitle:  title,
		Year:        year,
		Type:        mediaType,
		TmdbID:      result.ID,
		Description: result.Overview,
		TmdbRating:  result.VoteAvg,
		Popularity:  result.Popularity,
		Genre:       tmdb.GenreNames(result.GenreIDs),
	}

	posterPath := result.PosterPath

	if mediaType == "tv" {
		details, err := client.GetTVDetails(result.ID)
		if err == nil {
			if strings.TrimSpace(details.Name) != "" {
				m.Title = details.Name
				m.CleanTitle = details.Name
			}
			if m.Year == 0 {
				m.Year = parseYearFromDate(details.FirstAirDate)
			}
			if details.Overview != "" {
				m.Description = details.Overview
			}
			if details.VoteAvg > 0 {
				m.TmdbRating = details.VoteAvg
			}
			if details.Popularity > 0 {
				m.Popularity = details.Popularity
			}
			if len(details.Genres) > 0 {
				names := make([]string, len(details.Genres))
				for i, g := range details.Genres {
					names[i] = g.Name
				}
				m.Genre = strings.Join(names, ", ")
			}
			if details.PosterPath != "" {
				posterPath = details.PosterPath
			}
		}

		credits, err := client.GetTVCredits(result.ID)
		if err == nil {
			directors := []string{}
			for _, c := range credits.Crew {
				if c.Job == "Director" || c.Job == "Executive Producer" {
					directors = append(directors, c.Name)
				}
			}
			if len(directors) > 5 {
				directors = directors[:5]
			}
			m.Director = strings.Join(directors, ", ")

			castNames := []string{}
			for i, c := range credits.Cast {
				if i >= 10 {
					break
				}
				castNames = append(castNames, c.Name)
			}
			m.CastList = strings.Join(castNames, ", ")
		}
	} else {
		details, err := client.GetMovieDetails(result.ID)
		if err == nil {
			if strings.TrimSpace(details.Title) != "" {
				m.Title = details.Title
				m.CleanTitle = details.Title
			}
			if m.Year == 0 {
				m.Year = parseYearFromDate(details.ReleaseDate)
			}
			m.ImdbID = details.ImdbID
			if details.Overview != "" {
				m.Description = details.Overview
			}
			if details.VoteAvg > 0 {
				m.TmdbRating = details.VoteAvg
			}
			if details.Popularity > 0 {
				m.Popularity = details.Popularity
			}
			if len(details.Genres) > 0 {
				names := make([]string, len(details.Genres))
				for i, g := range details.Genres {
					names[i] = g.Name
				}
				m.Genre = strings.Join(names, ", ")
			}
			if details.PosterPath != "" {
				posterPath = details.PosterPath
			}
		}

		credits, err := client.GetMovieCredits(result.ID)
		if err == nil {
			directors := []string{}
			for _, c := range credits.Crew {
				if c.Job == "Director" {
					directors = append(directors, c.Name)
				}
			}
			m.Director = strings.Join(directors, ", ")

			castNames := []string{}
			for i, c := range credits.Cast {
				if i >= 10 {
					break
				}
				castNames = append(castNames, c.Name)
			}
			m.CastList = strings.Join(castNames, ", ")
		}
	}

	if posterPath != "" {
		if thumbPath, err := downloadSearchPoster(database, client, m, posterPath); err == nil {
			m.ThumbnailPath = thumbPath
		}
	}

	return m, nil
}

func parseYearFromDate(date string) int {
	if len(date) < 4 {
		return 0
	}
	y, err := strconv.Atoi(date[:4])
	if err != nil {
		return 0
	}
	return y
}

func downloadSearchPoster(database *db.DB, client *tmdb.Client, m *db.Media, posterPath string) (string, error) {
	if database == nil || client == nil || m == nil || posterPath == "" {
		return "", fmt.Errorf("invalid poster download input")
	}

	slug := cleaner.ToSlug(m.CleanTitle)
	if m.Year > 0 {
		slug += "-" + strconv.Itoa(m.Year)
	}
	if slug == "" {
		slug = fmt.Sprintf("tmdb-%d", m.TmdbID)
	}

	thumbDir := filepath.Join(database.BasePath, "thumbnails", slug)
	if err := os.MkdirAll(thumbDir, 0755); err != nil {
		return "", err
	}

	thumbPath := filepath.Join(thumbDir, slug+".jpg")
	if err := client.DownloadPoster(posterPath, thumbPath); err != nil {
		return "", err
	}
	return thumbPath, nil
}

func storeSearchMetadataByType(database *db.DB, m *db.Media, query string) (string, error) {
	if database == nil || m == nil {
		return "", fmt.Errorf("invalid metadata storage input")
	}

	key := "movies_dir"
	fallback := "~/Movies"
	if m.Type == "tv" {
		key = "tv_dir"
		fallback = "~/TVShows"
	}

	baseDir, _ := database.GetConfig(key)
	if strings.TrimSpace(baseDir) == "" {
		baseDir = fallback
	}

	home, _ := os.UserHomeDir()
	baseDir = expandHome(baseDir, home)

	slug := cleaner.ToSlug(m.CleanTitle)
	if m.Year > 0 {
		slug += "-" + strconv.Itoa(m.Year)
	}
	if slug == "" {
		slug = fmt.Sprintf("tmdb-%d", m.TmdbID)
	}

	metadataDir := filepath.Join(baseDir, ".mahin-metadata", slug)
	if err := os.MkdirAll(metadataDir, 0755); err != nil {
		return "", err
	}

	metadataPath := filepath.Join(metadataDir, "tmdb.json")
	payload := map[string]interface{}{
		"query":          query,
		"saved_at":       time.Now().Format(time.RFC3339),
		"title":          m.Title,
		"clean_title":    m.CleanTitle,
		"year":           m.Year,
		"type":           m.Type,
		"tmdb_id":        m.TmdbID,
		"imdb_id":        m.ImdbID,
		"description":    m.Description,
		"tmdb_rating":    m.TmdbRating,
		"popularity":     m.Popularity,
		"genre":          m.Genre,
		"director":       m.Director,
		"cast_list":      m.CastList,
		"thumbnail_path": m.ThumbnailPath,
	}

	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(metadataPath, b, 0644); err != nil {
		return "", err
	}

	return metadataPath, nil
}
