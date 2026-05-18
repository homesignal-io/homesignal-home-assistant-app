package readmodels

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/homesignal-io/homesignal-home-assistant-app/backend/internal/platform/authn"
)

type Store struct {
	DB  *sql.DB
	Now func() time.Time
}

func (s Store) Dashboard(ctx context.Context, subject authn.Subject) (any, error) {
	now := s.now()
	devices, err := s.deviceFacts(ctx, subject)
	if err != nil {
		return DashboardResponse{}, err
	}
	activity, err := s.Activity(ctx, subject)
	if err != nil {
		return DashboardResponse{}, err
	}
	return buildDashboard(now, devices, activity.(ActivityResponse).Activity), nil
}

func (s Store) Devices(ctx context.Context, subject authn.Subject) (any, error) {
	devices, err := s.deviceFacts(ctx, subject)
	if err != nil {
		return DevicesResponse{}, err
	}
	return DevicesResponse{
		SchemaVersion: 1,
		Devices:       buildDeviceRows(s.now(), devices),
	}, nil
}

func (s Store) Activity(ctx context.Context, subject authn.Subject) (any, error) {
	activity, err := s.activityRows(ctx, subject)
	if err != nil {
		return ActivityResponse{}, err
	}
	return ActivityResponse{
		SchemaVersion: 1,
		Activity:      activity,
	}, nil
}

func (s Store) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s Store) validate() error {
	if s.DB == nil {
		return fmt.Errorf("read model database is required")
	}
	return nil
}

func (s Store) deviceFacts(ctx context.Context, subject authn.Subject) ([]deviceFactRow, error) {
	if err := s.validate(); err != nil {
		return nil, err
	}
	rows, err := s.DB.QueryContext(ctx, deviceFactsSQL, string(subject.ID))
	if err != nil {
		return nil, fmt.Errorf("query device read models: %w", err)
	}
	defer rows.Close()

	facts := []deviceFactRow{}
	for rows.Next() {
		var row sqlDeviceFactRow
		if err := rows.Scan(
			&row.AccountID,
			&row.SiteID,
			&row.SiteName,
			&row.SiteCategory,
			&row.CustomerDisplayName,
			&row.CompactLocation,
			&row.DeviceID,
			&row.ThingName,
			&row.Presence,
			&row.LastSeenAt,
			&row.HAInstalledVersion,
			&row.HALatestVersion,
			&row.HALatestVersionSource,
			&row.SupervisorInstalledVersion,
			&row.SupervisorLatestVersion,
			&row.HAAppInstalledVersion,
			&row.HAAppDesiredVersion,
			&row.HAAppReleaseTrack,
			&row.HAAppUpdateStatus,
			&row.HAAppUpdateReferenceTime,
			&row.BackupStatus,
			&row.BackupLastSuccessAt,
			&row.BackupLastFailureAt,
			&row.StorageStatus,
			&row.StorageDetail,
			&row.HAUpdateAdvisoryStatus,
			&row.HAUpdateLatestVersion,
			&row.EmailAlertRecipients,
		); err != nil {
			return nil, fmt.Errorf("scan device read model: %w", err)
		}
		facts = append(facts, row.toFact())
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate device read models: %w", err)
	}
	return facts, nil
}

func (s Store) activityRows(ctx context.Context, subject authn.Subject) ([]ActivityRow, error) {
	if err := s.validate(); err != nil {
		return nil, err
	}
	rows, err := s.DB.QueryContext(ctx, activitySQL, string(subject.ID))
	if err != nil {
		return nil, fmt.Errorf("query activity read model: %w", err)
	}
	defer rows.Close()

	activity := []ActivityRow{}
	for rows.Next() {
		var row sqlActivityRow
		if err := rows.Scan(
			&row.ActivityID,
			&row.OccurredAt,
			&row.Category,
			&row.Action,
			&row.SubjectType,
			&row.SubjectID,
			&row.SubjectLabel,
			&row.Detail,
			&row.Severity,
			&row.ActorLabel,
		); err != nil {
			return nil, fmt.Errorf("scan activity row: %w", err)
		}
		activity = append(activity, row.toActivity())
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate activity read model: %w", err)
	}
	return activity, nil
}

