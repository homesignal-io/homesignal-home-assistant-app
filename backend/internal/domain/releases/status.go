package releases

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/homesignal-io/homesignal-home-assistant-app/backend/internal/domain/commands"
	"github.com/homesignal-io/homesignal-home-assistant-app/backend/internal/domain/edgestate"
)

const (
	UpdateStatusCurrent    UpdateStatus = "current"
	UpdateStatusPending    UpdateStatus = "pending"
	UpdateStatusBlocked    UpdateStatus = "blocked"
	UpdateStatusFailed     UpdateStatus = "failed"
	UpdateStatusRolledBack UpdateStatus = "rolled_back"
	UpdateStatusUnknown    UpdateStatus = "unknown"
)

type UpdateStatus string

type DeviceUpdateStatus struct {
	AccountID              string
	SiteID                 string
	DeviceID               string
	ChannelKey             ChannelKey
	HAAppSlug              string
	HAAppRepositorySource  string
	CloudEnvironment       CloudEnvironment
	ReleaseTrack           string
	InstalledVersion       string
	DesiredVersion         string
	LatestAvailableVersion string
	UpdateAvailable        *bool
	AutoUpdateEnabled      *bool
	Status                 UpdateStatus
	ReasonCode             string
	Metadata               json.RawMessage
	ReportedAt             time.Time
	ReceivedAt             time.Time
}

type UpdateStatusReport struct {
	AccountID              string
	SiteID                 string
	DeviceID               string
	ChannelKey             ChannelKey
	HAAppSlug              string
	HAAppRepositorySource  string
	CloudEnvironment       CloudEnvironment
	ReleaseTrack           string
	InstalledVersion       string
	DesiredVersion         string
	LatestAvailableVersion string
	UpdateAvailable        *bool
	AutoUpdateEnabled      *bool
	Status                 UpdateStatus
	ReasonCode             string
	Metadata               json.RawMessage
	ReportedAt             time.Time
}

type UpdateCommandRequest struct {
	AccountID         string
	SiteID            string
	DeviceID          string
	RequestedByUserID string
	IdempotencyKey    string
	ReasonCode        string
	DesiredVersion    string
	ChannelKey        ChannelKey
	RolloutID         string
}

type UpdateCommandCreator interface {
	Create(ctx context.Context, req commands.CreateRequest) (commands.Command, error)
}

func (s UpdateService) RecordReportedUpdateStatus(ctx context.Context, report UpdateStatusReport) (DeviceUpdateStatus, error) {
	if s.Repository == nil {
		return DeviceUpdateStatus{}, fmt.Errorf("update repository is required")
	}
	now := s.now()
	if report.ReportedAt.IsZero() {
		report.ReportedAt = now
	}
	status := DeviceUpdateStatus{
		AccountID:              strings.TrimSpace(report.AccountID),
		SiteID:                 strings.TrimSpace(report.SiteID),
		DeviceID:               strings.TrimSpace(report.DeviceID),
		ChannelKey:             ChannelKey(strings.TrimSpace(string(report.ChannelKey))),
		HAAppSlug:              strings.TrimSpace(report.HAAppSlug),
		HAAppRepositorySource:  strings.TrimSpace(report.HAAppRepositorySource),
		CloudEnvironment:       CloudEnvironment(strings.TrimSpace(string(report.CloudEnvironment))),
		ReleaseTrack:           strings.TrimSpace(report.ReleaseTrack),
		InstalledVersion:       strings.TrimSpace(report.InstalledVersion),
		DesiredVersion:         strings.TrimSpace(report.DesiredVersion),
		LatestAvailableVersion: strings.TrimSpace(report.LatestAvailableVersion),
		UpdateAvailable:        report.UpdateAvailable,
		AutoUpdateEnabled:      report.AutoUpdateEnabled,
		Status:                 report.Status,
		ReasonCode:             strings.TrimSpace(report.ReasonCode),
		Metadata:               normalizeJSON(report.Metadata),
		ReportedAt:             report.ReportedAt.UTC(),
		ReceivedAt:             now,
	}
	if err := validateDeviceUpdateStatus(status); err != nil {
		return DeviceUpdateStatus{}, err
	}
	if err := s.Repository.UpsertDeviceUpdateStatus(ctx, status); err != nil {
		return DeviceUpdateStatus{}, fmt.Errorf("upsert device update status: %w", err)
	}
	projectionPayload, err := marshalUpdateProjection(status)
	if err != nil {
		return DeviceUpdateStatus{}, err
	}
	if err := s.Repository.UpsertUpdateProjection(ctx, edgestate.Projection{
		DeviceID:          status.DeviceID,
		ProjectionKey:     edgestate.StateKeyUpdate,
		DesiredVersion:    status.DesiredVersion,
		ReportedVersion:   status.InstalledVersion,
		Projection:        projectionPayload,
		ConvergenceStatus: convergenceStatusForUpdate(status),
		LastReportedAt:    &status.ReportedAt,
	}); err != nil {
		return DeviceUpdateStatus{}, fmt.Errorf("upsert update projection: %w", err)
	}
	return status, nil
}

