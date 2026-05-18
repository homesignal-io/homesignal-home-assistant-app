package controlplane

import (
	"encoding/json"
	"os"
	"testing"
)

type dashboardFixture struct {
	SchemaVersion int `json:"schema_version"`
	Summary       struct {
		OpenIssueCount     int `json:"open_issue_count"`
		SitesNeedingReview int `json:"sites_needing_review"`
	} `json:"summary"`
	ManagedHomeAssistants []deviceReadModel `json:"managed_home_assistants"`
}

type devicesFixture struct {
	SchemaVersion int               `json:"schema_version"`
	Devices       []deviceReadModel `json:"devices"`
}

type activityFixture struct {
	SchemaVersion int           `json:"schema_version"`
	Activity      []activityRow `json:"activity"`
}

type deviceReadModel struct {
	SiteID        string `json:"site_id"`
	SiteName      string `json:"site_name"`
	PrimaryAction string `json:"primary_action"`
	Device        struct {
		DeviceID string `json:"device_id"`
		Presence string `json:"presence"`
	} `json:"device"`
	HomeAssistant struct {
		LatestVersionSource string `json:"latest_version_source"`
	} `json:"home_assistant"`
	Issues []issueProjection `json:"issues"`
}

type issueProjection struct {
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

type activityRow struct {
	ActivityID   string `json:"activity_id"`
	OccurredAt   string `json:"occurred_at"`
	Category     string `json:"category"`
	Action       string `json:"action"`
	SubjectType  string `json:"subject_type"`
	SubjectID    string `json:"subject_id"`
	SubjectLabel string `json:"subject_label"`
	Detail       string `json:"detail"`
	Severity     string `json:"severity"`
	ActorLabel   string `json:"actor_label"`
}

func TestPortalReadModelFixturesAgreeOnIssueProjection(t *testing.T) {
	var dashboard dashboardFixture
	readJSONFixture(t, "../../../testdata/contracts/api/public-v1/dashboard.json", &dashboard)
	var devices devicesFixture
	readJSONFixture(t, "../../../testdata/contracts/api/public-v1/devices.json", &devices)

	dashboardIssues := countIssues(dashboard.ManagedHomeAssistants)
	deviceIssues := countIssues(devices.Devices)
	if dashboardIssues != deviceIssues {
		t.Fatalf("dashboard issues = %d, devices issues = %d", dashboardIssues, deviceIssues)
	}
	if dashboard.Summary.OpenIssueCount != deviceIssues {
		t.Fatalf("dashboard summary open issues = %d, want %d", dashboard.Summary.OpenIssueCount, deviceIssues)
	}

	sitesWithIssues := map[string]bool{}
	for _, row := range devices.Devices {
		if len(row.Issues) > 0 {
			sitesWithIssues[row.SiteID] = true
		}
		for _, issue := range row.Issues {
			assertIssueProjectionComplete(t, row, issue)
		}
	}
	if dashboard.Summary.SitesNeedingReview != len(sitesWithIssues) {
		t.Fatalf("dashboard sites needing review = %d, want %d", dashboard.Summary.SitesNeedingReview, len(sitesWithIssues))
	}
}

func TestPortalReadModelFixturesHideHAUpdateWhenCatalogUnavailable(t *testing.T) {
	var devices devicesFixture
	readJSONFixture(t, "../../../testdata/contracts/api/public-v1/devices.json", &devices)

	for _, row := range devices.Devices {
		if row.HomeAssistant.LatestVersionSource != "unavailable" {
			continue
		}
		for _, issue := range row.Issues {
			if issue.IssueCode == "ha_update_advisory" {
				t.Fatalf("device %s exposes ha_update_advisory while latest version source is unavailable", row.Device.DeviceID)
			}
		}
	}
}

func TestPublicActivityFixtureExcludesInternalOnlyEvents(t *testing.T) {
	var fixture activityFixture
	readJSONFixture(t, "../../../testdata/contracts/api/public-v1/activity.json", &fixture)

	allowedCategories := map[string]bool{
		"alert":      true,
		"backup":     true,
		"device":     true,
		"update":     true,
		"enrollment": true,
		"account":    true,
	}
	for _, row := range fixture.Activity {
		if !allowedCategories[row.Category] {
			t.Fatalf("activity %s has non-public category %q", row.ActivityID, row.Category)
		}
		if row.Detail == "" || row.SubjectLabel == "" || row.OccurredAt == "" {
			t.Fatalf("activity row is not product-shaped: %#v", row)
		}
	}
}

func readJSONFixture(t *testing.T, path string, target any) {
	t.Helper()
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(payload, target); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}

func countIssues(rows []deviceReadModel) int {
	total := 0
	for _, row := range rows {
		total += len(row.Issues)
	}
	return total
}

func assertIssueProjectionComplete(t *testing.T, row deviceReadModel, issue issueProjection) {
	t.Helper()
	if issue.IssueCode == "" || issue.Severity == "" || issue.Label == "" || issue.Detail == "" || issue.SourceArea == "" || issue.PrimaryAction == "" {
		t.Fatalf("issue projection is incomplete: %#v", issue)
	}
	if issue.SiteID != row.SiteID {
		t.Fatalf("issue site_id = %q, want %q", issue.SiteID, row.SiteID)
	}
	if issue.DeviceID != row.Device.DeviceID {
		t.Fatalf("issue device_id = %q, want %q", issue.DeviceID, row.Device.DeviceID)
	}
	if issue.SortPriority <= 0 {
		t.Fatalf("issue sort_priority = %d, want positive", issue.SortPriority)
	}
	switch issue.Severity {
	case "critical", "warning", "info":
	default:
		t.Fatalf("issue severity = %q", issue.Severity)
	}
}
