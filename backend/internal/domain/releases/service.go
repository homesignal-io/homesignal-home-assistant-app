package releases

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var ErrNotFound = errors.New("release record not found")

const (
	ChannelStable    ChannelKey = "stable"
	ChannelCandidate ChannelKey = "candidate"
	ChannelStaging   ChannelKey = "staging"
	ChannelDev       ChannelKey = "dev"

	EnvironmentProduction CloudEnvironment = "production"
	EnvironmentStaging    CloudEnvironment = "staging"

	ChannelStatusActive   ChannelStatus = "active"
	ChannelStatusDisabled ChannelStatus = "disabled"

	ArtifactStatusDraft       ArtifactStatus = "draft"
	ArtifactStatusPublished   ArtifactStatus = "published"
	ArtifactStatusRetired     ArtifactStatus = "retired"
	ArtifactStatusUnsupported ArtifactStatus = "unsupported"

	SupportStatusSupported   SupportStatus = "supported"
	SupportStatusUnsupported SupportStatus = "unsupported"
)

type ChannelKey string
type CloudEnvironment string
type ChannelStatus string
type ArtifactStatus string
type SupportStatus string

type Channel struct {
	ChannelKey       ChannelKey
	CloudEnvironment CloudEnvironment
	ReleaseTrack     string
	HAAppSlug        string
	RepositoryURL    string
	Status           ChannelStatus
	InternalOnly     bool
	Metadata         json.RawMessage
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type Artifact struct {
	ReleaseArtifactID string
	ChannelKey        ChannelKey
	Version           string
	CloudEnvironment  CloudEnvironment
	ReleaseTrack      string
	HAAppSlug         string
	SourceCommitSHA   string
	ImageRef          string
	ImageDigest       string
	PackageDigest     string
	ConfigDigest      string
	Status            ArtifactStatus
	Compatibility     []CompatibilityWindow
	Metadata          json.RawMessage
	PublishedAt       *time.Time
	RetiredAt         *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type CompatibilityWindow struct {
	MinAgentProtocol string
	MaxAgentProtocol string
	MinHAVersion     string
	MaxHAVersion     string
	SupportStatus    SupportStatus
	ReasonCode       string
	Metadata         json.RawMessage
}

type CompatibilityRequest struct {
	ChannelKey       ChannelKey
	InstalledVersion string
	AgentProtocol    string
	HAVersion        string
}

type CompatibilityResult struct {
	ChannelKey       ChannelKey
	InstalledVersion string
	Visible          bool
	Supported        bool
	ReasonCode       string
	Artifact         Artifact
}

type Repository interface {
	SaveChannel(ctx context.Context, channel Channel) error
	GetChannel(ctx context.Context, channelKey ChannelKey) (Channel, error)
	SaveArtifact(ctx context.Context, artifact Artifact) error
	GetArtifactByVersion(ctx context.Context, channelKey ChannelKey, version string) (Artifact, error)
}

type Clock func() time.Time

type Service struct {
	Repository Repository
	Clock      Clock
}

func (s Service) RegisterChannel(ctx context.Context, channel Channel) (Channel, error) {
	if s.Repository == nil {
		return Channel{}, fmt.Errorf("release repository is required")
	}
	channel = normalizeChannel(channel, s.now())
	if err := validateChannel(channel); err != nil {
		return Channel{}, err
	}
	if err := s.Repository.SaveChannel(ctx, channel); err != nil {
		return Channel{}, fmt.Errorf("save release channel: %w", err)
	}
	return channel, nil
}

func (s Service) RegisterArtifact(ctx context.Context, artifact Artifact) (Artifact, error) {
	if s.Repository == nil {
		return Artifact{}, fmt.Errorf("release repository is required")
	}
	channel, err := s.Repository.GetChannel(ctx, artifact.ChannelKey)
	if err != nil {
		return Artifact{}, fmt.Errorf("load release channel: %w", err)
	}
	artifact = normalizeArtifact(artifact, channel, s.now())
	if err := validateArtifact(artifact, channel); err != nil {
		return Artifact{}, err
	}
	if err := s.Repository.SaveArtifact(ctx, artifact); err != nil {
		return Artifact{}, fmt.Errorf("save release artifact: %w", err)
	}
	return artifact, nil
}

func (s Service) CompatibilityForVersion(ctx context.Context, req CompatibilityRequest) (CompatibilityResult, error) {
	if s.Repository == nil {
		return CompatibilityResult{}, fmt.Errorf("release repository is required")
	}
	channelKey := ChannelKey(strings.TrimSpace(string(req.ChannelKey)))
	version := strings.TrimSpace(req.InstalledVersion)
	if channelKey == "" || version == "" {
		return CompatibilityResult{}, fmt.Errorf("channel_key and installed_version are required")
	}
	artifact, err := s.Repository.GetArtifactByVersion(ctx, channelKey, version)
	if errors.Is(err, ErrNotFound) {
		return CompatibilityResult{
			ChannelKey:       channelKey,
			InstalledVersion: version,
			Visible:          true,
			Supported:        false,
			ReasonCode:       "unsupported_version",
		}, nil
	}
	if err != nil {
		return CompatibilityResult{}, fmt.Errorf("load release artifact: %w", err)
	}
	if artifact.Status != ArtifactStatusPublished {
		return CompatibilityResult{
			ChannelKey:       channelKey,
			InstalledVersion: version,
			Visible:          true,
			Supported:        false,
			ReasonCode:       reasonForArtifactStatus(artifact.Status),
			Artifact:         artifact,
		}, nil
	}
	for _, window := range artifact.Compatibility {
		matches, err := matchesCompatibilityWindow(window, req)
		if err != nil {
			return CompatibilityResult{}, err
		}
		if !matches {
			continue
		}
		if window.SupportStatus == SupportStatusUnsupported {
			return CompatibilityResult{
				ChannelKey:       channelKey,
				InstalledVersion: version,
				Visible:          true,
				Supported:        false,
				ReasonCode:       defaultReason(window.ReasonCode, "unsupported_compatibility_window"),
				Artifact:         artifact,
			}, nil
		}
		return CompatibilityResult{
			ChannelKey:       channelKey,
			InstalledVersion: version,
			Visible:          true,
			Supported:        true,
			ReasonCode:       "supported",
			Artifact:         artifact,
		}, nil
	}
	return CompatibilityResult{
		ChannelKey:       channelKey,
		InstalledVersion: version,
		Visible:          true,
		Supported:        false,
		ReasonCode:       "outside_compatibility_window",
		Artifact:         artifact,
	}, nil
}

func normalizeChannel(channel Channel, now time.Time) Channel {
	channel.ChannelKey = ChannelKey(strings.TrimSpace(string(channel.ChannelKey)))
	channel.CloudEnvironment = CloudEnvironment(strings.TrimSpace(string(channel.CloudEnvironment)))
	channel.ReleaseTrack = strings.TrimSpace(channel.ReleaseTrack)
	channel.HAAppSlug = strings.TrimSpace(channel.HAAppSlug)
	channel.RepositoryURL = strings.TrimSpace(channel.RepositoryURL)
	if channel.Status == "" {
		channel.Status = ChannelStatusActive
	}
	channel.Metadata = normalizeJSON(channel.Metadata)
	if channel.CreatedAt.IsZero() {
		channel.CreatedAt = now
	}
	if channel.UpdatedAt.IsZero() {
		channel.UpdatedAt = now
	}
	return channel
}

func normalizeArtifact(artifact Artifact, channel Channel, now time.Time) Artifact {
	artifact.ReleaseArtifactID = strings.TrimSpace(artifact.ReleaseArtifactID)
	artifact.ChannelKey = ChannelKey(strings.TrimSpace(string(artifact.ChannelKey)))
	artifact.Version = strings.TrimSpace(artifact.Version)
	if artifact.CloudEnvironment == "" {
		artifact.CloudEnvironment = channel.CloudEnvironment
	}
	if artifact.ReleaseTrack == "" {
		artifact.ReleaseTrack = channel.ReleaseTrack
	}
	if artifact.HAAppSlug == "" {
		artifact.HAAppSlug = channel.HAAppSlug
	}
	artifact.SourceCommitSHA = strings.TrimSpace(artifact.SourceCommitSHA)
	artifact.ImageRef = strings.TrimSpace(artifact.ImageRef)
	artifact.ImageDigest = strings.TrimSpace(artifact.ImageDigest)
	artifact.PackageDigest = strings.TrimSpace(artifact.PackageDigest)
	artifact.ConfigDigest = strings.TrimSpace(artifact.ConfigDigest)
	if artifact.Status == "" {
		artifact.Status = ArtifactStatusDraft
	}
	artifact.Metadata = normalizeJSON(artifact.Metadata)
	for i := range artifact.Compatibility {
		artifact.Compatibility[i] = normalizeCompatibilityWindow(artifact.Compatibility[i])
	}
	if artifact.CreatedAt.IsZero() {
		artifact.CreatedAt = now
	}
	if artifact.UpdatedAt.IsZero() {
		artifact.UpdatedAt = now
	}
	return artifact
}

func normalizeCompatibilityWindow(window CompatibilityWindow) CompatibilityWindow {
	window.MinAgentProtocol = strings.TrimSpace(window.MinAgentProtocol)
	window.MaxAgentProtocol = strings.TrimSpace(window.MaxAgentProtocol)
	window.MinHAVersion = strings.TrimSpace(window.MinHAVersion)
	window.MaxHAVersion = strings.TrimSpace(window.MaxHAVersion)
	if window.SupportStatus == "" {
		window.SupportStatus = SupportStatusSupported
	}
	window.ReasonCode = strings.TrimSpace(window.ReasonCode)
	window.Metadata = normalizeJSON(window.Metadata)
	return window
}

func validateChannel(channel Channel) error {
	if channel.ChannelKey == "" || channel.CloudEnvironment == "" || channel.ReleaseTrack == "" {
		return fmt.Errorf("channel_key, cloud_environment, and release_track are required")
	}
	if channel.HAAppSlug == "" || channel.RepositoryURL == "" {
		return fmt.Errorf("ha_app_slug and repository_url are required")
	}
	if !json.Valid(channel.Metadata) {
		return fmt.Errorf("channel metadata must be valid JSON")
	}
	if err := validateRepositoryURL(channel.RepositoryURL); err != nil {
		return err
	}
	switch channel.Status {
	case ChannelStatusActive, ChannelStatusDisabled:
	default:
		return fmt.Errorf("unsupported channel status %q", channel.Status)
	}
	switch channel.ChannelKey {
	case ChannelStable, ChannelCandidate:
		if channel.CloudEnvironment != EnvironmentProduction || channel.ReleaseTrack != string(channel.ChannelKey) || channel.InternalOnly {
			return fmt.Errorf("%s channel must be a public production %s track", channel.ChannelKey, channel.ChannelKey)
		}
	case ChannelStaging:
		if channel.CloudEnvironment != EnvironmentStaging || channel.ReleaseTrack != string(ChannelStaging) {
			return fmt.Errorf("staging channel must use the staging cloud environment and staging track")
		}
	case ChannelDev:
		if channel.CloudEnvironment != EnvironmentStaging || channel.ReleaseTrack != string(ChannelDev) || !channel.InternalOnly {
			return fmt.Errorf("dev channel must be internal-only in the staging cloud environment")
		}
	default:
		return fmt.Errorf("unsupported release channel %q", channel.ChannelKey)
	}
	return nil
}

func validateArtifact(artifact Artifact, channel Channel) error {
	if artifact.ReleaseArtifactID == "" || artifact.ChannelKey == "" || artifact.Version == "" {
		return fmt.Errorf("release_artifact_id, channel_key, and version are required")
	}
	if artifact.ChannelKey != channel.ChannelKey {
		return fmt.Errorf("artifact channel does not match loaded channel")
	}
	if artifact.CloudEnvironment != channel.CloudEnvironment || artifact.ReleaseTrack != channel.ReleaseTrack || artifact.HAAppSlug != channel.HAAppSlug {
		return fmt.Errorf("artifact metadata must match release channel assignment")
	}
	if err := validateImmutableVersion(artifact.Version); err != nil {
		return err
	}
	if artifact.SourceCommitSHA == "" || artifact.ImageRef == "" || artifact.ImageDigest == "" {
		return fmt.Errorf("source_commit_sha, image_ref, and image_digest are required")
	}
	if strings.HasSuffix(artifact.ImageRef, ":latest") || strings.Contains(artifact.ImageRef, ":latest@") {
		return fmt.Errorf("release artifact image_ref must not use latest")
	}
	if !strings.HasPrefix(artifact.ImageDigest, "sha256:") {
		return fmt.Errorf("release artifact image_digest must be content-addressed")
	}
	if !json.Valid(artifact.Metadata) {
		return fmt.Errorf("artifact metadata must be valid JSON")
	}
	switch artifact.Status {
	case ArtifactStatusDraft, ArtifactStatusPublished, ArtifactStatusRetired, ArtifactStatusUnsupported:
	default:
		return fmt.Errorf("unsupported artifact status %q", artifact.Status)
	}
	if len(artifact.Compatibility) == 0 {
		return fmt.Errorf("release artifact requires at least one compatibility window")
	}
	for _, window := range artifact.Compatibility {
		if err := validateCompatibilityWindow(window); err != nil {
			return err
		}
	}
	return nil
}

func validateCompatibilityWindow(window CompatibilityWindow) error {
	if !json.Valid(window.Metadata) {
		return fmt.Errorf("compatibility metadata must be valid JSON")
	}
	switch window.SupportStatus {
	case SupportStatusSupported, SupportStatusUnsupported:
	default:
		return fmt.Errorf("unsupported compatibility status %q", window.SupportStatus)
	}
	if err := validateWindowBound("agent protocol", window.MinAgentProtocol, window.MaxAgentProtocol); err != nil {
		return err
	}
	if err := validateWindowBound("Home Assistant version", window.MinHAVersion, window.MaxHAVersion); err != nil {
		return err
	}
	return nil
}

func validateWindowBound(label string, minVersion string, maxVersion string) error {
	if minVersion == "" || maxVersion == "" {
		return nil
	}
	comparison, err := compareDottedVersion(minVersion, maxVersion)
	if err != nil {
		return fmt.Errorf("invalid %s window: %w", label, err)
	}
	if comparison > 0 {
		return fmt.Errorf("invalid %s window: min exceeds max", label)
	}
	return nil
}

func validateRepositoryURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("repository_url must be a valid URL")
	}
	if parsed.Scheme != "https" || parsed.Host == "" {
		return fmt.Errorf("repository_url must be an HTTPS URL")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("repository_url must not include query or fragment data")
	}
	return nil
}

