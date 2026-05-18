package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/homesignal-io/homesignal-home-assistant-app/telemetry-ingest/internal/pipeline"
)

func (w Writer) WriteLifecycle(ctx context.Context, event pipeline.LifecycleEvent) (pipeline.LifecycleResult, error) {
	if w.Pool == nil {
		return pipeline.LifecycleResult{}, fmt.Errorf("postgres pool is required")
	}
	event.EventType = strings.TrimSpace(event.EventType)
	event.ClientID = strings.TrimSpace(event.ClientID)
	if event.EventType == "" || event.ClientID == "" {
		return pipeline.LifecycleResult{}, fmt.Errorf("lifecycle event_type and client_id are required")
	}
	if event.ObservedAt.IsZero() {
		event.ObservedAt = time.Now().UTC()
	}
	if event.ReceivedAt.IsZero() {
		event.ReceivedAt = time.Now().UTC()
	}

	var deviceID string
	if err := w.Pool.QueryRow(ctx, `
SELECT device_id
FROM devices
WHERE device_id = $1
  AND claim_state = 'CLAIMED'
  AND revoked_at IS NULL
`, event.ClientID).Scan(&deviceID); err != nil {
		_ = w.RecordFailure(ctx, pipeline.IngestFailure{
			Device:     pipeline.AuthenticatedDeviceContext{DeviceID: event.ClientID},
			Stage:      "lifecycle_authority",
			Reason:     "unknown_or_unclaimed_device",
			ReceivedAt: event.ReceivedAt,
		})
		return pipeline.LifecycleResult{}, fmt.Errorf("%w: lifecycle client is not a claimed device", pipeline.ErrIdentityDrift)
	}

	rawEvent, err := json.Marshal(map[string]string{
		"event_type":           event.EventType,
		"principal_identifier": event.PrincipalIdentifier,
		"session_identifier":   event.SessionIdentifier,
		"version_number":       event.VersionNumber,
	})
	if err != nil {
		return pipeline.LifecycleResult{}, fmt.Errorf("marshal lifecycle event: %w", err)
	}

	tx, err := w.Pool.Begin(ctx)
	if err != nil {
		return pipeline.LifecycleResult{}, fmt.Errorf("begin lifecycle transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
INSERT INTO device_lifecycle_events (
  device_id,
  client_id,
  event_type,
  session_identifier,
  observed_at,
  raw_event,
  created_at
)
VALUES ($1, $2, $3, $4, $5, $6::jsonb, now())
`, deviceID, event.ClientID, event.EventType, event.SessionIdentifier, event.ObservedAt, string(rawEvent)); err != nil {
		return pipeline.LifecycleResult{}, fmt.Errorf("insert lifecycle event: %w", err)
	}

	connectionState := lifecycleConnectionState(event.EventType)
	switch connectionState {
	case "online":
		if _, err := tx.Exec(ctx, `
INSERT INTO device_presence (
  device_id,
  connection_state,
  last_client_id,
  last_session_identifier,
  last_connected_at,
  last_seen_at,
  updated_at
)
VALUES ($1, 'online', $2, $3, $4, $4, now())
ON CONFLICT (device_id)
DO UPDATE SET
  connection_state = CASE
    WHEN COALESCE(device_presence.last_seen_at, '-infinity'::timestamptz) <= EXCLUDED.last_seen_at THEN 'online'
    ELSE device_presence.connection_state
  END,
  last_client_id = EXCLUDED.last_client_id,
  last_session_identifier = EXCLUDED.last_session_identifier,
  last_connected_at = GREATEST(COALESCE(device_presence.last_connected_at, EXCLUDED.last_connected_at), EXCLUDED.last_connected_at),
  last_seen_at = GREATEST(COALESCE(device_presence.last_seen_at, EXCLUDED.last_seen_at), EXCLUDED.last_seen_at),
  updated_at = now()
`, deviceID, event.ClientID, event.SessionIdentifier, event.ObservedAt); err != nil {
			return pipeline.LifecycleResult{}, fmt.Errorf("upsert lifecycle connect presence: %w", err)
		}
	case "disconnected":
		if _, err := tx.Exec(ctx, `
INSERT INTO device_presence (
  device_id,
  connection_state,
  last_client_id,
  last_session_identifier,
  last_disconnected_at,
  last_seen_at,
  updated_at
)
VALUES ($1, 'disconnected', $2, $3, $4, $4, now())
ON CONFLICT (device_id)
DO UPDATE SET
  connection_state = CASE
    WHEN COALESCE(device_presence.last_seen_at, '-infinity'::timestamptz) <= EXCLUDED.last_seen_at THEN 'disconnected'
    ELSE device_presence.connection_state
  END,
  last_client_id = EXCLUDED.last_client_id,
  last_session_identifier = EXCLUDED.last_session_identifier,
  last_disconnected_at = GREATEST(COALESCE(device_presence.last_disconnected_at, EXCLUDED.last_disconnected_at), EXCLUDED.last_disconnected_at),
  last_seen_at = GREATEST(COALESCE(device_presence.last_seen_at, EXCLUDED.last_seen_at), EXCLUDED.last_seen_at),
  updated_at = now()
`, deviceID, event.ClientID, event.SessionIdentifier, event.ObservedAt); err != nil {
			return pipeline.LifecycleResult{}, fmt.Errorf("upsert lifecycle disconnect presence: %w", err)
		}
	default:
		return pipeline.LifecycleResult{}, fmt.Errorf("unsupported lifecycle event_type %q", event.EventType)
	}

	if err := tx.Commit(ctx); err != nil {
		return pipeline.LifecycleResult{}, fmt.Errorf("commit lifecycle transaction: %w", err)
	}
	return pipeline.LifecycleResult{
		Accepted:        true,
		DeviceID:        deviceID,
		EventType:       event.EventType,
		ConnectionState: connectionState,
		ObservedAt:      event.ObservedAt,
	}, nil
}

func lifecycleConnectionState(eventType string) string {
	switch eventType {
	case "connected":
		return "online"
	case "disconnected", "connect_failed":
		return "disconnected"
	default:
		return "unknown"
	}
}
