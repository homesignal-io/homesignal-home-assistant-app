package readmodels

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const appUpdateGracePeriod = 48 * time.Hour

type deviceFactRow struct {
	AccountID                  string
	SiteID                     string
	SiteName                   string
	SiteCategory               *string
	CustomerDisplayName        string
	CompactLocation            string
	DeviceID                   string
	ThingName                  string
	Presence                   string
	LastSeenAt                 *time.Time
	HAInstalledVersion         string
	HALatestVersion            *string
	HALatestVersionSource      string
	SupervisorInstalledVersion string
	SupervisorLatestVersion    *string
	HAAppInstalledVersion      string
	HAAppDesiredVersion        *string
	HAAppReleaseTrack          *string
	HAAppUpdateStatus          string
	HAAppUpdateReferenceTime   *time.Time
	BackupStatus               string
	BackupLastSuccessAt        *time.Time
	BackupLastFailureAt        *time.Time
	StorageStatus              string
	StorageDetail              string
	HAUpdateAdvisoryStatus     string
	HAUpdateLatestVersion      *string
	EmailAlertRecipients       int
}

func buildDashboard(now time.Time, rows []deviceFactRow, activity []ActivityRow) DashboardResponse {
	devices := buildDeviceRows(now, rows)
	summary := buildSummary(devices, rows)
	state := "ok"
	if summary.OpenIssueCount > 0 {
		state = "action_required"
	}
	return DashboardResponse{
		SchemaVersion:         1,
		DashboardState:        state,
		Summary:               summary,
		ManagedHomeAssistants: devices,
		Activity:              activity,
	}
}

func buildDeviceRows(now time.Time, rows []deviceFactRow) []DeviceReadModel {
	devices := make([]DeviceReadModel, 0, len(rows))
	for _, row := range rows {
		devices = append(devices, buildDeviceRow(now, row))
	}
	sort.SliceStable(devices, func(i, j int) bool {
		if len(devices[i].Issues) != len(devices[j].Issues) {
			return len(devices[i].Issues) > len(devices[j].Issues)
		}
		return devices[i].SiteName < devices[j].SiteName
	})
	return devices
}

func buildDeviceRow(now time.Time, row deviceFactRow) DeviceReadModel {
	lastSeen := valueTime(row.LastSeenAt, now)
	presence := publicPresence(row.Presence)
	haLatest := row.HALatestVersion
	haLatestSource := row.HALatestVersionSource
	if row.HAUpdateAdvisoryStatus == "update_available" && row.HAUpdateLatestVersion != nil {
		haLatest = row.HAUpdateLatestVersion
		haLatestSource = "catalog"
	}
	if haLatestSource == "" {
		haLatestSource = "unavailable"
	}

	device := DeviceReadModel{
		SiteID:              row.SiteID,
		SiteName:            row.SiteName,
		SiteCategory:        row.SiteCategory,
		CustomerDisplayName: firstNonEmpty(row.CustomerDisplayName, "Unknown customer"),
		CompactLocation:     firstNonEmpty(row.CompactLocation, "Unknown location"),
		Device: DeviceSummary{
			DeviceID:   row.DeviceID,
			ThingName:  firstNonEmpty(row.ThingName, row.DeviceID),
			Presence:   presence,
			LastSeenAt: lastSeen,
		},
		HomeAssistant: VersionSummary{
			InstalledVersion:    firstNonEmpty(row.HAInstalledVersion, "unknown"),
			LatestVersion:       haLatest,
			LatestVersionSource: haLatestSource,
		},
		Supervisor: BasicVersionSummary{
			InstalledVersion: firstNonEmpty(row.SupervisorInstalledVersion, "unknown"),
			LatestVersion:    row.SupervisorLatestVersion,
		},
		HAApp: HAAppSummary{
			InstalledVersion: firstNonEmpty(row.HAAppInstalledVersion, "unknown"),
			DesiredVersion:   row.HAAppDesiredVersion,
			ReleaseTrack:     row.HAAppReleaseTrack,
			UpdateStatus:     firstNonEmpty(row.HAAppUpdateStatus, "unknown"),
		},
		Backup: BackupSummary{
			Status:        publicBackupStatus(row.BackupStatus),
			LastSuccessAt: row.BackupLastSuccessAt,
			LastFailureAt: row.BackupLastFailureAt,
		},
		Storage: StorageSummary{
			Status: publicStorageStatus(row.StorageStatus),
			Detail: storageDetail(row.StorageStatus, row.StorageDetail),
		},
		PrimaryAction: "view_device",
	}
	device.Issues = buildIssues(now, row, device)
	if len(device.Issues) > 0 {
		device.PrimaryAction = device.Issues[0].PrimaryAction
	}
	return device
}