type sqlDeviceFactRow struct {
	AccountID                  string
	SiteID                     string
	SiteName                   string
	SiteCategory               sql.NullString
	CustomerDisplayName        sql.NullString
	CompactLocation            sql.NullString
	DeviceID                   string
	ThingName                  sql.NullString
	Presence                   sql.NullString
	LastSeenAt                 sql.NullTime
	HAInstalledVersion         sql.NullString
	HALatestVersion            sql.NullString
	HALatestVersionSource      sql.NullString
	SupervisorInstalledVersion sql.NullString
	SupervisorLatestVersion    sql.NullString
	HAAppInstalledVersion      sql.NullString
	HAAppDesiredVersion        sql.NullString
	HAAppReleaseTrack          sql.NullString
	HAAppUpdateStatus          sql.NullString
	HAAppUpdateReferenceTime   sql.NullTime
	BackupStatus               sql.NullString
	BackupLastSuccessAt        sql.NullTime
	BackupLastFailureAt        sql.NullTime
	StorageStatus              sql.NullString
	StorageDetail              sql.NullString
	HAUpdateAdvisoryStatus     sql.NullString
	HAUpdateLatestVersion      sql.NullString
	EmailAlertRecipients       int
}

func (r sqlDeviceFactRow) toFact() deviceFactRow {
	return deviceFactRow{
		AccountID:                  r.AccountID,
		SiteID:                     r.SiteID,
		SiteName:                   firstNonEmpty(r.SiteName, "Unnamed site"),
		SiteCategory:               nullableString(r.SiteCategory),
		CustomerDisplayName:        stringValue(r.CustomerDisplayName),
		CompactLocation:            stringValue(r.CompactLocation),
		DeviceID:                   r.DeviceID,
		ThingName:                  stringValue(r.ThingName),
		Presence:                   stringValue(r.Presence),
		LastSeenAt:                 nullableTime(r.LastSeenAt),
		HAInstalledVersion:         stringValue(r.HAInstalledVersion),
		HALatestVersion:            nullableString(r.HALatestVersion),
		HALatestVersionSource:      stringValue(r.HALatestVersionSource),
		SupervisorInstalledVersion: stringValue(r.SupervisorInstalledVersion),
		SupervisorLatestVersion:    nullableString(r.SupervisorLatestVersion),
		HAAppInstalledVersion:      stringValue(r.HAAppInstalledVersion),
		HAAppDesiredVersion:        nullableString(r.HAAppDesiredVersion),
		HAAppReleaseTrack:          nullableString(r.HAAppReleaseTrack),
		HAAppUpdateStatus:          stringValue(r.HAAppUpdateStatus),
		HAAppUpdateReferenceTime:   nullableTime(r.HAAppUpdateReferenceTime),
		BackupStatus:               stringValue(r.BackupStatus),
		BackupLastSuccessAt:        nullableTime(r.BackupLastSuccessAt),
		BackupLastFailureAt:        nullableTime(r.BackupLastFailureAt),
		StorageStatus:              stringValue(r.StorageStatus),
		StorageDetail:              stringValue(r.StorageDetail),
		HAUpdateAdvisoryStatus:     stringValue(r.HAUpdateAdvisoryStatus),
		HAUpdateLatestVersion:      nullableString(r.HAUpdateLatestVersion),
		EmailAlertRecipients:       r.EmailAlertRecipients,
	}
}

type sqlActivityRow struct {
	ActivityID   string
	OccurredAt   time.Time
	Category     string
	Action       string
	SubjectType  string
	SubjectID    string
	SubjectLabel string
	Detail       string
	Severity     string
	ActorLabel   string
}

