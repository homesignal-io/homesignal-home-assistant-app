package releases

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestHAVersionServiceShowsUpdateAdvisoryFromCachedLatest(t *testing.T) {
	now := fixedClock()
	repo := newFakeHAVersionRepository()
	repo.catalog = HACoreVersionCatalog{
		CatalogKey:    HACatalogStable,
		LatestVersion: "2026.5.2",
		Status:        HACatalogStatusActive,
		FetchedAt:     now.Add(-time.Hour),
		ExpiresAt:     now.Add(time.Hour),
	}
	service := HAVersionService{Repository: repo, Clock: fixedClock}

	advisory, err := service.AdvisoryForInstalled(context.Background(), HAAdvisoryRequest{
		AccountID:        "acct_123",
		SiteID:           "site_123",
		DeviceID:         "dev_123",
		InstalledVersion: "2026.5.1",
	})
	if err != nil {
		t.Fatalf("advisory: %v", err)
	}
	if advisory.AdvisoryStatus != HAAdvisoryUpdateAvailable || advisory.ReasonCode != "ha_core_update_available" {
		t.Fatalf("expected update advisory, got %#v", advisory)
	}
	if repo.advisories["dev_123"].LatestVersion != "2026.5.2" {
		t.Fatalf("expected advisory read model to be stored")
	}
}

func TestHAVersionServiceMarksCurrentWhenInstalledVersionIsLatest(t *testing.T) {
	now := fixedClock()
	repo := newFakeHAVersionRepository()
	repo.catalog = HACoreVersionCatalog{
		CatalogKey:    HACatalogStable,
		LatestVersion: "2026.5.2",
		Status:        HACatalogStatusActive,
		FetchedAt:     now.Add(-time.Hour),
		ExpiresAt:     now.Add(time.Hour),
	}
	service := HAVersionService{Repository: repo, Clock: fixedClock}

	advisory, err := service.AdvisoryForInstalled(context.Background(), HAAdvisoryRequest{
		AccountID:        "acct_123",
		SiteID:           "site_123",
		DeviceID:         "dev_123",
		InstalledVersion: "2026.5.2",
	})
	if err != nil {
		t.Fatalf("advisory: %v", err)
	}
	if advisory.AdvisoryStatus != HAAdvisoryCurrent || advisory.ReasonCode != "ha_core_current" {
		t.Fatalf("expected current advisory, got %#v", advisory)
	}
}

func TestHAVersionServiceHidesAdvisoryWhenCatalogIsMissingOrStale(t *testing.T) {
	now := fixedClock()
	tests := []struct {
		name    string
		catalog HACoreVersionCatalog
		missing bool
		reason  string
	}{
		{name: "missing", missing: true, reason: "catalog_missing"},
		{
			name: "stale",
			catalog: HACoreVersionCatalog{
				CatalogKey:    HACatalogStable,
				LatestVersion: "2026.5.2",
				Status:        HACatalogStatusActive,
				FetchedAt:     now.Add(-48 * time.Hour),
				ExpiresAt:     now.Add(-time.Hour),
			},
			reason: "catalog_stale",
		},
		{
			name: "unavailable",
			catalog: HACoreVersionCatalog{
				CatalogKey: HACatalogStable,
				Status:     HACatalogStatusUnavailable,
			},
			reason: "catalog_unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeHAVersionRepository()
			repo.missing = tt.missing
			repo.catalog = tt.catalog
			service := HAVersionService{Repository: repo, Clock: fixedClock}

			advisory, err := service.AdvisoryForInstalled(context.Background(), HAAdvisoryRequest{
				AccountID:        "acct_123",
				SiteID:           "site_123",
				DeviceID:         "dev_123",
				InstalledVersion: "2026.5.1",
			})
			if err != nil {
				t.Fatalf("advisory: %v", err)
			}
			if advisory.AdvisoryStatus != HAAdvisoryHidden || advisory.ReasonCode != tt.reason {
				t.Fatalf("expected hidden %s advisory, got %#v", tt.reason, advisory)
			}
		})
	}
}

func TestHAVersionServiceRefreshesLatestStableCatalog(t *testing.T) {
	repo := newFakeHAVersionRepository()
	provider := &fakeHAVersionProvider{catalog: HACoreVersionCatalog{
		LatestVersion: "2026.5.2",
		SourceURL:     "https://github.com/home-assistant/core/releases/tag/2026.5.2",
	}}
	service := HAVersionService{Repository: repo, Provider: provider, Clock: fixedClock}

	catalog, err := service.RefreshLatestStable(context.Background())
	if err != nil {
		t.Fatalf("refresh latest stable: %v", err)
	}
	if catalog.Status != HACatalogStatusActive || catalog.ExpiresAt.Sub(catalog.FetchedAt) != DefaultHACatalogTTL {
		t.Fatalf("unexpected catalog %#v", catalog)
	}
	if repo.catalog.LatestVersion != "2026.5.2" {
		t.Fatalf("expected catalog to be stored")
	}
}

type fakeHAVersionRepository struct {
	catalog    HACoreVersionCatalog
	missing    bool
	advisories map[string]HAUpdateAdvisory
}

func newFakeHAVersionRepository() *fakeHAVersionRepository {
	return &fakeHAVersionRepository{advisories: map[string]HAUpdateAdvisory{}}
}

func (r *fakeHAVersionRepository) SaveHACoreCatalog(_ context.Context, catalog HACoreVersionCatalog) error {
	r.catalog = catalog
	r.missing = false
	return nil
}

func (r *fakeHAVersionRepository) GetHACoreCatalog(_ context.Context, _ string) (HACoreVersionCatalog, error) {
	if r.missing {
		return HACoreVersionCatalog{}, ErrNotFound
	}
	return r.catalog, nil
}

func (r *fakeHAVersionRepository) UpsertHAUpdateAdvisory(_ context.Context, advisory HAUpdateAdvisory) error {
	r.advisories[advisory.DeviceID] = advisory
	return nil
}

type fakeHAVersionProvider struct {
	catalog HACoreVersionCatalog
	err     error
}

func (p *fakeHAVersionProvider) FetchLatestStable(context.Context) (HACoreVersionCatalog, error) {
	if p.err != nil {
		return HACoreVersionCatalog{}, p.err
	}
	if p.catalog.LatestVersion == "error" {
		return HACoreVersionCatalog{}, errors.New("provider failed")
	}
	return p.catalog, nil
}
