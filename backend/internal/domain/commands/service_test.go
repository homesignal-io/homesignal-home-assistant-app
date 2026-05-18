package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestServiceCreatesQueuedCommandWithDefaultWindows(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	repo := newFakeRepository()
	service := newTestService(repo, &now)

	command, err := service.Create(context.Background(), CreateRequest{
		AccountID:         "acct_123",
		SiteID:            "site_123",
		DeviceID:          "dev_123",
		CommandType:       TypeRefreshPublishPolicy,
		RequestedByUserID: "usr_123",
		Payload:           json.RawMessage(`{"reason":"manual_refresh"}`),
	})
	if err != nil {
		t.Fatalf("create command: %v", err)
	}
	if command.CommandID != "cmd_generated" {
		t.Fatalf("unexpected generated command id %q", command.CommandID)
	}
	if command.Status != StatusQueued {
		t.Fatalf("expected queued, got %s", command.Status)
	}
	if got := command.AckDeadlineAt.Sub(now); got != 15*time.Second {
		t.Fatalf("expected 15s ack window, got %s", got)
	}
	if got := command.ResultDeadlineAt.Sub(now); got != time.Minute {
		t.Fatalf("expected 1m result window, got %s", got)
	}
	if string(repo.commands[command.CommandID].Payload) != `{"reason":"manual_refresh"}` {
		t.Fatalf("payload was not stored")
	}
}

func TestServiceRecordsAcceptedLifecycleThroughTerminalResult(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	repo := newFakeRepository()
	service := newTestService(repo, &now)
	ctx := context.Background()

	command, err := service.Create(ctx, CreateRequest{
		CommandID:   "cmd_accepted",
		AccountID:   "acct_123",
		SiteID:      "site_123",
		DeviceID:    "dev_123",
		CommandType: TypeTriggerBackup,
	})
	if err != nil {
		t.Fatalf("create command: %v", err)
	}

	now = now.Add(time.Second)
	command, err = service.MarkSent(ctx, command.CommandID)
	if err != nil {
		t.Fatalf("mark sent: %v", err)
	}
	if command.Status != StatusSent || command.SentAt == nil {
		t.Fatalf("expected sent with sent_at, got %#v", command)
	}

	now = now.Add(2 * time.Second)
	command, err = service.RecordAck(ctx, AckRequest{CommandID: command.CommandID, Accepted: true})
	if err != nil {
		t.Fatalf("record ack: %v", err)
	}
	if command.Status != StatusAckAccepted || command.AckedAt == nil {
		t.Fatalf("expected ack accepted, got %#v", command)
	}

	now = now.Add(time.Second)
	command, err = service.RecordProgress(ctx, ProgressRequest{
		CommandID: command.CommandID,
		Phase:     "uploading",
		Payload:   json.RawMessage(`{"artifact":"backup"}`),
	})
	if err != nil {
		t.Fatalf("record progress: %v", err)
	}
	if command.Status != StatusRunning {
		t.Fatalf("expected running after progress, got %s", command.Status)
	}
	if len(repo.progress) != 1 || repo.progress[0].Phase != "uploading" {
		t.Fatalf("expected one uploading progress event, got %#v", repo.progress)
	}

	now = now.Add(time.Second)
	command, err = service.RecordResult(ctx, ResultRequest{
		CommandID:  command.CommandID,
		Status:     StatusSucceeded,
		ResultType: "backup",
		Payload:    json.RawMessage(`{"backup_id":"bak_123"}`),
	})
	if err != nil {
		t.Fatalf("record result: %v", err)
	}
	if command.Status != StatusSucceeded || command.TerminalAt == nil {
		t.Fatalf("expected succeeded terminal command, got %#v", command)
	}
	if len(repo.results) != 1 || repo.results[0].Status != StatusSucceeded {
		t.Fatalf("expected one succeeded result, got %#v", repo.results)
	}
}

