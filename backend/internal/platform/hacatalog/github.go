package hacatalog

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/homesignal-io/homesignal-home-assistant-app/backend/internal/domain/releases"
)

const DefaultGitHubLatestReleaseURL = "https://api.github.com/repos/home-assistant/core/releases/latest"

type GitHubCoreReleaseProvider struct {
	Client *http.Client
	URL    string
}

func (p GitHubCoreReleaseProvider) FetchLatestStable(ctx context.Context) (releases.HACoreVersionCatalog, error) {
	client := p.Client
	if client == nil {
		client = http.DefaultClient
	}
	url := strings.TrimSpace(p.URL)
	if url == "" {
		url = DefaultGitHubLatestReleaseURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return releases.HACoreVersionCatalog{}, fmt.Errorf("build Home Assistant release request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.Do(req)
	if err != nil {
		return releases.HACoreVersionCatalog{}, fmt.Errorf("fetch Home Assistant latest release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return releases.HACoreVersionCatalog{}, fmt.Errorf("fetch Home Assistant latest release: status %d", resp.StatusCode)
	}
	var payload struct {
		TagName     string    `json:"tag_name"`
		HTMLURL     string    `json:"html_url"`
		Draft       bool      `json:"draft"`
		Prerelease  bool      `json:"prerelease"`
		PublishedAt time.Time `json:"published_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return releases.HACoreVersionCatalog{}, fmt.Errorf("decode Home Assistant latest release: %w", err)
	}
	if payload.Draft || payload.Prerelease {
		return releases.HACoreVersionCatalog{}, fmt.Errorf("latest Home Assistant release is not stable")
	}
	version := strings.TrimPrefix(strings.TrimSpace(payload.TagName), "v")
	if version == "" {
		return releases.HACoreVersionCatalog{}, fmt.Errorf("latest Home Assistant release tag is empty")
	}
	return releases.HACoreVersionCatalog{
		CatalogKey:    releases.HACatalogStable,
		LatestVersion: version,
		SourceURL:     strings.TrimSpace(payload.HTMLURL),
		Status:        releases.HACatalogStatusActive,
		Metadata:      mustJSON(map[string]any{"provider": "github_releases_latest"}),
		FetchedAt:     time.Now().UTC(),
	}, nil
}

func mustJSON(value any) json.RawMessage {
	payload, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return payload
}