func validateImmutableVersion(version string) error {
	if version == "" {
		return fmt.Errorf("version is required")
	}
	lower := strings.ToLower(version)
	if lower == "latest" || lower == "edge" || lower == "main" || lower == "master" {
		return fmt.Errorf("release version must be immutable, got %q", version)
	}
	if strings.ContainsAny(version, " \t\n\r/\\") {
		return fmt.Errorf("release version contains unsupported characters")
	}
	return nil
}

func matchesCompatibilityWindow(window CompatibilityWindow, req CompatibilityRequest) (bool, error) {
	if ok, err := versionWithin(req.AgentProtocol, window.MinAgentProtocol, window.MaxAgentProtocol); err != nil || !ok {
		return ok, err
	}
	if ok, err := versionWithin(req.HAVersion, window.MinHAVersion, window.MaxHAVersion); err != nil || !ok {
		return ok, err
	}
	return true, nil
}

func versionWithin(value string, minVersion string, maxVersion string) (bool, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		if minVersion != "" || maxVersion != "" {
			return false, nil
		}
		return true, nil
	}
	if minVersion != "" {
		comparison, err := compareDottedVersion(value, minVersion)
		if err != nil {
			return false, err
		}
		if comparison < 0 {
			return false, nil
		}
	}
	if maxVersion != "" {
		comparison, err := compareDottedVersion(value, maxVersion)
		if err != nil {
			return false, err
		}
		if comparison > 0 {
			return false, nil
		}
	}
	return true, nil
}

