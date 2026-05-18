package backups

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/homesignal-io/homesignal-home-assistant-app/backend/internal/domain/commands"
)

const (
	StatusRequested Status = "requested"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	StatusTimedOut  Status = "timed_out"
	StatusCanceled  Status = "canceled"
	StatusOverdue   Status = "overdue"
	StatusUnknown   Status = "unknown"
)

type Status string

type Policy struct {
	AccountID              string
	SiteID                 string
	DeviceID               string
	Status                 string
	ExpectedCadenceSeconds int
	OverdueAfterSeconds    int
	OffsiteEnabled         bool
	Metadata               json.RawMessage
}

type Run struct {
	BackupID          string
	AccountID         string
	SiteID            string
	DeviceID          string
	CommandID         string
	ArtifactUploadID  string
	Status            Status
	RequestedByUserID string
	ReasonCode        string
	Metadata          json.RawMessage
	RequestedAt       time.Time
	StartedAt         *time.Time
	FinishedAt        *time.Time
}

type DeviceStatus struct {
	AccountID          string
	SiteID             string
	DeviceID           string
	CurrentStatus      Status
	LastBackupID       string
	InProgressBackupID string
	LastSuccessAt      *time.Time
	LastFailureAt      *time.Time
	OverdueAfter       *time.Time
	ArtifactUploadID   string
	ReasonCode         string
	UpdatedAt          time.Time
}

type TriggerRequest struct {
	AccountID         string
	SiteID            string
	DeviceID          string
	RequestedByUserID string
	IdempotencyKey    string
	Metadata          json.RawMessage
}

type CommandResult struct {
	CommandID        string
	Status           commands.Status
	ReasonCode       string
	ArtifactUploadID string
	FinishedAt       time.Time
}

type Repository interface {
	CreateRun(ctx context.Context, run Run) error
	GetRunByCommandID(ctx context.Context, commandID string) (Run, error)
	SaveRun(ctx context.Context, run Run) error
	UpsertDeviceStatus(ctx context.Context, status DeviceStatus) error
}

type CommandCreator interface {
	Create(ctx context.Context, req commands.CreateRequest) (commands.Command, error)
}

type IDGenerator func() string
type Clock func() time.Time

type Service struct {
	Repository     Repository
	CommandCreator CommandCreator
	IDGenerator    IDGenerator
	Clock          Clock
}

func (s Service) TriggerBackup(ctx context.Context, req TriggerRequest) (Run, commands.Command, error) {
	if s.Repository == nil {
		return Run{}, commands.Command{}, fmt.Errorf("backup repository is required")
	}
	if s.CommandCreator == nil {
		return Run{}, commands.Command{}, fmt.Errorf("command creator is required")
	}
	now := s.now()
	backupID, err := s.newBackupID()
	if err != nil {
		return Run{}, commands.Command{}, err
	}
	metadata := normalizeJSON(req.Metadata)
	if !json.Valid(metadata) {
		return Run{}, commands.Command{}, fmt.Errorf("metadata must be valid JSON")
	}
	accountID := strings.TrimSpace(req.AccountID)
	siteID := strings.TrimSpace(req.SiteID)
	deviceID := strings.TrimSpace(req.DeviceID)
	if accountID == "" || siteID == "" || deviceID == "" {
		return Run{}, commands.Command{}, fmt.Errorf("account_id, site_id, and device_id are required")
	}
	command, err := s.CommandCreator.Create(ctx, commands.CreateRequest{
		AccountID:         accountID,
		SiteID:            siteID,
		DeviceID:          deviceID,
		CommandType:       commands.TypeTriggerBackup,
		IdempotencyKey:    strings.TrimSpace(req.IdempotencyKey),
		RequestedByUserID: strings.TrimSpace(req.RequestedByUserID),
		Payload:           json.RawMessage(`{"backup_scope":"home_assistant"}`),
	})
	if err != nil {
		return Run{}, commands.Command{}, fmt.Errorf("create backup command: %w", err)
	}
	run := Run{
		BackupID:          backupID,
		AccountID:         accountID,
		SiteID:            siteID,
		DeviceID:          deviceID,
		CommandID:         command.CommandID,
		Status:            StatusRequested,
		RequestedByUserID: strings.TrimSpace(req.RequestedByUserID),
		Metadata:          metadata,
		RequestedAt:       now,
	}
	if err := s.Repository.CreateRun(ctx, run); err != nil {
		return Run{}, commands.Command{}, fmt.Errorf("create backup run: %w", err)
	}
	if err := s.Repository.UpsertDeviceStatus(ctx, DeviceStatus{
		AccountID:          accountID,
		SiteID:             siteID,
		DeviceID:           deviceID,
		CurrentStatus:      StatusRequested,
		LastBackupID:       backupID,
		InProgressBackupID: backupID,
		UpdatedAt:          now,
	}); err != nil {
		return Run{}, commands.Command{}, fmt.Errorf("update backup status: %w", err)
	}
	return run, command, nil
}