func buildIssues(now time.Time, row deviceFactRow, device DeviceReadModel) []IssueProjection {
	issues := []IssueProjection{}
	add := func(issue IssueProjection) {
		issue.SiteID = row.SiteID
		issue.DeviceID = row.DeviceID
		issues = append(issues, issue)
	}

	if device.Device.Presence != "online" {
		add(IssueProjection{
			IssueCode:     "device_disconnected",
			Severity:      "critical",
			Label:         "Disconnected",
			Detail:        fmt.Sprintf("Last seen %s", formatIssueTime(device.Device.LastSeenAt)),
			SourceArea:    "presence",
			SortPriority:  10,
			PrimaryAction: "view_device",
		})
	}

	switch device.Backup.Status {
	case "failed":
		add(IssueProjection{
			IssueCode:     "backup_failed",
			Severity:      "critical",
			Label:         "Backup failed",
			Detail:        fmt.Sprintf("Last backup attempt failed %s", formatIssueTime(valueTime(row.BackupLastFailureAt, now))),
			SourceArea:    "backup",
			SortPriority:  20,
			PrimaryAction: "view_backups",
		})
	case "overdue":
		add(IssueProjection{
			IssueCode:     "backup_overdue",
			Severity:      "warning",
			Label:         "Backup overdue",
			Detail:        fmt.Sprintf("Last successful backup %s", formatIssueTime(valueTime(row.BackupLastSuccessAt, time.Time{}))),
			SourceArea:    "backup",
			SortPriority:  30,
			PrimaryAction: "view_backups",
		})
	}

	if appUpdateNeedsAttention(now, row) {
		desired := "the desired version"
		if row.HAAppDesiredVersion != nil && *row.HAAppDesiredVersion != "" {
			desired = *row.HAAppDesiredVersion
		}
		add(IssueProjection{
			IssueCode:     "app_update_attention",
			Severity:      "warning",
			Label:         "App update attention",
			Detail:        fmt.Sprintf("HomeSignal app %s is desired; device reports %s", desired, device.HAApp.InstalledVersion),
			SourceArea:    "updates",
			SortPriority:  40,
			PrimaryAction: "view_updates",
		})
	}

	if row.HAUpdateAdvisoryStatus == "update_available" && device.HomeAssistant.LatestVersion != nil {
		add(IssueProjection{
			IssueCode:     "ha_update_advisory",
			Severity:      "info",
			Label:         "Home Assistant update available",
			Detail:        fmt.Sprintf("Home Assistant %s is available; device reports %s", *device.HomeAssistant.LatestVersion, device.HomeAssistant.InstalledVersion),
			SourceArea:    "updates",
			SortPriority:  50,
			PrimaryAction: "view_device",
		})
	}

	if device.Storage.Status == "warning" {
		add(IssueProjection{
			IssueCode:     "storage_warning",
			Severity:      "warning",
			Label:         "Storage warning",
			Detail:        device.Storage.Detail,
			SourceArea:    "storage",
			SortPriority:  60,
			PrimaryAction: "view_device",
		})
	}

	sort.SliceStable(issues, func(i, j int) bool {
		return issues[i].SortPriority < issues[j].SortPriority
	})
	return issues
}

func buildSummary(devices []DeviceReadModel, facts []deviceFactRow) DashboardSummary {
	siteIDs := map[string]struct{}{}
	sitesWithIssues := map[string]struct{}{}
	backupIssues := 0
	appUpdateIssues := 0
	onlineDevices := 0
	openIssues := 0
	emailRecipients := 0

	for index, device := range devices {
		siteIDs[device.SiteID] = struct{}{}
		if device.Device.Presence == "online" {
			onlineDevices++
		}
		if len(device.Issues) > 0 {
			sitesWithIssues[device.SiteID] = struct{}{}
			openIssues += len(device.Issues)
		}
		for _, issue := range device.Issues {
			switch issue.IssueCode {
			case "backup_failed", "backup_overdue":
				backupIssues++
			case "app_update_attention":
				appUpdateIssues++
			}
		}
		if index < len(facts) {
			emailRecipients += facts[index].EmailAlertRecipients
		}
	}

	emailStatus := "not_configured"
	if emailRecipients > 0 {
		emailStatus = "configured"
	}

	return DashboardSummary{
		ManagedSites:            len(siteIDs),
		ManagedDevices:          len(devices),
		OnlineDevices:           onlineDevices,
		SitesNeedingReview:      len(sitesWithIssues),
		OpenIssueCount:          openIssues,
		BackupIssueCount:        backupIssues,
		AppUpdateAttentionCount: appUpdateIssues,
		EmailAlertsStatus:       emailStatus,
	}
}

func publicPresence(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "online":
		return "online"
	case "disconnected":
		return "disconnected"
	default:
		return "degraded"
	}
}

func publicBackupStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "succeeded":
		return "succeeded"
	case "failed", "timed_out", "canceled":
		return "failed"
	case "overdue":
		return "overdue"
	case "requested", "running":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "unknown"
	}
}

func publicStorageStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "warning", "critical":
		return "warning"
	case "healthy", "ok", "ready":
		return "healthy"
	default:
		return "healthy"
	}
}

func storageDetail(status string, detail string) string {
	if strings.TrimSpace(detail) != "" {
		return detail
	}
	if publicStorageStatus(status) == "warning" {
		return "Storage pressure reported by the app"
	}
	return "Storage is within local policy"
}

func appUpdateNeedsAttention(now time.Time, row deviceFactRow) bool {
	if row.HAAppDesiredVersion == nil || *row.HAAppDesiredVersion == "" {
		return false
	}
	if row.HAAppInstalledVersion == "" || row.HAAppInstalledVersion == *row.HAAppDesiredVersion {
		return false
	}
	if strings.EqualFold(row.HAAppUpdateStatus, "current") {
		return false
	}
	reference := valueTime(row.HAAppUpdateReferenceTime, now)
	return !reference.After(now.Add(-appUpdateGracePeriod))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func valueTime(value *time.Time, fallback time.Time) time.Time {
	if value == nil || value.IsZero() {
		return fallback
	}
	return value.UTC()
}

func formatIssueTime(value time.Time) string {
	if value.IsZero() {
		return "unknown"
	}
	return value.UTC().Format("Jan 2, 2006, 3:04 PM")
}
