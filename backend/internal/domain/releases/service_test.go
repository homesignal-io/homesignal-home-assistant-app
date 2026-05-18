package releases

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestServiceRegistersProductionCandidateChannel(t *testing.T) {
	repo := newFakeRepository()
	service := Service{Repository: repo, Clock: fixedClock}

	channel, err := service.RegisterChannel(context.Background(), Channel{
		ChannelKey:       ChannelCandidate,
		CloudEnvironment: EnvironmentProduction,
		ReleaseTrack:     "candidate",
		HAAppSlug:        "homesignal_manager_candidate",
		RepositoryURL:    "https://github.com/homesignal-io/homesignal-home-assistant-app-candidate",
	})
	if err != nil {
		t.Fatalf("register candidate channel: %v", err)
	}
	if channel.Status != ChannelStatusActive {
		t.Fatalf("expected active default status, got %q", channel.Status)
	}
	if repo.channels[ChannelCandidate].HAAppSlug != "homesignal_manager_candidate" {
		t.Fatalf("channel was not stored")
	}
}

func TestServiceRejectsInvalidChannelAssignment(t *testing.T) {
	repo := newFakeRepository()
	service := Service{Repository: repo, Clock: fixedClock}

	_, err := service.RegisterChannel(context.Background(), Channel{
		ChannelKey:       ChannelStable,
		CloudEnvironment: EnvironmentStaging,
		ReleaseTrack:     "stable",
		HAAppSlug:        "homesignal_manager",
		RepositoryURL:    "https://github.com/homesignal-io/homesignal-home-assistant-app",
	})
	if err == nil || !strings.Contains(err.Error(), "production") {
		t.Fatalf("expected channel assignment validation error, got %v", err)
	}
}

func TestServiceRegistersImmutableArtifactWithCompatibilityWindow(t *testing.T) {
	repo := newFakeRepository()
	service := Service{Repository: repo, Clock: fixedClock}
	repo.channels[ChannelStable] = stableChannel()

	artifact, err := service.RegisterArtifact(context.Background(), Artifact{
		ReleaseArtifactID: "rel_art_123",
		ChannelKey:        ChannelStable,
		Version:           "0.1.4",
		SourceCommitSHA:   "abcdef1234567890",
		ImageRef:          "ghcr.io/homesignal-io/homesignal-manager@sha256:aaaaaaaa",
		ImageDigest:       "sha256:aaaaaaaa",
		Status:            ArtifactStatusPublished,
		Compatibility: []CompatibilityWindow{
			{
				MinAgentProtocol: "1",
				MaxAgentProtocol: "1",
				MinHAVersion:     "2026.5.0",
				SupportStatus:    SupportStatusSupported,
			},
		},
	})
	if err != nil {
		t.Fatalf("register artifact: %v", err)
	}
	if artifact.CloudEnvironment != EnvironmentProduction {
		t.Fatalf("expected artifact to inherit channel environment, got %q", artifact.CloudEnvironment)
	}
	if len(repo.artifacts) != 1 {
		t.Fatalf("expected stored artifact, got %d", len(repo.artifacts))
	}
}

