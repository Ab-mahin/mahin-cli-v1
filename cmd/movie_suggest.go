// movie_suggest.go — mahin movie suggest [N]
package cmd

import (
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mahin/mahin-cli-v1/db"
	"github.com/mahin/mahin-cli-v1/tmdb"
)

var (
	movieSuggestRandom bool
	movieSuggestType   string
)

var movieSuggestCmd = &cobra.Command{
	Use:   "suggest [N]",
	Short: "Get movie or TV show suggestions",
	Long: `Suggests movies or TV shows based on your library.
Use --random for random trending picks without library analysis.`,
	Args: cobra.MaximumNArgs(1),
	Run:  runMovieSuggest,
}

func init() {
	movieSuggestCmd.Flags().BoolVar(&movieSuggestRandom, "random", false, "Return random suggestions from trending content")
	movieSuggestCmd.Flags().StringVar(&movieSuggestType, "type", "auto", "Suggestion source: auto, movie, tv")
}

func runMovieSuggest(cmd *cobra.Command, args []string) {
	count := 10
	if len(args) > 0 {
		if n, err := strconv.Atoi(args[0]); err == nil && n > 0 {
			count = n
		}
	}

	database, err := db.Open()
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Database error: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	apiKey, _ := database.GetConfig("tmdb_api_key")
	if apiKey == "" {
		apiKey = os.Getenv("TMDB_API_KEY")
	}
	if apiKey == "" {
		apiKey = tmdb.DefaultAPIKey
	}
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "❌ TMDb API key required for suggestions.")
		fmt.Fprintln(os.Stderr, "   Set with: mahin movie config set tmdb_api_key YOUR_KEY")
		os.Exit(1)
	}

	client := tmdb.NewClient(apiKey)

	if movieSuggestRandom {
		suggestRandom(client, count)
		return
	}

	typeChoice := strings.ToLower(strings.TrimSpace(movieSuggestType))
	switch typeChoice {
	case "movie":
		suggestByType(database, client, "movie", count)
	case "tv":
		suggestByType(database, client, "tv", count)
	case "auto", "":
		movieCount, _ := database.CountMedia("movie")
		tvCount, _ := database.CountMedia("tv")

		if movieCount == 0 && tvCount == 0 {
			fmt.Println("📭 Your library is empty. Showing random trending suggestions.")
			suggestRandom(client, count)
			return
		}

		if movieCount >= tvCount {
			suggestByType(database, client, "movie", count)
			return
		}
		suggestByType(database, client, "tv", count)
	default:
		fmt.Fprintf(os.Stderr, "❌ Invalid --type value %q. Use: auto, movie, tv\n", movieSuggestType)
		os.Exit(1)
	}
}

func suggestByType(database *db.DB, client *tmdb.Client, mediaType string, count int) {
	typeName := "Movies"
	if mediaType == "tv" {
		typeName = "TV Shows"
	}

	fmt.Printf("🔍 Analyzing your %s library...\n\n", typeName)

	// Get top genres from library
	genres, err := database.TopGenres(5)
	if err != nil || len(genres) == 0 {
		fmt.Println("⚠️  Not enough data. Showing trending instead.")
		showTrending(client, mediaType, count)
		return
	}

	// Sort genres by frequency
	type genreCount struct {
		name  string
		count int
	}
	var sorted []genreCount
	for name, cnt := range genres {
		sorted = append(sorted, genreCount{name, cnt})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].count > sorted[j].count
	})

	fmt.Printf("📊 Your top genres: ")
	for i, g := range sorted {
		if i >= 3 {
			break
		}
		if i > 0 {
			fmt.Print(", ")
		}
		fmt.Printf("%s (%d)", g.name, g.count)
	}
	fmt.Println()
	fmt.Println()

	// Get existing media to avoid duplicates
	existing, _ := database.MediaByType(mediaType, 1000)
	existingIDs := make(map[int]bool)
	for _, e := range existing {
		existingIDs[e.TmdbID] = true
	}

	// Fetch recommendations based on random items from library
	var suggestions []tmdb.SearchResult
	if len(existing) > 0 {
		// Pick random items to get recommendations from
		indices := rand.Perm(len(existing))
		for _, idx := range indices {
			if len(suggestions) >= count {
				break
			}
			recs, err := client.GetRecommendations(existing[idx].TmdbID, mediaType, 1)
			if err != nil {
				continue
			}
			for _, r := range recs {
				if !existingIDs[r.ID] && len(suggestions) < count {
					suggestions = append(suggestions, r)
					existingIDs[r.ID] = true
				}
			}
		}
	}

	// Fill remaining with trending
	if len(suggestions) < count {
		trending, _ := client.Trending(mediaType)
		for _, t := range trending {
			if !existingIDs[t.ID] && len(suggestions) < count {
				suggestions = append(suggestions, t)
				existingIDs[t.ID] = true
			}
		}
	}

	printSuggestions(suggestions, typeName)
}

func suggestRandom(client *tmdb.Client, count int) {
	fmt.Println("🎲 Fetching random suggestions...")

	var suggestions []tmdb.SearchResult
	seenIDs := make(map[int]bool)

	// Mix movie and TV trending
	movieTrending, _ := client.Trending("movie")
	tvTrending, _ := client.Trending("tv")

	all := append(movieTrending, tvTrending...)
	rand.Shuffle(len(all), func(i, j int) { all[i], all[j] = all[j], all[i] })

	for _, item := range all {
		if len(suggestions) >= count {
			break
		}
		if !seenIDs[item.ID] {
			suggestions = append(suggestions, item)
			seenIDs[item.ID] = true
		}
	}

	printSuggestions(suggestions, "Movies & TV Shows")
}

func showTrending(client *tmdb.Client, mediaType string, count int) {
	trending, err := client.Trending(mediaType)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ TMDb error: %v\n", err)
		return
	}
	if len(trending) > count {
		trending = trending[:count]
	}

	typeName := "Movies"
	if mediaType == "tv" {
		typeName = "TV Shows"
	}
	printSuggestions(trending, typeName)
}

func printSuggestions(suggestions []tmdb.SearchResult, category string) {
	if len(suggestions) == 0 {
		fmt.Println("📭 No suggestions available.")
		return
	}

	fmt.Printf("✨ Suggested %s (%d):\n\n", category, len(suggestions))

	for i, s := range suggestions {
		title := s.GetDisplayTitle()
		year := s.GetYear()
		rating := fmt.Sprintf("%.1f", s.VoteAvg)
		genres := tmdb.GenreNames(s.GenreIDs)

		fmt.Printf("  %2d. %s", i+1, title)
		if year != "" {
			fmt.Printf(" (%s)", year)
		}
		fmt.Printf("  ⭐ %s", rating)
		if genres != "" {
			fmt.Printf("  [%s]", genres)
		}
		fmt.Println()
	}
}
