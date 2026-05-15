package pipeline

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const maxDiagnosticExcerptBytes = 5 * 1024

type DeviceHealthSnapshotHandler struct{}

func (DeviceHealthSnapshotHandler) Key() SchemaKey {
	return SchemaKey{
		MessageType: MessageTypeTelemetry,
		SchemaType:  SchemaTypeDeviceHealthSnapshot,
		Version:     RuntimeSchemaVersionV1,
	}
}

func (DeviceHealthSnapshotHandler) Validate(envelope RuntimeEnvelope) (Projection, error) {
	var payload DeviceHealthSnapshotPayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return Projection{}, fmt.Errorf("decode device.health_snapshot payload: %w", err)
	}
	if err := payload.Validate(envelope.AppliedPublishPolicyVersion); err != nil {
		return Projection{}, err
	}
	return Projection{
		Material: map[string]string{
			"agent.status":                 payload.Agent.Status,
			"home_assistant.core.status":   payload.HomeAssistant.Core.Status,
			"home_assistant.backup.status": payload.HomeAssistant.Backup.Status,
		},
		Sidecar: map[string]string{
			"runtime_log_summary.count": fmt.Sprintf("%d", len(payload.RuntimeLogSummary)),
			"addons.count":              fmt.Sprintf("%d", len(payload.Addons)),
		},
	}, nil
}

type DeviceHealthSnapshotPayload struct {
	Agent             AgentSnapshot            `json:"agent"`
	HomeAssistant     HomeAssistantSnapshot    `json:"home_assistant"`
	Addons            []AddonSnapshot          `json:"addons,omitempty"`
	RuntimeLogSummary []RuntimeLogSummaryEntry `json:"runtime_log_summary,omitempty"`
}

type AgentSnapshot struct {
	Status            string                `json:"status"`
	Version           string                `json:"version"`
	UptimeSeconds     int64                 `json:"uptime_seconds,omitempty"`
	CloudConnection   CloudConnectionStatus `json:"cloud_connection"`
	PublishPolicy     PublishPolicyStatus   `json:"publish_policy"`
	Update            UpdateStatus          `json:"update"`
	SuppressionCounts []SuppressionCount    `json:"suppression_counts,omitempty"`
}

type CloudConnectionStatus struct {
	AgentHTTPSReachable bool       `json:"agent_https_reachable"`
	AWSIoTConnected     bool       `json:"aws_iot_connected"`
	LastSuccessAt       *time.Time `json:"last_success_at,omitempty"`
	LastFailureAt       *time.Time `json:"last_failure_at,omitempty"`
}

type PublishPolicyStatus struct {
	AppliedVersion string    `json:"applied_version"`
	IssuedAt       time.Time `json:"issued_at"`
	RefreshAfter   time.Time `json:"refresh_after"`
	ExpiresAt      time.Time `json:"expires_at"`
	Status         string    `json:"status"`
}

type UpdateStatus struct {
	Status            string     `json:"status"`
	CurrentVersion    string     `json:"current_version,omitempty"`
	DesiredVersion    string     `json:"desired_version,omitempty"`
	LatestVersion     string     `json:"latest_version,omitempty"`
	Channel           string     `json:"channel,omitempty"`
	LastCheckedAt     *time.Time `json:"last_checked_at,omitempty"`
	LastFailureReason *string    `json:"last_failure_reason,omitempty"`
}

type SuppressionCount struct {
	Signal          string    `json:"signal"`
	Reason          string    `json:"reason"`
	Count           int       `json:"count"`
	WindowStartedAt time.Time `json:"window_started_at"`
	WindowEndedAt   time.Time `json:"window_ended_at"`
}

type HomeAssistantSnapshot struct {
	Core       HAComponentStatus `json:"core"`
	Supervisor HAComponentStatus `json:"supervisor"`
	Backup     HABackupStatus    `json:"backup"`
	Storage    HAStorageStatus   `json:"storage"`
}

type HAComponentStatus struct {
	Status     string     `json:"status"`
	Version    string     `json:"version,omitempty"`
	LastSeenAt *time.Time `json:"last_seen_at,omitempty"`
}

type HABackupStatus struct {
	Status            string     `json:"status"`
	LastSuccessAt     *time.Time `json:"last_success_at,omitempty"`
	LastFailureAt     *time.Time `json:"last_failure_at,omitempty"`
	LastFailureReason *string    `json:"last_failure_reason,omitempty"`
	InProgress        bool       `json:"in_progress"`
}

type HAStorageStatus struct {
	Status        string     `json:"status"`
	UsedPercent   *float64   `json:"used_percent,omitempty"`
	FreeBytes     *int64     `json:"free_bytes,omitempty"`
	LastCheckedAt *time.Time `json:"last_checked_at,omitempty"`
}

type AddonSnapshot struct {
	AddonID     *string       `json:"addon_id,omitempty"`
	Slug        string        `json:"slug"`
	DisplayName string        `json:"display_name,omitempty"`
	Repository  string        `json:"repository,omitempty"`
	Status      string        `json:"status"`
	Version     string        `json:"version,omitempty"`
	Enabled     bool          `json:"enabled"`
	LastSeenAt  *time.Time    `json:"last_seen_at,omitempty"`
	Update      UpdateStatus  `json:"update,omitempty"`
	Health      AddonHealth   `json:"health,omitempty"`
	Activity    AddonActivity `json:"activity,omitempty"`
}