func compareDottedVersion(left string, right string) (int, error) {
	leftParts, err := parseDottedVersion(left)
	if err != nil {
		return 0, err
	}
	rightParts, err := parseDottedVersion(right)
	if err != nil {
		return 0, err
	}
	maxLen := len(leftParts)
	if len(rightParts) > maxLen {
		maxLen = len(rightParts)
	}
	for i := 0; i < maxLen; i++ {
		leftValue := 0
		if i < len(leftParts) {
			leftValue = leftParts[i]
		}
		rightValue := 0
		if i < len(rightParts) {
			rightValue = rightParts[i]
		}
		switch {
		case leftValue < rightValue:
			return -1, nil
		case leftValue > rightValue:
			return 1, nil
		}
	}
	return 0, nil
}

func parseDottedVersion(version string) ([]int, error) {
	version = strings.TrimPrefix(strings.TrimSpace(version), "v")
	if idx := strings.IndexAny(version, "-+"); idx >= 0 {
		version = version[:idx]
	}
	if version == "" {
		return nil, fmt.Errorf("empty version")
	}
	parts := strings.Split(version, ".")
	values := make([]int, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			return nil, fmt.Errorf("invalid version %q", version)
		}
		value, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("invalid version %q", version)
		}
		values = append(values, value)
	}
	return values, nil
}

func reasonForArtifactStatus(status ArtifactStatus) string {
	switch status {
	case ArtifactStatusDraft:
		return "release_not_published"
	case ArtifactStatusRetired:
		return "release_retired"
	case ArtifactStatusUnsupported:
		return "unsupported_version"
	default:
		return "release_unavailable"
	}
}

func defaultReason(reason string, fallback string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return fallback
	}
	return reason
}

func normalizeJSON(value json.RawMessage) json.RawMessage {
	if len(value) == 0 {
		return json.RawMessage(`{}`)
	}
	return value
}

func (s Service) now() time.Time {
	if s.Clock != nil {
		return s.Clock().UTC()
	}
	return time.Now().UTC()
}