func (s UpdateService) RequestUpdateStatusCheck(ctx context.Context, req UpdateCommandRequest) (commands.Command, error) {
	return s.createUpdateCommand(ctx, req, commands.TypeCheckUpdateStatus, map[string]any{
		"operation":   "check_update_status",
		"reason_code": strings.TrimSpace(req.ReasonCode),
	})
}

func (s UpdateService) RequestUpdateRepair(ctx context.Context, req UpdateCommandRequest) (commands.Command, error) {
	if strings.TrimSpace(req.DesiredVersion) == "" || strings.TrimSpace(string(req.ChannelKey)) == "" {
		return commands.Command{}, fmt.Errorf("desired_version and channel_key are required for update repair")
	}
	if err := validateImmutableVersion(req.DesiredVersion); err != nil {
		return commands.Command{}, err
	}
	return s.createUpdateCommand(ctx, req, commands.TypeRepairUpdateState, map[string]any{
		"operation":       "repair_update_state",
		"desired_version": strings.TrimSpace(req.DesiredVersion),
		"channel":         strings.TrimSpace(string(req.ChannelKey)),
		"rollout_id":      strings.TrimSpace(req.RolloutID),
		"reason_code":     strings.TrimSpace(req.ReasonCode),
	})
}

func (s UpdateService) createUpdateCommand(ctx context.Context, req UpdateCommandRequest, commandType commands.CommandType, payload map[string]any) (commands.Command, error) {
	if s.CommandCreator == nil {
		return commands.Command{}, fmt.Errorf("update command creator is required")
	}
	if err := validateUpdateCommandRequest(req); err != nil {
		return commands.Command{}, err
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return commands.Command{}, fmt.Errorf("marshal update command payload: %w", err)
	}
	return s.CommandCreator.Create(ctx, commands.CreateRequest{
		AccountID:         strings.TrimSpace(req.AccountID),
		SiteID:            strings.TrimSpace(req.SiteID),
		DeviceID:          strings.TrimSpace(req.DeviceID),
		CommandType:       commandType,
		IdempotencyKey:    strings.TrimSpace(req.IdempotencyKey),
		RequestedByUserID: strings.TrimSpace(req.RequestedByUserID),
		Payload:           payloadJSON,
	})
}

func validateDeviceUpdateStatus(status DeviceUpdateStatus) error {
	if status.AccountID == "" || status.SiteID == "" || status.DeviceID == "" {
		return fmt.Errorf("account_id, site_id, and device_id are required")
	}
	if status.Status == "" {
		return fmt.Errorf("update status is required")
	}
	switch status.Status {
	case UpdateStatusCurrent, UpdateStatusPending, UpdateStatusBlocked, UpdateStatusFailed, UpdateStatusRolledBack, UpdateStatusUnknown:
	default:
		return fmt.Errorf("unsupported update status %q", status.Status)
	}
	switch status.CloudEnvironment {
	case "", EnvironmentProduction, EnvironmentStaging:
	default:
		return fmt.Errorf("unsupported cloud environment %q", status.CloudEnvironment)
	}
	if status.ReleaseTrack != "" {
		switch status.ReleaseTrack {
		case "stable", "candidate", "staging", "dev":
		default:
			return fmt.Errorf("unsupported release track %q", status.ReleaseTrack)
		}
	}
	if !json.Valid(status.Metadata) {
		return fmt.Errorf("update status metadata must be valid JSON")
	}
	return nil
}

func validateUpdateCommandRequest(req UpdateCommandRequest) error {
	if strings.TrimSpace(req.AccountID) == "" || strings.TrimSpace(req.SiteID) == "" || strings.TrimSpace(req.DeviceID) == "" {
		return fmt.Errorf("account_id, site_id, and device_id are required")
	}
	return nil
}

func marshalUpdateProjection(status DeviceUpdateStatus) (json.RawMessage, error) {
	payload, err := json.Marshal(map[string]any{
		"update": map[string]any{
			"current_version":          emptyToNil(status.InstalledVersion),
			"desired_version":          emptyToNil(status.DesiredVersion),
			"latest_available_version": emptyToNil(status.LatestAvailableVersion),
			"channel":                  emptyToNil(string(status.ChannelKey)),
			"status":                   status.Status,
			"reason_code":              emptyToNil(status.ReasonCode),
			"reported_at":              status.ReportedAt.Format(time.RFC3339),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal update projection: %w", err)
	}
	return payload, nil
}

func convergenceStatusForUpdate(status DeviceUpdateStatus) string {
	switch status.Status {
	case UpdateStatusCurrent:
		if status.DesiredVersion == "" || status.InstalledVersion == status.DesiredVersion {
			return "converged"
		}
		return "pending"
	case UpdateStatusPending:
		return "pending"
	case UpdateStatusBlocked, UpdateStatusFailed, UpdateStatusRolledBack:
		return "failed"
	default:
		return "unknown"
	}
}

func emptyToNil(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