type AddonHealth struct {
	State   string   `json:"state,omitempty"`
	Reasons []string `json:"reasons,omitempty"`
}

type AddonActivity struct {
	EventsProcessedLastHour *int       `json:"events_processed_last_hour,omitempty"`
	LastEventAt             *time.Time `json:"last_event_at,omitempty"`
}

type RuntimeLogSummaryEntry struct {
	Level                      string    `json:"level"`
	Source                     string    `json:"source"`
	Component                  string    `json:"component"`
	ReasonCode                 string    `json:"reason_code"`
	OccurrenceCount            int       `json:"occurrence_count"`
	FirstSeenAt                time.Time `json:"first_seen_at"`
	LastSeenAt                 time.Time `json:"last_seen_at"`
	SampleMessage              string    `json:"sample_message,omitempty"`
	DiagnosticExcerpt          *string   `json:"diagnostic_excerpt,omitempty"`
	DiagnosticExcerptTruncated bool      `json:"diagnostic_excerpt_truncated"`
	MoreLogsAvailable          bool      `json:"more_logs_available"`
	LocalArtifactRef           string    `json:"local_artifact_ref,omitempty"`
	SuppressedCount            int       `json:"suppressed_count,omitempty"`
}

func (p DeviceHealthSnapshotPayload) Validate(appliedPolicyVersion string) error {
	if strings.TrimSpace(p.Agent.Status) == "" {
		return fmt.Errorf("payload.agent.status is required")
	}
	if strings.TrimSpace(p.Agent.Version) == "" {
		return fmt.Errorf("payload.agent.version is required")
	}
	if strings.TrimSpace(p.Agent.PublishPolicy.AppliedVersion) == "" {
		return fmt.Errorf("payload.agent.publish_policy.applied_version is required")
	}
	if p.Agent.PublishPolicy.AppliedVersion != appliedPolicyVersion {
		return fmt.Errorf("payload.agent.publish_policy.applied_version must match envelope applied_publish_policy_version")
	}
	if strings.TrimSpace(p.HomeAssistant.Core.Status) == "" {
		return fmt.Errorf("payload.home_assistant.core.status is required")
	}
	if strings.TrimSpace(p.HomeAssistant.Supervisor.Status) == "" {
		return fmt.Errorf("payload.home_assistant.supervisor.status is required")
	}
	if strings.TrimSpace(p.HomeAssistant.Backup.Status) == "" {
		return fmt.Errorf("payload.home_assistant.backup.status is required")
	}
	if strings.TrimSpace(p.HomeAssistant.Storage.Status) == "" {
		return fmt.Errorf("payload.home_assistant.storage.status is required")
	}

	for index, addon := range p.Addons {
		if strings.TrimSpace(addon.Slug) == "" && (addon.AddonID == nil || strings.TrimSpace(*addon.AddonID) == "") {
			return fmt.Errorf("payload.addons[%d] requires slug or addon_id", index)
		}
		if strings.TrimSpace(addon.Status) == "" {
			return fmt.Errorf("payload.addons[%d].status is required", index)
		}
	}

	for index, entry := range p.RuntimeLogSummary {
		if err := entry.Validate(index); err != nil {
			return err
		}
	}
	return nil
}

func (e RuntimeLogSummaryEntry) Validate(index int) error {
	if !validRuntimeLogLevel(e.Level) {
		return fmt.Errorf("payload.runtime_log_summary[%d].level is invalid", index)
	}
	if strings.TrimSpace(e.Source) == "" {
		return fmt.Errorf("payload.runtime_log_summary[%d].source is required", index)
	}
	if strings.TrimSpace(e.Component) == "" {
		return fmt.Errorf("payload.runtime_log_summary[%d].component is required", index)
	}
	if strings.TrimSpace(e.ReasonCode) == "" {
		return fmt.Errorf("payload.runtime_log_summary[%d].reason_code is required", index)
	}
	if e.OccurrenceCount <= 0 {
		return fmt.Errorf("payload.runtime_log_summary[%d].occurrence_count must be positive", index)
	}
	if e.FirstSeenAt.IsZero() || e.LastSeenAt.IsZero() {
		return fmt.Errorf("payload.runtime_log_summary[%d] first_seen_at and last_seen_at are required", index)
	}
	if e.LastSeenAt.Before(e.FirstSeenAt) {
		return fmt.Errorf("payload.runtime_log_summary[%d].last_seen_at cannot be before first_seen_at", index)
	}
	if e.DiagnosticExcerpt != nil && len([]byte(*e.DiagnosticExcerpt)) > maxDiagnosticExcerptBytes {
		return fmt.Errorf("payload.runtime_log_summary[%d].diagnostic_excerpt exceeds %d bytes", index, maxDiagnosticExcerptBytes)
	}
	if e.MoreLogsAvailable && strings.TrimSpace(e.LocalArtifactRef) == "" {
		return fmt.Errorf("payload.runtime_log_summary[%d].local_artifact_ref is required when more_logs_available is true", index)
	}
	if e.SuppressedCount < 0 {
		return fmt.Errorf("payload.runtime_log_summary[%d].suppressed_count cannot be negative", index)
	}
	return nil
}

func validRuntimeLogLevel(value string) bool {
	switch value {
	case "info", "warning", "error", "critical":
		return true
	default:
		return false
	}
}
