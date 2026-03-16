package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mahin/mahin-cli-v1/db"
)

// resolveMediaByQuery resolves a media item by numeric ID or fuzzy title query.
func resolveMediaByQuery(database *db.DB, query string) (*db.Media, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("empty media identifier")
	}

	if id, err := strconv.ParseInt(query, 10, 64); err == nil {
		m, err := database.GetMediaByID(id)
		if err != nil {
			return nil, fmt.Errorf("media not found for ID %d", id)
		}
		return m, nil
	}

	results, err := database.SearchMedia(query)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("media not found for %q", query)
	}

	for _, m := range results {
		if strings.EqualFold(m.CleanTitle, query) || strings.EqualFold(m.Title, query) {
			picked := m
			return &picked, nil
		}
	}

	queryLower := strings.ToLower(query)
	for _, m := range results {
		if strings.HasPrefix(strings.ToLower(m.CleanTitle), queryLower) ||
			strings.HasPrefix(strings.ToLower(m.Title), queryLower) {
			picked := m
			return &picked, nil
		}
	}

	picked := results[0]
	return &picked, nil
}
