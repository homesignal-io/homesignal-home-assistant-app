package backups

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/homesignal-io/homesignal-home-assistant-app/backend/internal/domain/commands"
)

func TestServiceTriggersBackupCommandAndRun(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	repo := newFakeRepository()
	commandCreator := &fakeCommandCreator{}
	service := newTestService(repo, commandCreator, now)

	run, command, err := service.TriggerBackup(context.Background(), TriggerRequest{
		AccountID:         "acct_123",
		SiteID:            "site_123",
		DeviceID:          "dev_123",
		RequestedByUserID: "usr_123",
		IdempotencyKey:    "idem_123",
	})
	if err != nil {
		t.Fatalf("trigger backup: %v", err)
	}
	if command.CommandType != commands.TypeTriggerBackup {
		t.Fatalf("expected trigger_backup command, got %s", command.CommandType)
	}
	if string(commandCreator.requests[0].Payload) != `{"backup_scope":"home_assistant"}` {
		t.Fatalf("unexpected command payload %s", commandCreator.requests[0].Payload)
	}
	if run.Status != StatusRequested || run.CommandID != command.CommandID {
		t.Fatalf("unexpected backup run %#v", run)
	}
	status := repo.statusByDevice["dev_123"]
	if status.CurrentStatus != StatusRequested || status.InProgressBackupID != "bak_123" {
		t.Fatalf("unexpected device backup status %#v", status)
	}
}

func TestServiceRecordsSucceededCommandResult(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	repo := newFakeRepository()
	service := newTestService(repo, &fakeCommandCreator{}, now)
	run := Run{
		BackupID:  "bak_123",
		AccountID: "acct_123",
		SiteID:    "site_123",
		DeviceID:  "dev_123",
		CommandID: "cmd_123",
		Status:    StatusRequested,
	}
	repo.runsByID[run.BackupID] = run
	repo.commandToRun[run.CommandID] = run.BackupID

	finishedAt := now.Add(3 * time.Minute)
	updatedRun, status, err := service.RecordCommandResult(context.Background(), CommandResult{
		CommandID:        "cmd_123",
		Status:           commands.StatusSucceeded,
		ArtifactUploadID: "art_123",
		FinishedAt:       finishedAt,
	})
	if err != nil {
		t.Fatalf("record command result: %v", err)
	}
	if updatedRun.Status != StatusSucceeded || updatedRun.ArtifactUploadID != "art_123" {
		t.Fatalf("unexpected run update %#v", updatedRun)
	}
	if status.CurrentStatus != StatusSucceeded || status.LastSuccessAt == nil || !status.LastSuccessAt.Equal(finishedAt) {
		t.Fatalf("unexpected status %#v", status)
	}
	if status.InProgressBackupID != "" {
		t.Fatalf("expected in-progress backup to clear, got %q", status.InProgressBackupID)
	}
}

func TestServiceRecordsFailedCommandResult(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	repo := newFakeRepository()
	service := newTestService(repo, &fakeCommandCreator{}, now)
	run := Run{
		BackupID:  "bak_123",
		AccountID: "acct_123",
		SiteID:    "site_123",
		DeviceID:  "dev_123",
		CommandID: "cmd_123",
		Status:    StatusRunning,
	}
	repo.runsByID[run.BackupID] = run
	repo.commandToRun[run.CommandID] = run.BackupID

	_, status, err := service.RecordCommandResult(context.Background(), CommandResult{
		CommandID:  "cmd_123",
		Status:     commands.StatusFailed,
		ReasonCode: "ha_backup_failed",
	})
	if err != nil {
		t.Fatalf("record command result: %v", err)
	}
	if status.CurrentStatus != StatusFailed || status.LastFailureAt == nil {
		t.Fatalf("unexpected failure status %#v", status)
	}
	if status.ReasonCode != "ha_backup_failed" {
		t.Fatalf("expected reason code to persist, got %q", status.ReasonCode)
	}
}

func TestServiceRejectsNonTerminalCommandResult(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	repo := newFakeRepository()
	service := newTestService(repo, &fakeCommandCreator{}, now)
	run := Run{
		BackupID:  "bak_123",
		AccountID: "acct_123",
		SiteID:    "site_123",
		DeviceID:  "dev_123",
		CommandID: "cmd_123",
		Status:    StatusRequested,
	}
	repo.runsByID[run.BackupID] = run
	repo.commandToRun[run.CommandID] = run.BackupID

	_, _, err := service.RecordCommandResult(context.Background(), CommandResult{
		CommandID: "cmd_123",
		Status:    commands.StatusAckAccepted,
	})
	if err == nil {
		t.Fatalf("expected non-terminal command status to fail")
	}
}

func TestIsOverdueIgnoresInProgressBackups(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	overdueAt := now.Add(-time.Minute)
	if !IsOverdue(DeviceStatus{CurrentStatus: StatusSucceeded, OverdueAfter: &overdueAt}, now) {
		t.Fatalf("expected stale successful status to be overdue")
	}
	if IsOverdue(DeviceStatus{CurrentStatus: StatusRunning, OverdueAfter: &overdueAt}, now) {
		t.Fatalf("running backups should not be marked overdue")
	}
}

func newTestService(repo *fakeRepository, commandCreator *fakeCommandCreator, now time.Time) Service {
	return Service{
		Repository:     repo,
		CommandCreator: commandCreator,
		IDGenerator: func() string {
			return "bak_123"
		},
		Clock: func() time.Time {
			return now
		},
	}
}

type fakeRepository struct {
	runsByID       map[string]Run
	commandToRun   map[string]string
	statusByDevice map[string]DeviceStatus
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{
		runsByID:       map[string]Run{},
		commandToRun:   map[string]string{},
		statusByDevice: map[string]DeviceStatus{},
	}
}

func (r *fakeRepository) CreateRun(_ context.Context, run Run) error {
	r.runsByID[run.BackupID] = run
	r.commandToRun[run.CommandID] = run.BackupID
	return nil
}

func (r *fakeRepository) GetRunByCommandID(_ context.Context, commandID string) (Run, error) {
	backupID, ok := r.commandToRun[commandID]
	if !ok {
		return Run{}, fmt.Errorf("backup run not found")
	}
	return r.runsByID[backupID], nil
}

func (r *fakeRepository) SaveRun(_ context.Context, run Run) error {
	r.runsByID[run.BackupID] = run
	r.commandToRun[run.CommandID] = run.BackupID
	return nil
}

func (r *fakeRepository) UpsertDeviceStatus(_ context.Context, status DeviceStatus) error {
	r.statusByDevice[status.DeviceID] = status
	return nil
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
	}, nil
}