func (r sqlActivityRow) toActivity() ActivityRow {
	return ActivityRow{
		ActivityID:   r.ActivityID,
		OccurredAt:   r.OccurredAt.UTC(),
		Category:     r.Category,
		Action:       r.Action,
		SubjectType:  r.SubjectType,
		SubjectID:    r.SubjectID,
		SubjectLabel: r.SubjectLabel,
		Detail:       r.Detail,
		Severity:     r.Severity,
		ActorLabel:   r.ActorLabel,
	}
}

func nullableString(value sql.NullString) *string {
	if !value.Valid || value.String == "" {
		return nil
	}
	return &value.String
}

func stringValue(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func nullableTime(value sql.NullTime) *time.Time {
	if !value.Valid || value.Time.IsZero() {
		return nil
	}
	t := value.Time.UTC()
	return &t
}

const deviceFactsSQL = `
WITH authorized_accounts AS (
  SELECT account_id
  FROM account_memberships
  WHERE user_id = $1
    AND status = 'active'
)
SELECT
  d.account_id,
  s.site_id,
  COALESCE(s.display_name, s.site_id) AS site_name,
  s.site_category,
  cr.display_name AS customer_display_name,
  NULLIF(CONCAT_WS(', ', s.service_address->>'city', s.service_address->>'state'), '') AS compact_location,
  d.device_id,
  d.iot_thing_name,
  COALESCE(dp.connection_state, 'unknown') AS presence,
  COALESCE(dp.last_seen_at, d.last_seen_at) AS last_seen_at,
  latest.payload #>> '{home_assistant,core,version}' AS ha_installed_version,
  ha_advisory.latest_version AS ha_latest_version,
  CASE
    WHEN ha_advisory.advisory_status IN ('current', 'update_available') THEN 'catalog'
    ELSE 'unavailable'
  END AS ha_latest_version_source,
  latest.payload #>> '{home_assistant,supervisor,version}' AS supervisor_installed_version,
  latest.payload #>> '{home_assistant,supervisor,version}' AS supervisor_latest_version,
  COALESCE(update_status.installed_version, latest.payload #>> '{agent,update,current_version}', latest.payload #>> '{agent,version}') AS ha_app_installed_version,
  COALESCE(update_status.desired_version, latest.payload #>> '{agent,update,desired_version}') AS ha_app_desired_version,
  COALESCE(update_status.release_track, latest.payload #>> '{agent,update,channel}') AS ha_app_release_track,
  COALESCE(update_status.status, latest.payload #>> '{agent,update,status}', 'unknown') AS ha_app_update_status,
  COALESCE(update_status.reported_at, update_status.received_at, update_status.updated_at) AS ha_app_update_reference_time,
  backup.current_status AS backup_status,
  backup.last_success_at AS backup_last_success_at,
  backup.last_failure_at AS backup_last_failure_at,
  latest.payload #>> '{home_assistant,storage,status}' AS storage_status,
  CASE
    WHEN latest.payload #>> '{home_assistant,storage,status}' IN ('warning', 'critical') THEN 'Storage pressure reported by the app'
    ELSE NULL
  END AS storage_detail,
  COALESCE(ha_advisory.advisory_status, 'hidden') AS ha_update_advisory_status,
  ha_advisory.latest_version AS ha_update_latest_version,
  COALESCE(alert_recipient_counts.verified_count, 0) AS email_alert_recipients
FROM devices d
JOIN authorized_accounts aa ON aa.account_id = d.account_id
JOIN sites s ON s.site_id = d.site_id
LEFT JOIN customer_records cr ON cr.customer_record_id = s.customer_record_id
LEFT JOIN device_presence dp ON dp.device_id = d.device_id
LEFT JOIN device_latest_state latest
  ON latest.device_id = d.device_id
  AND latest.message_type = 'telemetry'
  AND latest.schema_type = 'device.health_snapshot'
  AND latest.schema_version = 1
LEFT JOIN device_backup_status backup ON backup.device_id = d.device_id
LEFT JOIN device_update_status update_status ON update_status.device_id = d.device_id
LEFT JOIN device_ha_version_advisories ha_advisory ON ha_advisory.device_id = d.device_id
LEFT JOIN (
  SELECT account_id, COUNT(*)::int AS verified_count
  FROM alert_recipients
  WHERE status = 'verified'
    AND deleted_at IS NULL
  GROUP BY account_id
) alert_recipient_counts ON alert_recipient_counts.account_id = d.account_id
WHERE d.claim_state = 'CLAIMED'
  AND d.revoked_at IS NULL
  AND s.deleted_at IS NULL
ORDER BY COALESCE(dp.last_seen_at, d.last_seen_at, d.updated_at) DESC NULLS LAST, s.display_name ASC
LIMIT 250
`

const activitySQL = `
WITH authorized_accounts AS (
  SELECT account_id
  FROM account_memberships
  WHERE user_id = $1
    AND status = 'active'
),
activity AS (
  SELECT
    'alert:' || a.alert_id AS activity_id,
    a.updated_at AS occurred_at,
    'alert' AS category,
    a.family AS action,
    'site' AS subject_type,
    a.site_id AS subject_id,
    COALESCE(s.display_name, s.site_id) AS subject_label,
    a.detail AS detail,
    a.severity AS severity,
    'HomeSignal' AS actor_label
  FROM alerts a
  JOIN authorized_accounts aa ON aa.account_id = a.account_id
  JOIN sites s ON s.site_id = a.site_id
  WHERE a.status = 'active'

  UNION ALL

  SELECT
    'backup:' || b.backup_id AS activity_id,
    COALESCE(b.finished_at, b.started_at, b.requested_at) AS occurred_at,
    'backup' AS category,
    b.status AS action,
    'site' AS subject_type,
    b.site_id AS subject_id,
    COALESCE(s.display_name, s.site_id) AS subject_label,
    CASE
      WHEN b.status = 'succeeded' THEN 'Offsite backup completed'
      WHEN b.status = 'failed' THEN 'Scheduled backup failed'
      ELSE 'Backup status changed'
    END AS detail,
    CASE
      WHEN b.status = 'failed' THEN 'critical'
      ELSE 'info'
    END AS severity,
    'HomeSignal' AS actor_label
  FROM backup_runs b
  JOIN authorized_accounts aa ON aa.account_id = b.account_id
  JOIN sites s ON s.site_id = b.site_id
  WHERE b.status IN ('succeeded', 'failed')

  UNION ALL

  SELECT
    'device_lifecycle:' || dle.event_id::text AS activity_id,
    dle.observed_at AS occurred_at,
    'device' AS category,
    CASE
      WHEN dle.event_type = 'connected' THEN 'reported_connected'
      ELSE 'reported_disconnected'
    END AS action,
    'device' AS subject_type,
    dle.device_id AS subject_id,
    COALESCE(s.display_name, d.device_id) AS subject_label,
    CASE
      WHEN dle.event_type = 'connected' THEN 'HomeSignal heard from this Home Assistant instance'
      ELSE 'HomeSignal has not heard from this Home Assistant instance recently'
    END AS detail,
    CASE
      WHEN dle.event_type = 'connected' THEN 'info'
      ELSE 'critical'
    END AS severity,
    'HomeSignal' AS actor_label
  FROM device_lifecycle_events dle
  JOIN devices d ON d.device_id = dle.device_id
  JOIN authorized_accounts aa ON aa.account_id = d.account_id
  LEFT JOIN sites s ON s.site_id = d.site_id
  WHERE dle.event_type IN ('connected', 'disconnected', 'connect_failed')
)
SELECT activity_id, occurred_at, category, action, subject_type, subject_id, subject_label, detail, severity, actor_label
FROM activity
ORDER BY occurred_at DESC
LIMIT 50
`
