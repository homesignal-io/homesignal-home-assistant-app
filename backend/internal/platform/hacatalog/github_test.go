package hacatalog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGitHubCoreReleaseProviderFetchesLatestStableRelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"tag_name": "2026.5.2",
			"html_url": "https://github.com/home-assistant/core/releases/tag/2026.5.2",
			"draft": false,
			"prerelease": false,
			"published_at": "2026-05-18T12:00:00Z"
		}`))
	}))
	defer server.Close()
	provider := GitHubCoreReleaseProvider{Client: server.Client(), URL: server.URL}

	catalog, err := provider.FetchLatestStable(context.Background())
	if err != nil {
		t.Fatalf("fetch latest stable: %v", err)
	}
	if catalog.LatestVersion != "2026.5.2" {
		t.Fatalf("unexpected latest version %q", catalog.LatestVersion)
	}
}

func TestGitHubCoreReleaseProviderRejectsPrerelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"tag_name": "2026.6.0b0",
			"html_url": "https://github.com/home-assistant/core/releases/tag/2026.6.0b0",
			"draft": false,
			"prerelease": true
		}`))
	}))
	defer server.Close()
	provider := GitHubCoreReleaseProvider{Client: server.Client(), URL: server.URL}

	if _, err := provider.FetchLatestStable(context.Background()); err == nil {
		t.Fatalf("expected prerelease to fail")
	}
}
