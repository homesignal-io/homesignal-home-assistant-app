package releases

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	HACatalogStable = "stable"

	HACatalogStatusActive      HACatalogStatus = "active"
	HACatalogStatusStale       HACatalogStatus = "stale"
	HACatalogStatusUnavailable HACatalogStatus = "unavailable"

	HAAdvisoryCurrent         HAAdvisoryStatus = "current"
	HAAdvisoryUpdateAvailable HAAdvisoryStatus = "update_available"
	HAAdvisoryHidden          HAAdvisoryStatus = "hidden"

	DefaultHACatalogTTL = 24 * time.Hour
)

type HACatalogStatus string
type HAAdvisoryStatus string

type HACoreVersionCatalog struct {
	CatalogKey    string
	LatestVersion string
	SourceURL     string
	Status        HACatalogStatus
	Metadata      json.RawMessage
	FetchedAt     time.Time
	ExpiresAt     time.Time
}

type HAUpdateAdvisory struct {
	AccountID        string
	SiteID           string
	DeviceID         string
	InstalledVersion string
	LatestVersion    string
	AdvisoryStatus   HAAdvisoryStatus
	ReasonCode       string
	CatalogFetchedAt *time.Time
	ObservedAt       time.Time
}

type HAAdvisoryRequest struct {
	AccountID        string
	SiteID           string
	DeviceID         string
	InstalledVersion string
	ObservedAt       time.Time
}

type HAVersionRepository interface {
	SaveHACoreCatalog(ctx context.Context, catalog HACoreVersionCatalog) error
	GetHACoreCatalog(ctx context.Context, catalogKey string) (HACoreVersionCatalog, error)
	UpsertHAUpdateAdvisory(ctx context.Context, advisory HAUpdateAdvisory) error
}

type HAVersionProvider interface {
	FetchLatestStable(ctx context.Context) (HACoreVersionCatalog, error)
}

type HAVersionService struct {
	Repository HAVersionRepository
	Provider   HAVersionProvider
	Clock      Clock
	TTL        time.Duration
}

func (s HAVersionService) RefreshLatestStable(ctx context.Context) (HACoreVersionCatalog, error) {
	if s.Repository == nil {
		return HACoreVersionCatalog{}, fmt.Errorf("Home Assistant version repository is required")
	}
	if s.Provider == nil {
		return HACoreVersionCatalog{}, fmt.Errorf("Home Assistant version provider is required")
	}
	now := s.now()
	catalog, err := s.Provider.FetchLatestStable(ctx)
	if err != nil {
		catalog = HACoreVersionCatalog{
			CatalogKey: HACatalogStable,
			Status:     HACatalogStatusUnavailable,
			Metadata:   mustMarshalJSON(map[string]string{"reason_code": "provider_unavailable"}),
			FetchedAt:  now,
			ExpiresAt:  now.Add(s.ttl()),
		}
		if saveErr := s.Repository.SaveHACoreCatalog(ctx, catalog); saveErr != nil {
			return HACoreVersionCatalog{}, fmt.Errorf("fetch latest Home Assistant version: %w; save unavailable catalog: %v", err, saveErr)
		}
		return catalog, fmt.Errorf("fetch latest Home Assistant version: %w", err)
	}
	catalog.CatalogKey = strings.TrimSpace(catalog.CatalogKey)
	if catalog.CatalogKey == "" {
		catalog.CatalogKey = HACatalogStable
	}
	catalog.Status = HACatalogStatusActive
	catalog.LatestVersion = strings.TrimSpace(catalog.LatestVersion)
	catalog.SourceURL = strings.TrimSpace(catalog.SourceURL)
	catalog.Metadata = normalizeJSON(catalog.Metadata)
	if catalog.FetchedAt.IsZero() {
		catalog.FetchedAt = now
	}
	if catalog.ExpiresAt.IsZero() {
		catalog.ExpiresAt = catalog.FetchedAt.Add(s.ttl())
	}
	if err := validateHACatalog(catalog); err != nil {
		return HACoreVersionCatalog{}, err
	}
	if err := s.Repository.SaveHACoreCatalog(ctx, catalog); err != nil {
		return HACoreVersionCatalog{}, fmt.Errorf("save Home Assistant version catalog: %w", err)
	}
	return catalog, nil
}

