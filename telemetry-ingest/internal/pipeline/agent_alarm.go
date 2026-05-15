package pipeline

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type AgentAlarmHandler struct{}

func (AgentAlarmHandler) Key() SchemaKey {
	return SchemaKey{
		MessageType: MessageTypeEvent,
		SchemaType:  SchemaTypeAgentAlarm,
		Version:     RuntimeSchemaVersionV1,
	}
}

func (AgentAlarmHandler) Validate(envelope RuntimeEnvelope) (Projection, error) {
	var payload AgentAlarmPayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return Projection{}, fmt.Errorf("decode agent_alarm payload: %w", err)
	}
	if err := payload.Validate(); err != nil {
		return Projection{}, err
	}
	return Projection{
		Material: map[string]string{
			"alarm_type": payload.AlarmType,
			"severity":   payload.Severity,
		},
		Sidecar: map[string]string{
			"reason_code": payload.ReasonCode,
		},
	}, nil
}

type AgentAlarmPayload struct {
	AlarmType                  string    `json:"alarm_type"`
	Severity                   string    `json:"severity"`
	ReasonCode                 string    `json:"reason_code"`
	OccurrenceCount            int       `json:"occurrence_count"`
	FirstSeenAt                time.Time `json:"first_seen_at"`
	LastSeenAt                 time.Time `json:"last_seen_at"`
	AttemptedPolicyVersion     string    `json:"attempted_policy_version,omitempty"`
	ActivePolicyVersion        string    `json:"active_policy_version,omitempty"`
	CommandID                  string    `json:"command_id,omitempty"`
	ArtifactPurpose            string    `json:"artifact_purpose,omitempty"`
	DiagnosticExcerpt          *string   `json:"diagnostic_excerpt,omitempty"`
	DiagnosticExcerptTruncated bool      `json:"diagnostic_excerpt_truncated"`
	MoreLogsAvailable          bool      `json:"more_logs_available"`
	LocalArtifactRef           string    `json:"local_artifact_ref,omitempty"`
	SuppressedCount            int       `json:"suppressed_count,omitempty"`
}

func (p AgentAlarmPayload) Validate() error {
	if !validAgentAlarmType(p.AlarmType) {
		return fmt.Errorf("payload.alarm_type is invalid")
	}
	if !validAgentAlarmSeverity(p.Severity) {
		return fmt.Errorf("payload.severity is invalid")
	}
	if strings.TrimSpace(p.ReasonCode) == "" {
		return fmt.Errorf("payload.reason_code is required")
	}
	if p.OccurrenceCount <= 0 {
		return fmt.Errorf("payload.occurrence_count must be positive")
	}
	if p.FirstSeenAt.IsZero() || p.LastSeenAt.IsZero() {
		return fmt.Errorf("payload first_seen_at and last_seen_at are required")
	}
	if p.LastSeenAt.Before(p.FirstSeenAt) {
		return fmt.Errorf("payload.last_seen_at cannot be before first_seen_at")
	}
	if p.DiagnosticExcerpt != nil && len([]byte(*p.DiagnosticExcerpt)) > maxDiagnosticExcerptBytes {
		return fmt.Errorf("payload.diagnostic_excerpt exceeds %d bytes", maxDiagnosticExcerptBytes)
	}
	if p.MoreLogsAvailable && strings.TrimSpace(p.LocalArtifactRef) == "" {
		return fmt.Errorf("payload.local_artifact_ref is required when more_logs_available is true")
	}
	if p.AlarmType == "artifact_upload_failed" && p.MoreLogsAvailable {
		return fmt.Errorf("payload.more_logs_available must be false for artifact_upload_failed")
	}
	if p.SuppressedCount < 0 {
		return fmt.Errorf("payload.suppressed_count cannot be negative")
	}
	return nil
}

func validAgentAlarmType(value string) bool {
	switch value {
	case "potential_abuse_detected", "publish_policy_apply_failed", "publish_policy_rejected_suspicious", "artifact_upload_failed":
		return true
	default:
		return false
	}
}

func validAgentAlarmSeverity(value string) bool {
	switch value {
	case "info", "warning", "critical":
		return true
	default:
		return false
	}
}
