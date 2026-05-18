package readmodels

import "time"

type DashboardResponse struct {
	SchemaVersion         int               `json:"schema_version"`
	DashboardState        string            `json:"dashboard_state"`
	Summary               DashboardSummary  `json:"summary"`
	ManagedHomeAssistants []DeviceReadModel `json:"managed_home_assistants"`
	Activity              []ActivityRow     `json:"activity"`
}

type DevicesResponse struct {
	SchemaVersion int               `json:"schema_version"`
	Devices       []DeviceReadModel `json:"devices"`
}

type ActivityResponse struct {
	SchemaVersion int           `json:"schema_version"`
	Activity      []ActivityRow `json:"activity"`
}

type DashboardSummary struct {
	ManagedSites            int    `json:"managed_sites"`
	ManagedDevices          int    `json:"managed_devices"`
	OnlineDevices           int    `json:"online_devices"`
	SitesNeedingReview      int    `json:"sites_needing_review"`
	OpenIssueCount          int    `json:"open_issue_count"`
	BackupIssueCount        int    `json:"backup_issue_count"`
	AppUpdateAttentionCount int    `json:"app_update_attention_count"`
	EmailAlertsStatus       string `json:"email_alerts_status"`
}

type DeviceReadModel struct {
	SiteID              string              `json:"site_id"`
	SiteName            string              `json:"site_name"`
	SiteCategory        *string             `json:"site_category"`
	CustomerDisplayName string              `json:"customer_display_name"`
	CompactLocation     string              `json:"compact_location"`
	Device              DeviceSummary       `json:"device"`
	HomeAssistant       VersionSummary      `json:"home_assistant"`
	Supervisor          BasicVersionSummary `json:"supervisor"`
	HAApp               HAAppSummary        `json:"ha_app"`
	Backup              BackupSummary       `json:"backup"`
	Storage             StorageSummary      `json:"storage"`
	PrimaryAction       string              `json:"primary_action"`
	Issues              []IssueProjection   `json:"issues"`
}

type DeviceSummary struct {
	DeviceID   string    `json:"device_id"`
	ThingName  string    `json:"thing_name"`
	Presence   string    `json:"presence"`
	LastSeenAt time.Time `json:"last_seen_at"`
}

type VersionSummary struct {
	InstalledVersion    string  `json:"installed_version"`
	LatestVersion       *string `json:"latest_version"`
	LatestVersionSource string  `json:"latest_version_source"`
}

type BasicVersionSummary struct {
	InstalledVersion string  `json:"installed_version"`
	LatestVersion    *string `json:"latest_version"`
}

type HAAppSummary struct {
	InstalledVersion string  `json:"installed_version"`
	DesiredVersion   *string `json:"desired_version"`
	ReleaseTrack     *string `json:"release_track"`
	UpdateStatus     string  `json:"update_status"`
}

type BackupSummary struct {
	Status        string     `json:"status"`
	LastSuccessAt *time.Time `json:"last_success_at"`
	LastFailureAt *time.Time `json:"last_failure_at"`
}

type StorageSummary struct {
	Status string `json:"status"`
	Detail string `json:"detail"`
}

type IssueProjection struct {
	IssueCode     string `json:"issue_code"`
	Severity      string `json:"severity"`
	Label         string `json:"label"`
	Detail        string `json:"detail"`
	SourceArea    string `json:"source_area"`
	SortPriority  int    `json:"sort_priority"`
	PrimaryAction string `json:"primary_action"`
	SiteID        string `json:"site_id"`
	DeviceID      string `json:"device_id"`
}

type ActivityRow struct {
	ActivityID   string    `json:"activity_id"`
	OccurredAt   time.Time `json:"occurred_at"`
	Category     string    `json:"category"`
	Action       string    `json:"action"`
	SubjectType  string    `json:"subject_type"`
	SubjectID    string    `json:"subject_id"`
	SubjectLabel string    `json:"subject_label"`
	Detail       string    `json:"detail"`
	Severity     string    `json:"severity"`
	ActorLabel   string    `json:"actor_label"`
}
