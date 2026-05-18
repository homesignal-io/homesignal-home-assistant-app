package readmodels

import (
	"testing"
	"time"
)

func TestBuildDeviceRowProjectsIssuesAndPrimaryAction(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	lastSeen := now.Add(-72 * time.Hour)
	backupFailure := now.Add(-70 * time.Hour)
	updateReported := now.Add(-73 * time.Hour)
	desired := "0.1.4"
	latestHA := "2026.5.2"
	category := "residential"

	row := buildDeviceRow(now, deviceFactRow{
		SiteID:                   "site_124",
		SiteName:                 "Lee Residence",
		SiteCategory:             &category,
		CustomerDisplayName:      "Jordan Lee",
		CompactLocation:          "Columbus, OH",
		DeviceID:                 "dev_124",
		ThingName:                "dev_124",
		Presence:                 "disconnected",
		LastSeenAt:               &lastSeen,
		HAInstalledVersion:       "2026.5.1",
		HAUpdateAdvisoryStatus:   "update_available",
		HAUpdateLatestVersion:    &latestHA,
		HAAppInstalledVersion:    "0.1.3",
		HAAppDesiredVersion:      &desired,
		HAAppUpdateStatus:        "pending",
		HAAppUpdateReferenceTime: &updateReported,
		BackupStatus:             "failed",
		BackupLastFailureAt:      &backupFailure,
		StorageStatus:            "warning",
	})

	if row.PrimaryAction != "view_device" {
		t.Fatalf("primary action = %q, want first issue action view_device", row.PrimaryAction)
	}
	wantCodes := []string{
		"device_disconnected",
		"backup_failed",
		"app_update_attention",
		"ha_update_advisory",
		"storage_warning",
	}
	if len(row.Issues) != len(wantCodes) {
		t.Fatalf("issues = %d, want %d: %#v", len(row.Issues), len(wantCodes), row.Issues)
	}
	for i, want := range wantCodes {
		if row.Issues[i].IssueCode != want {
			t.Fatalf("issue %d = %q, want %q", i, row.Issues[i].IssueCode, want)
		}
	}
	if row.HomeAssistant.LatestVersion == nil || *row.HomeAssistant.LatestVersion != latestHA {
		t.Fatalf("HA latest = %#v, want %q", row.HomeAssistant.LatestVersion, latestHA)
	}
	if row.Storage.Status != "warning" {
		t.Fatalf("storage status = %q, want warning", row.Storage.Status)
	}
}

func TestBuildSummaryUsesProjectedIssues(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	lastSeen := now.Add(-2 * time.Hour)
	desired := "0.1.4"
	oldUpdate := now.Add(-72 * time.Hour)
	facts := []deviceFactRow{
		{
			SiteID:                   "site_1",
			SiteName:                 "One",
			DeviceID:                 "dev_1",
			Presence:                 "online",
			LastSeenAt:               &lastSeen,
			BackupStatus:             "succeeded",
			HAAppInstalledVersion:    "0.1.3",
			HAAppDesiredVersion:      &desired,
			HAAppUpdateStatus:        "pending",
			HAAppUpdateReferenceTime: &oldUpdate,
			EmailAlertRecipients:     1,
		},
		{
			SiteID:       "site_2",
			SiteName:     "Two",
			DeviceID:     "dev_2",
			Presence:     "online",
			LastSeenAt:   &lastSeen,
			BackupStatus: "succeeded",
		},
	}

	devices := buildDeviceRows(now, facts)
	summary := buildSummary(devices, facts)

	if summary.ManagedSites != 2 || summary.ManagedDevices != 2 || summary.OnlineDevices != 2 {
		t.Fatalf("unexpected fleet summary: %#v", summary)
	}
	if summary.OpenIssueCount != 1 || summary.AppUpdateAttentionCount != 1 || summary.SitesNeedingReview != 1 {
		t.Fatalf("unexpected issue summary: %#v", summary)
	}
	if summary.EmailAlertsStatus != "configured" {
		t.Fatalf("email status = %q, want configured", summary.EmailAlertsStatus)
	}
}

func TestAppUpdateAttentionHonorsGracePeriod(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	desired := "0.1.4"
	recent := now.Add(-2 * time.Hour)
	old := now.Add(-72 * time.Hour)

	base := deviceFactRow{
		HAAppInstalledVersion: "0.1.3",
		HAAppDesiredVersion:   &desired,
		HAAppUpdateStatus:     "pending",
	}

	base.HAAppUpdateReferenceTime = &recent
	if appUpdateNeedsAttention(now, base) {
		t.Fatal("recent update should still be inside grace period")
	}

	base.HAAppUpdateReferenceTime = &old
	if !appUpdateNeedsAttention(now, base) {
		t.Fatal("old update should need attention")
	}
}