func (s HAVersionService) AdvisoryForInstalled(ctx context.Context, req HAAdvisoryRequest) (HAUpdateAdvisory, error) {
	if s.Repository == nil {
		return HAUpdateAdvisory{}, fmt.Errorf("Home Assistant version repository is required")
	}
	now := s.now()
	if req.ObservedAt.IsZero() {
		req.ObservedAt = now
	}
	base := HAUpdateAdvisory{
		AccountID:        strings.TrimSpace(req.AccountID),
		SiteID:           strings.TrimSpace(req.SiteID),
		DeviceID:         strings.TrimSpace(req.DeviceID),
		InstalledVersion: strings.TrimSpace(req.InstalledVersion),
		AdvisoryStatus:   HAAdvisoryHidden,
		ReasonCode:       "no_advisory",
		ObservedAt:       req.ObservedAt.UTC(),
	}
	if err := validateHAAdvisoryBase(base); err != nil {
		return HAUpdateAdvisory{}, err
	}
	catalog, err := s.Repository.GetHACoreCatalog(ctx, HACatalogStable)
	if err != nil {
		base.ReasonCode = "catalog_missing"
		return s.saveAdvisory(ctx, base)
	}
	if !catalogUsable(catalog, now) {
		base.LatestVersion = strings.TrimSpace(catalog.LatestVersion)
		base.ReasonCode = "catalog_unavailable"
		if catalog.Status == HACatalogStatusActive && now.After(catalog.ExpiresAt) {
			base.ReasonCode = "catalog_stale"
		}
		if !catalog.FetchedAt.IsZero() {
			base.CatalogFetchedAt = &catalog.FetchedAt
		}
		return s.saveAdvisory(ctx, base)
	}
	base.LatestVersion = catalog.LatestVersion
	base.CatalogFetchedAt = &catalog.FetchedAt
	if base.InstalledVersion == "" {
		base.ReasonCode = "installed_version_missing"
		return s.saveAdvisory(ctx, base)
	}
	comparison, err := compareDottedVersion(base.InstalledVersion, catalog.LatestVersion)
	if err != nil {
		base.ReasonCode = "installed_version_ambiguous"
		return s.saveAdvisory(ctx, base)
	}
	if comparison < 0 {
		base.AdvisoryStatus = HAAdvisoryUpdateAvailable
		base.ReasonCode = "ha_core_update_available"
		return s.saveAdvisory(ctx, base)
	}
	base.AdvisoryStatus = HAAdvisoryCurrent
	base.ReasonCode = "ha_core_current"
	return s.saveAdvisory(ctx, base)
}

func validateHACatalog(catalog HACoreVersionCatalog) error {
	if catalog.CatalogKey != HACatalogStable {
		return fmt.Errorf("unsupported Home Assistant version catalog %q", catalog.CatalogKey)
	}
	switch catalog.Status {
	case HACatalogStatusActive:
		if strings.TrimSpace(catalog.LatestVersion) == "" {
			return fmt.Errorf("latest Home Assistant version is required for active catalog")
		}
		if _, err := parseDottedVersion(catalog.LatestVersion); err != nil {
			return fmt.Errorf("latest Home Assistant version is ambiguous: %w", err)
		}
	case HACatalogStatusStale, HACatalogStatusUnavailable:
	default:
		return fmt.Errorf("unsupported Home Assistant catalog status %q", catalog.Status)
	}
	if !json.Valid(catalog.Metadata) {
		return fmt.Errorf("Home Assistant catalog metadata must be valid JSON")
	}
	return nil
}

func validateHAAdvisoryBase(advisory HAUpdateAdvisory) error {
	if advisory.AccountID == "" || advisory.SiteID == "" || advisory.DeviceID == "" {
		return fmt.Errorf("account_id, site_id, and device_id are required")
	}
	return nil
}

func catalogUsable(catalog HACoreVersionCatalog, now time.Time) bool {
	if catalog.Status != HACatalogStatusActive {
		return false
	}
	if strings.TrimSpace(catalog.LatestVersion) == "" {
		return false
	}
	return catalog.ExpiresAt.IsZero() || now.Before(catalog.ExpiresAt) || now.Equal(catalog.ExpiresAt)
}

func (s HAVersionService) saveAdvisory(ctx context.Context, advisory HAUpdateAdvisory) (HAUpdateAdvisory, error) {
	if err := s.Repository.UpsertHAUpdateAdvisory(ctx, advisory); err != nil {
		return HAUpdateAdvisory{}, fmt.Errorf("save Home Assistant update advisory: %w", err)
	}
	return advisory, nil
}

func (s HAVersionService) now() time.Time {
	if s.Clock != nil {
		return s.Clock().UTC()
	}
	return time.Now().UTC()
}

func (s HAVersionService) ttl() time.Duration {
	if s.TTL > 0 {
		return s.TTL
	}
	return DefaultHACatalogTTL
}
