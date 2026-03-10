// github.go — queries the GitHub Releases API.
// Only job: fetch the latest release metadata (tag + asset download URLs).
package updater

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/mahin/mahin-cli-v1/config"
)

// release holds the fields we need from the GitHub API response.
type release struct {
	TagName string  `json:"tag_name"` // e.g. "v1.2.0"
	Assets  []asset `json:"assets"`
}

// asset is a single file attached to a GitHub Release.
type asset struct {
	Name               string `json:"name"`                 // e.g. "mahin-linux-amd64"
	BrowserDownloadURL string `json:"browser_download_url"` // direct download URL
}

// fetchLatestRelease calls the GitHub API and returns the latest release.
func fetchLatestRelease() (*release, error) {
	url := fmt.Sprintf(
		"https://api.github.com/repos/%s/%s/releases/latest",
		config.GitHubOwner,
		config.GitHubRepo,
	)

	client := &http.Client{Timeout: 30 * time.Second}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned HTTP %d", resp.StatusCode)
	}

	var r release
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, fmt.Errorf("failed to parse GitHub response: %w", err)
	}
	return &r, nil
}

// findAssetURLs searches the release asset list for the binary and its
// optional checksum file, both matching the current platform.
// Returns an error if the binary asset is not found.
func findAssetURLs(assets []asset, binaryName, checksumName string) (binaryURL, checksumURL string, err error) {
	for _, a := range assets {
		switch a.Name {
		case binaryName:
			binaryURL = a.BrowserDownloadURL
		case checksumName:
			checksumURL = a.BrowserDownloadURL
		}
	}

	if binaryURL == "" {
		names := make([]string, len(assets))
		for i, a := range assets {
			names[i] = a.Name
		}
		return "", "", fmt.Errorf(
			"no asset %q in release — available: %s",
			binaryName,
			strings.Join(names, ", "),
		)
	}

	return binaryURL, checksumURL, nil
}