func TestServiceRejectsFloatingArtifactVersionAndLatestImage(t *testing.T) {
	repo := newFakeRepository()
	service := Service{Repository: repo, Clock: fixedClock}
	repo.channels[ChannelStable] = stableChannel()

	_, err := service.RegisterArtifact(context.Background(), Artifact{
		ReleaseArtifactID: "rel_art_123",
		ChannelKey:        ChannelStable,
		Version:           "latest",
		SourceCommitSHA:   "abcdef1234567890",
		ImageRef:          "ghcr.io/homesignal-io/homesignal-manager:latest",
		ImageDigest:       "sha256:aaaaaaaa",
		Status:            ArtifactStatusPublished,
		Compatibility: []CompatibilityWindow{
			{SupportStatus: SupportStatusSupported},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "immutable") {
		t.Fatalf("expected immutable version error, got %v", err)
	}
}

func TestCompatibilityForVersionReturnsVisibleUnsupportedVersion(t *testing.T) {
	repo := newFakeRepository()
	service := Service{Repository: repo, Clock: fixedClock}

	result, err := service.CompatibilityForVersion(context.Background(), CompatibilityRequest{
		ChannelKey:       ChannelStable,
		InstalledVersion: "0.0.9",
		AgentProtocol:    "1",
		HAVersion:        "2026.5.1",
	})
	if err != nil {
		t.Fatalf("compatibility: %v", err)
	}
	if !result.Visible || result.Supported || result.ReasonCode != "unsupported_version" {
		t.Fatalf("unexpected compatibility result: %#v", result)
	}
}

func TestCompatibilityForVersionChecksProtocolAndHAVersionWindow(t *testing.T) {
	repo := newFakeRepository()
	service := Service{Repository: repo, Clock: fixedClock}
	artifact := publishedArtifact()
	repo.artifacts[artifactKey(artifact.ChannelKey, artifact.Version)] = artifact

	supported, err := service.CompatibilityForVersion(context.Background(), CompatibilityRequest{
		ChannelKey:       ChannelStable,
		InstalledVersion: "0.1.4",
		AgentProtocol:    "1",
		HAVersion:        "2026.5.1",
	})
	if err != nil {
		t.Fatalf("compatibility supported: %v", err)
	}
	if !supported.Supported || supported.ReasonCode != "supported" {
		t.Fatalf("expected supported compatibility, got %#v", supported)
	}

	blocked, err := service.CompatibilityForVersion(context.Background(), CompatibilityRequest{
		ChannelKey:       ChannelStable,
		InstalledVersion: "0.1.4",
		AgentProtocol:    "2",
		HAVersion:        "2026.5.1",
	})
	if err != nil {
		t.Fatalf("compatibility blocked: %v", err)
	}
	if blocked.Supported || blocked.ReasonCode != "outside_compatibility_window" {
		t.Fatalf("expected outside window, got %#v", blocked)
	}
}

func fixedClock() time.Time {
	return time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
}

func stableChannel() Channel {
	return Channel{
		ChannelKey:       ChannelStable,
		CloudEnvironment: EnvironmentProduction,
		ReleaseTrack:     "stable",
		HAAppSlug:        "homesignal_manager",
		RepositoryURL:    "https://github.com/homesignal-io/homesignal-home-assistant-app",
		Status:           ChannelStatusActive,
	}
}

func publishedArtifact() Artifact {
	return Artifact{
		ReleaseArtifactID: "rel_art_123",
		ChannelKey:        ChannelStable,
		Version:           "0.1.4",
		CloudEnvironment:  EnvironmentProduction,
		ReleaseTrack:      "stable",
		HAAppSlug:         "homesignal_manager",
		SourceCommitSHA:   "abcdef1234567890",
		ImageRef:          "ghcr.io/homesignal-io/homesignal-manager@sha256:aaaaaaaa",
		ImageDigest:       "sha256:aaaaaaaa",
		Status:            ArtifactStatusPublished,
		Compatibility: []CompatibilityWindow{
			{
				MinAgentProtocol: "1",
				MaxAgentProtocol: "1",
				MinHAVersion:     "2026.5.0",
				MaxHAVersion:     "2026.6.99",
				SupportStatus:    SupportStatusSupported,
			},
		},
	}
}

type fakeRepository struct {
	channels  map[ChannelKey]Channel
	artifacts map[string]Artifact
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{
		channels:  map[ChannelKey]Channel{},
		artifacts: map[string]Artifact{},
	}
}

func (r *fakeRepository) SaveChannel(_ context.Context, channel Channel) error {
	r.channels[channel.ChannelKey] = channel
	return nil
}

func (r *fakeRepository) GetChannel(_ context.Context, channelKey ChannelKey) (Channel, error) {
	channel, ok := r.channels[channelKey]
	if !ok {
		return Channel{}, ErrNotFound
	}
	return channel, nil
}

func (r *fakeRepository) SaveArtifact(_ context.Context, artifact Artifact) error {
	r.artifacts[artifactKey(artifact.ChannelKey, artifact.Version)] = artifact
	return nil
}

func (r *fakeRepository) GetArtifactByVersion(_ context.Context, channelKey ChannelKey, version string) (Artifact, error) {
	artifact, ok := r.artifacts[artifactKey(channelKey, version)]
	if !ok {
		return Artifact{}, ErrNotFound
	}
	if artifact.ReleaseArtifactID == "explode" {
		return Artifact{}, errors.New("boom")
	}
	return artifact, nil
}

func artifactKey(channelKey ChannelKey, version string) string {
	return string(channelKey) + ":" + version
}