func TestServiceRejectsUnsupportedCommandTypes(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	service := newTestService(newFakeRepository(), &now)

	_, err := service.Create(context.Background(), CreateRequest{
		CommandID:   "cmd_bad",
		AccountID:   "acct_123",
		SiteID:      "site_123",
		DeviceID:    "dev_123",
		CommandType: "apply_update",
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported command type") {
		t.Fatalf("expected unsupported command type error, got %v", err)
	}
}

func TestServiceRejectsInvalidTransitions(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	repo := newFakeRepository()
	service := newTestService(repo, &now)
	ctx := context.Background()

	command, err := service.Create(ctx, CreateRequest{
		CommandID:   "cmd_invalid",
		AccountID:   "acct_123",
		SiteID:      "site_123",
		DeviceID:    "dev_123",
		CommandType: TypeRefreshPublishPolicy,
	})
	if err != nil {
		t.Fatalf("create command: %v", err)
	}
	if _, err := service.RecordAck(ctx, AckRequest{CommandID: command.CommandID, Accepted: true}); err == nil {
		t.Fatalf("expected ack before sent to fail")
	}
	if _, err := service.RecordResult(ctx, ResultRequest{CommandID: command.CommandID, Status: StatusSucceeded, ResultType: "publish_policy"}); err == nil {
		t.Fatalf("expected result before accepted ack to fail")
	}
}

func TestServiceExpiresMissingAckWithoutRetryingNonIdempotentCommand(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	repo := newFakeRepository()
	service := newTestService(repo, &now)
	ctx := context.Background()

	command, err := service.Create(ctx, CreateRequest{
		CommandID:   "cmd_backup",
		AccountID:   "acct_123",
		SiteID:      "site_123",
		DeviceID:    "dev_123",
		CommandType: TypeTriggerBackup,
	})
	if err != nil {
		t.Fatalf("create command: %v", err)
	}
	command, err = service.MarkSent(ctx, command.CommandID)
	if err != nil {
		t.Fatalf("mark sent: %v", err)
	}

	now = command.AckDeadlineAt.Add(time.Nanosecond)
	command, expired, err := service.ExpireMissingAck(ctx, command.CommandID)
	if err != nil {
		t.Fatalf("expire missing ack: %v", err)
	}
	if !expired {
		t.Fatalf("expected command to expire")
	}
	if command.Status != StatusAckTimedOut {
		t.Fatalf("expected ack_timed_out, got %s", command.Status)
	}
	if repo.createCalls != 1 {
		t.Fatalf("non-idempotent timeout should not create retry commands; create calls=%d", repo.createCalls)
	}
	if len(repo.results) != 0 {
		t.Fatalf("ack timeout should not create a terminal result, got %#v", repo.results)
	}
}

func TestServiceExpiresMissingTerminalResult(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	repo := newFakeRepository()
	service := newTestService(repo, &now)
	ctx := context.Background()

	command, err := service.Create(ctx, CreateRequest{
		CommandID:   "cmd_policy",
		AccountID:   "acct_123",
		SiteID:      "site_123",
		DeviceID:    "dev_123",
		CommandType: TypeRefreshPublishPolicy,
	})
	if err != nil {
		t.Fatalf("create command: %v", err)
	}
	if _, err := service.MarkSent(ctx, command.CommandID); err != nil {
		t.Fatalf("mark sent: %v", err)
	}
	if _, err := service.RecordAck(ctx, AckRequest{CommandID: command.CommandID, Accepted: true}); err != nil {
		t.Fatalf("record ack: %v", err)
	}

	now = command.ResultDeadlineAt.Add(time.Nanosecond)
	command, expired, err := service.ExpireResult(ctx, command.CommandID)
	if err != nil {
		t.Fatalf("expire result: %v", err)
	}
	if !expired {
		t.Fatalf("expected result to expire")
	}
	if command.Status != StatusTimedOut || command.TerminalAt == nil {
		t.Fatalf("expected timed_out terminal command, got %#v", command)
	}
	if len(repo.results) != 1 || repo.results[0].ReasonCode != "result_deadline_exceeded" {
		t.Fatalf("expected timeout result, got %#v", repo.results)
	}
}

func newTestService(repo *fakeRepository, now *time.Time) Service {
	return Service{
		Repository: repo,
		IDGenerator: func() string {
			return "cmd_generated"
		},
		Clock: func() time.Time {
			return *now
		},
	}
}

type fakeRepository struct {
	commands    map[string]Command
	progress    []ProgressEvent
	results     []Result
	createCalls int
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{commands: map[string]Command{}}
}

func (r *fakeRepository) CreateCommand(_ context.Context, command Command) error {
	if _, ok := r.commands[command.CommandID]; ok {
		return fmt.Errorf("command already exists")
	}
	r.createCalls++
	r.commands[command.CommandID] = command
	return nil
}

func (r *fakeRepository) GetCommand(_ context.Context, commandID string) (Command, error) {
	command, ok := r.commands[commandID]
	if !ok {
		return Command{}, fmt.Errorf("command not found")
	}
	return command, nil
}

func (r *fakeRepository) SaveCommand(_ context.Context, command Command) error {
	if _, ok := r.commands[command.CommandID]; !ok {
		return fmt.Errorf("command not found")
	}
	r.commands[command.CommandID] = command
	return nil
}

func (r *fakeRepository) AppendProgressEvent(_ context.Context, event ProgressEvent) error {
	r.progress = append(r.progress, event)
	return nil
}

func (r *fakeRepository) RecordResult(_ context.Context, result Result) error {
	for _, existing := range r.results {
		if existing.CommandID == result.CommandID {
			return fmt.Errorf("terminal result already exists")
		}
	}
	r.results = append(r.results, result)
	return nil
}
