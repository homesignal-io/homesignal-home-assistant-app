package releases

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/homesignal-io/homesignal-home-assistant-app/backend/internal/domain/commands"
	"github.com/homesignal-io/homesignal-home-assistant-app/backend/internal/domain/edgestate"
)

func TestUpdateServiceRecordsReportedStatusIntoReadModelAndProjection(t *testing.T) {
	repo := newFakeUpdateRepository()
	service := UpdateService{Repository: repo, Clock: fixedClock}
	autoUpdate := true
	updateAvailable := false
	reportedAt := time.Date(2026, 5, 18, 12, 30, 0, 0, time.UTC)

	status, err := service.RecordReportedUpdateStatus(context.Background(), UpdateStatusReport{
		AccountID:              "acct_123",
		SiteID:                 "site_123",
		DeviceID:               "dev_123",
		ChannelKey:             ChannelStable,
		HAAppSlug:              "homesignal_manager",
		HAAppRepositorySource:  "https://github.com/homesignal-io/homesignal-home-assistant-app",
		CloudEnvironment:       EnvironmentProduction,
		ReleaseTrack:           "stable",
		InstalledVersion:       "0.1.4",
		DesiredVersion:         "0.1.4",
		LatestAvailableVersion: "0.1.4",
		AutoUpdateEnabled:      &autoUpdate,
		UpdateAvailable:        &updateAvailable,
		Status:                 UpdateStatusCurrent,
		Metadata:               json.RawMessage(`{"source":"shadow_reported"}`),
		ReportedAt:             reportedAt,
	})
	if err != nil {
		t.Fatalf("record status: %v", err)
	}
	if status.Status != UpdateStatusCurrent {
		t.Fatalf("unexpected status %#v", status)
	}
	if repo.updateStatuses["dev_123"].InstalledVersion != "0.1.4" {
		t.Fatalf("expected update read model to be stored")
	}
	if len(repo.projections) != 1 {
		t.Fatalf("expected one update projection, got %d", len(repo.projections))
	}
	projection := repo.projections[0]
	if projection.ProjectionKey != edgestate.StateKeyUpdate || projection.ConvergenceStatus != "converged" {
		t.Fatalf("unexpected update projection %#v", projection)
	}
	if !strings.Contains(string(projection.Projection), `"current_version":"0.1.4"`) {
		t.Fatalf("projection payload missing current version: %s", projection.Projection)
	}
}

func TestUpdateServiceRequestsBoundedStatusCheckCommand(t *testing.T) {
	creator := &fakeCommandCreator{}
	service := UpdateService{CommandCreator: creator}

	command, err := service.RequestUpdateStatusCheck(context.Background(), UpdateCommandRequest{
		AccountID:         "acct_123",
		SiteID:            "site_123",
		DeviceID:          "dev_123",
		RequestedByUserID: "user_123",
		IdempotencyKey:    "check_update_status:dev_123",
		ReasonCode:        "stale_projection",
	})
	if err != nil {
		t.Fatalf("request update status check: %v", err)
	}
	if command.CommandType != commands.TypeCheckUpdateStatus {
		t.Fatalf("expected check_update_status command, got %q", command.CommandType)
	}
	if !strings.Contains(string(creator.requests[0].Payload), `"operation":"check_update_status"`) {
		t.Fatalf("unexpected command payload: %s", creator.requests[0].Payload)
	}
}

func TestUpdateServiceRequestsBoundedRepairCommand(t *testing.T) {
	creator := &fakeCommandCreator{}
	service := UpdateService{CommandCreator: creator}

	command, err := service.RequestUpdateRepair(context.Background(), UpdateCommandRequest{
		AccountID:         "acct_123",
		SiteID:            "site_123",
		DeviceID:          "dev_123",
		RequestedByUserID: "user_123",
		DesiredVersion:    "0.1.4",
		ChannelKey:        ChannelStable,
		RolloutID:         "rollout_123",
	})
	if err != nil {
		t.Fatalf("request update repair: %v", err)
	}
	if command.CommandType != commands.TypeRepairUpdateState {
		t.Fatalf("expected repair_update_state command, got %q", command.CommandType)
	}
	if strings.Contains(string(creator.requests[0].Payload), "url") || strings.Contains(string(creator.requests[0].Payload), "binary") {
		t.Fatalf("repair payload must not include binary delivery details: %s", creator.requests[0].Payload)
	}
}

func TestUpdateServiceRejectsInstallLikeRepairPayload(t *testing.T) {
	service := UpdateService{CommandCreator: &fakeCommandCreator{}}

	_, err := service.RequestUpdateRepair(context.Background(), UpdateCommandRequest{
		AccountID:      "acct_123",
		SiteID:         "site_123",
		DeviceID:       "dev_123",
		DesiredVersion: "latest",
		ChannelKey:     ChannelStable,
	})
	if err == nil || !strings.Contains(err.Error(), "immutable") {
		t.Fatalf("expected immutable repair target error, got %v", err)
	}
}

type fakeCommandCreator struct {
	requests []commands.CreateRequest
}

func (c *fakeCommandCreator) Create(_ context.Context, req commands.CreateRequest) (commands.Command, error) {
	c.requests = append(c.requests, req)
	return commands.Command{
		CommandID:   "cmd_123",
		AccountID:   req.AccountID,
		SiteID:      req.SiteID,
		DeviceID:    req.DeviceID,
		CommandType: req.CommandType,
		Status:      commands.StatusQueued,
		Payload:     req.Payload,
	}, nil
}