func (s Service) RecordCommandResult(ctx context.Context, result CommandResult) (Run, DeviceStatus, error) {
	if s.Repository == nil {
		return Run{}, DeviceStatus{}, fmt.Errorf("backup repository is required")
	}
	run, err := s.Repository.GetRunByCommandID(ctx, strings.TrimSpace(result.CommandID))
	if err != nil {
		return Run{}, DeviceStatus{}, fmt.Errorf("load backup run: %w", err)
	}
	status, err := backupStatusFromCommandStatus(result.Status)
	if err != nil {
		return Run{}, DeviceStatus{}, err
	}
	now := s.now()
	finishedAt := result.FinishedAt
	if finishedAt.IsZero() {
		finishedAt = now
	}
	run.Status = status
	run.ReasonCode = strings.TrimSpace(result.ReasonCode)
	run.ArtifactUploadID = strings.TrimSpace(result.ArtifactUploadID)
	run.FinishedAt = &finishedAt
	if err := s.Repository.SaveRun(ctx, run); err != nil {
		return Run{}, DeviceStatus{}, fmt.Errorf("save backup run: %w", err)
	}
	deviceStatus := DeviceStatus{
		AccountID:          run.AccountID,
		SiteID:             run.SiteID,
		DeviceID:           run.DeviceID,
		CurrentStatus:      status,
		LastBackupID:       run.BackupID,
		ArtifactUploadID:   run.ArtifactUploadID,
		ReasonCode:         run.ReasonCode,
		InProgressBackupID: "",
		UpdatedAt:          now,
	}
	switch status {
	case StatusSucceeded:
		deviceStatus.LastSuccessAt = &finishedAt
	case StatusFailed, StatusTimedOut, StatusCanceled:
		deviceStatus.LastFailureAt = &finishedAt
	}
	if err := s.Repository.UpsertDeviceStatus(ctx, deviceStatus); err != nil {
		return Run{}, DeviceStatus{}, fmt.Errorf("update backup status: %w", err)
	}
	return run, deviceStatus, nil
}

func IsOverdue(status DeviceStatus, now time.Time) bool {
	if status.OverdueAfter == nil {
		return false
	}
	if status.CurrentStatus == StatusRunning || status.CurrentStatus == StatusRequested {
		return false
	}
	return now.After(*status.OverdueAfter)
}

func backupStatusFromCommandStatus(status commands.Status) (Status, error) {
	switch status {
	case commands.StatusSucceeded:
		return StatusSucceeded, nil
	case commands.StatusFailed:
		return StatusFailed, nil
	case commands.StatusTimedOut:
		return StatusTimedOut, nil
	case commands.StatusCanceled:
		return StatusCanceled, nil
	default:
		return "", fmt.Errorf("unsupported backup command result status %q", status)
	}
}

func (s Service) now() time.Time {
	if s.Clock != nil {
		return s.Clock().UTC()
	}
	return time.Now().UTC()
}

func (s Service) newBackupID() (string, error) {
	if s.IDGenerator == nil {
		return "", fmt.Errorf("backup id generator is required")
	}
	backupID := strings.TrimSpace(s.IDGenerator())
	if backupID == "" {
		return "", fmt.Errorf("backup id is required")
	}
	return backupID, nil
}

func normalizeJSON(value json.RawMessage) json.RawMessage {
	if len(value) == 0 {
		return json.RawMessage(`{}`)
	}
	clone := make(json.RawMessage, len(value))
	copy(clone, value)
	return clone
}
