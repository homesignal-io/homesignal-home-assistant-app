package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	StatusQueued      Status = "queued"
	StatusSent        Status = "sent"
	StatusAckAccepted Status = "ack_accepted"
	StatusAckRejected Status = "ack_rejected"
	StatusAckTimedOut Status = "ack_timed_out"
	StatusRunning     Status = "running"
	StatusSucceeded   Status = "succeeded"
	StatusFailed      Status = "failed"
	StatusTimedOut    Status = "timed_out"
	StatusCanceled    Status = "canceled"

	TypeRefreshPublishPolicy CommandType = "refresh_publish_policy"
	TypeTriggerBackup        CommandType = "trigger_backup"
	TypeUploadArtifact       CommandType = "upload_artifact"
	TypeEnableDebugCapture   CommandType = "enable_debug_capture"
	TypeDisableDebugCapture  CommandType = "disable_debug_capture"
	TypeCollectDebugSnapshot CommandType = "collect_debug_snapshot"
	TypeRequestDebugBundle   CommandType = "request_debug_bundle"
)

type Status string
type CommandType string

type CommandDefinition struct {
	AckWindow    time.Duration
	ResultWindow time.Duration
	Idempotent   bool
}

type Command struct {
	CommandID         string
	AccountID         string
	SiteID            string
	DeviceID          string
	CommandType       CommandType
	Status            Status
	IdempotencyKey    string
	RequestedByUserID string
	Payload           json.RawMessage
	AckDeadlineAt     time.Time
	ResultDeadlineAt  time.Time
	QueuedAt          time.Time
	SentAt            *time.Time
	AckedAt           *time.Time
	TerminalAt        *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type ProgressEvent struct {
	CommandID  string
	Phase      string
	ReasonCode string
	Payload    json.RawMessage
	ReportedAt time.Time
	ReceivedAt time.Time
}

type Result struct {
	CommandID  string
	Status     Status
	ResultType string
	ReasonCode string
	Payload    json.RawMessage
	StartedAt  *time.Time
	FinishedAt time.Time
	ReceivedAt time.Time
}

type Repository interface {
	CreateCommand(ctx context.Context, command Command) error
	GetCommand(ctx context.Context, commandID string) (Command, error)
	SaveCommand(ctx context.Context, command Command) error
	AppendProgressEvent(ctx context.Context, event ProgressEvent) error
	RecordResult(ctx context.Context, result Result) error
}

type IDGenerator func() string
type Clock func() time.Time

type Service struct {
	Repository  Repository
	Definitions map[CommandType]CommandDefinition
	IDGenerator IDGenerator
	Clock       Clock
}

type CreateRequest struct {
	CommandID         string
	AccountID         string
	SiteID            string
	DeviceID          string
	CommandType       CommandType
	IdempotencyKey    string
	RequestedByUserID string
	Payload           json.RawMessage
}

type AckRequest struct {
	CommandID  string
	Accepted   bool
	ReasonCode string
	ReportedAt time.Time
}

type ProgressRequest struct {
	CommandID  string
	Phase      string
	ReasonCode string
	Payload    json.RawMessage
	ReportedAt time.Time
}

type ResultRequest struct {
	CommandID  string
	Status     Status
	ResultType string
	ReasonCode string
	Payload    json.RawMessage
	StartedAt  *time.Time
	FinishedAt time.Time
}

func DefaultDefinitions() map[CommandType]CommandDefinition {
	return map[CommandType]CommandDefinition{
		TypeRefreshPublishPolicy: {
			AckWindow:    15 * time.Second,
			ResultWindow: time.Minute,
			Idempotent:   true,
		},
		TypeTriggerBackup: {
			AckWindow:    15 * time.Second,
			ResultWindow: 30 * time.Minute,
			Idempotent:   false,
		},
		TypeUploadArtifact: {
			AckWindow:    15 * time.Second,
			ResultWindow: 30 * time.Minute,
			Idempotent:   false,
		},
		TypeEnableDebugCapture: {
			AckWindow:    15 * time.Second,
			ResultWindow: 5 * time.Minute,
			Idempotent:   true,
		},
		TypeDisableDebugCapture: {
			AckWindow:    15 * time.Second,
			ResultWindow: 5 * time.Minute,
			Idempotent:   true,
		},
		TypeCollectDebugSnapshot: {
			AckWindow:    15 * time.Second,
			ResultWindow: 10 * time.Minute,
			Idempotent:   true,
		},
		TypeRequestDebugBundle: {
			AckWindow:    15 * time.Second,
			ResultWindow: 30 * time.Minute,
			Idempotent:   false,
		},
	}
}

func (s Service) Create(ctx context.Context, req CreateRequest) (Command, error) {
	if s.Repository == nil {
		return Command{}, fmt.Errorf("command repository is required")
	}
	definition, err := s.definitionFor(req.CommandType)
	if err != nil {
		return Command{}, err
	}
	now := s.now()
	commandID := strings.TrimSpace(req.CommandID)
	if commandID == "" {
		if s.IDGenerator == nil {
			return Command{}, fmt.Errorf("command id is required")
		}
		commandID = strings.TrimSpace(s.IDGenerator())
		if commandID == "" {
			return Command{}, fmt.Errorf("command id is required")
		}
	}
	payload := normalizePayload(req.Payload)
	if !json.Valid(payload) {
		return Command{}, fmt.Errorf("payload must be valid JSON")
	}
	command := Command{
		CommandID:         commandID,
		AccountID:         strings.TrimSpace(req.AccountID),
		SiteID:            strings.TrimSpace(req.SiteID),
		DeviceID:          strings.TrimSpace(req.DeviceID),
		CommandType:       req.CommandType,
		Status:            StatusQueued,
		IdempotencyKey:    strings.TrimSpace(req.IdempotencyKey),
		RequestedByUserID: strings.TrimSpace(req.RequestedByUserID),
		Payload:           payload,
		AckDeadlineAt:     now.Add(definition.AckWindow),
		ResultDeadlineAt:  now.Add(definition.ResultWindow),
		QueuedAt:          now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := validateCommand(command); err != nil {
		return Command{}, err
	}
	if err := s.Repository.CreateCommand(ctx, command); err != nil {
		return Command{}, fmt.Errorf("create command: %w", err)
	}
	return command, nil
}

func (s Service) MarkSent(ctx context.Context, commandID string) (Command, error) {
	command, err := s.load(ctx, commandID)
	if err != nil {
		return Command{}, err
	}
	if command.Status != StatusQueued {
		return Command{}, fmt.Errorf("cannot mark %s command as sent", command.Status)
	}
	definition, err := s.definitionFor(command.CommandType)
	if err != nil {
		return Command{}, err
	}
	now := s.now()
	command.Status = StatusSent
	command.SentAt = &now
	command.AckDeadlineAt = now.Add(definition.AckWindow)
	command.UpdatedAt = now
	if err := s.Repository.SaveCommand(ctx, command); err != nil {
		return Command{}, fmt.Errorf("mark command sent: %w", err)
	}
	return command, nil
}

func (s Service) RecordAck(ctx context.Context, req AckRequest) (Command, error) {
	command, err := s.load(ctx, req.CommandID)
	if err != nil {
		return Command{}, err
	}
	if command.Status != StatusSent {
		return Command{}, fmt.Errorf("cannot record ack for %s command", command.Status)
	}
	now := s.now()
	reportedAt := req.ReportedAt
	if reportedAt.IsZero() {
		reportedAt = now
	}
	if now.After(command.AckDeadlineAt) {
		command.Status = StatusAckTimedOut
		command.UpdatedAt = now
		if err := s.Repository.SaveCommand(ctx, command); err != nil {
			return Command{}, fmt.Errorf("mark command ack timed out: %w", err)
		}
		return Command{}, fmt.Errorf("command ack deadline passed")
	}
	command.AckedAt = &reportedAt
	if req.Accepted {
		command.Status = StatusAckAccepted
	} else {
		command.Status = StatusAckRejected
	}
	command.UpdatedAt = now
	if err := s.Repository.SaveCommand(ctx, command); err != nil {
		return Command{}, fmt.Errorf("record command ack: %w", err)
	}
	return command, nil
}

func (s Service) ExpireMissingAck(ctx context.Context, commandID string) (Command, bool, error) {
	command, err := s.load(ctx, commandID)
	if err != nil {
		return Command{}, false, err
	}
	if command.Status != StatusSent {
		return command, false, nil
	}
	now := s.now()
	if now.Before(command.AckDeadlineAt) || now.Equal(command.AckDeadlineAt) {
		return command, false, nil
	}
	command.Status = StatusAckTimedOut
	command.UpdatedAt = now
	if err := s.Repository.SaveCommand(ctx, command); err != nil {
		return Command{}, false, fmt.Errorf("expire missing ack: %w", err)
	}
	return command, true, nil
}

func (s Service) RecordProgress(ctx context.Context, req ProgressRequest) (Command, error) {
	command, err := s.load(ctx, req.CommandID)
	if err != nil {
		return Command{}, err
	}
	switch command.Status {
	case StatusAckAccepted, StatusRunning:
	default:
		return Command{}, fmt.Errorf("cannot record progress for %s command", command.Status)
	}
	now := s.now()
	payload := normalizePayload(req.Payload)
	if !json.Valid(payload) {
		return Command{}, fmt.Errorf("progress payload must be valid JSON")
	}
	phase := strings.TrimSpace(req.Phase)
	if phase == "" {
		return Command{}, fmt.Errorf("progress phase is required")
	}
	reportedAt := req.ReportedAt
	if reportedAt.IsZero() {
		reportedAt = now
	}
	event := ProgressEvent{
		CommandID:  command.CommandID,
		Phase:      phase,
		ReasonCode: strings.TrimSpace(req.ReasonCode),
		Payload:    payload,
		ReportedAt: reportedAt,
		ReceivedAt: now,
	}
	if err := s.Repository.AppendProgressEvent(ctx, event); err != nil {
		return Command{}, fmt.Errorf("record command progress: %w", err)
	}
	command.Status = StatusRunning
	command.UpdatedAt = now
	if err := s.Repository.SaveCommand(ctx, command); err != nil {
		return Command{}, fmt.Errorf("mark command running: %w", err)
	}
	return command, nil
}

func (s Service) RecordResult(ctx context.Context, req ResultRequest) (Command, error) {
	command, err := s.load(ctx, req.CommandID)
	if err != nil {
		return Command{}, err
	}
	switch command.Status {
	case StatusAckAccepted, StatusRunning:
	default:
		return Command{}, fmt.Errorf("cannot record result for %s command", command.Status)
	}
	if !isTerminalResultStatus(req.Status) {
		return Command{}, fmt.Errorf("invalid terminal result status %q", req.Status)
	}
	resultType := strings.TrimSpace(req.ResultType)
	if resultType == "" {
		return Command{}, fmt.Errorf("result_type is required")
	}
	now := s.now()
	finishedAt := req.FinishedAt
	if finishedAt.IsZero() {
		finishedAt = now
	}
	payload := normalizePayload(req.Payload)
	if !json.Valid(payload) {
		return Command{}, fmt.Errorf("result payload must be valid JSON")
	}
	result := Result{
		CommandID:  command.CommandID,
		Status:     req.Status,
		ResultType: resultType,
		ReasonCode: strings.TrimSpace(req.ReasonCode),
		Payload:    payload,
		StartedAt:  req.StartedAt,
		FinishedAt: finishedAt,
		ReceivedAt: now,
	}
	if err := s.Repository.RecordResult(ctx, result); err != nil {
		return Command{}, fmt.Errorf("record command result: %w", err)
	}
	command.Status = req.Status
	command.TerminalAt = &now
	command.UpdatedAt = now
	if err := s.Repository.SaveCommand(ctx, command); err != nil {
		return Command{}, fmt.Errorf("mark command terminal: %w", err)
	}
	return command, nil
}

func (s Service) ExpireResult(ctx context.Context, commandID string) (Command, bool, error) {
	command, err := s.load(ctx, commandID)
	if err != nil {
		return Command{}, false, err
	}
	switch command.Status {
	case StatusAckAccepted, StatusRunning:
	default:
		return command, false, nil
	}
	now := s.now()
	if now.Before(command.ResultDeadlineAt) || now.Equal(command.ResultDeadlineAt) {
		return command, false, nil
	}
	result := Result{
		CommandID:  command.CommandID,
		Status:     StatusTimedOut,
		ResultType: string(command.CommandType),
		ReasonCode: "result_deadline_exceeded",
		Payload:    json.RawMessage(`{}`),
		FinishedAt: now,
		ReceivedAt: now,
	}
	if err := s.Repository.RecordResult(ctx, result); err != nil {
		return Command{}, false, fmt.Errorf("record command timeout result: %w", err)
	}
	command.Status = StatusTimedOut
	command.TerminalAt = &now
	command.UpdatedAt = now
	if err := s.Repository.SaveCommand(ctx, command); err != nil {
		return Command{}, false, fmt.Errorf("mark command result timed out: %w", err)
	}
	return command, true, nil
}

func (s Service) Cancel(ctx context.Context, commandID string) (Command, error) {
	command, err := s.load(ctx, commandID)
	if err != nil {
		return Command{}, err
	}
	if isTerminalCommandStatus(command.Status) {
		return Command{}, fmt.Errorf("cannot cancel terminal %s command", command.Status)
	}
	now := s.now()
	command.Status = StatusCanceled
	command.TerminalAt = &now
	command.UpdatedAt = now
	result := Result{
		CommandID:  command.CommandID,
		Status:     StatusCanceled,
		ResultType: string(command.CommandType),
		ReasonCode: "canceled",
		Payload:    json.RawMessage(`{}`),
		FinishedAt: now,
		ReceivedAt: now,
	}
	if err := s.Repository.RecordResult(ctx, result); err != nil {
		return Command{}, fmt.Errorf("record command cancellation result: %w", err)
	}
	if err := s.Repository.SaveCommand(ctx, command); err != nil {
		return Command{}, fmt.Errorf("cancel command: %w", err)
	}
	return command, nil
}

func (s Service) load(ctx context.Context, commandID string) (Command, error) {
	if s.Repository == nil {
		return Command{}, fmt.Errorf("command repository is required")
	}
	commandID = strings.TrimSpace(commandID)
	if commandID == "" {
		return Command{}, fmt.Errorf("command_id is required")
	}
	command, err := s.Repository.GetCommand(ctx, commandID)
	if err != nil {
		return Command{}, fmt.Errorf("load command: %w", err)
	}
	return command, nil
}

func (s Service) definitionFor(commandType CommandType) (CommandDefinition, error) {
	definitions := s.Definitions
	if definitions == nil {
		definitions = DefaultDefinitions()
	}
	definition, ok := definitions[commandType]
	if !ok {
		return CommandDefinition{}, fmt.Errorf("unsupported command type %q", commandType)
	}
	if definition.AckWindow <= 0 {
		return CommandDefinition{}, fmt.Errorf("ack window is required for %q", commandType)
	}
	if definition.ResultWindow <= definition.AckWindow {
		return CommandDefinition{}, fmt.Errorf("result window must exceed ack window for %q", commandType)
	}
	return definition, nil
}

func (s Service) now() time.Time {
	if s.Clock != nil {
		return s.Clock().UTC()
	}
	return time.Now().UTC()
}

func validateCommand(command Command) error {
	if strings.TrimSpace(command.CommandID) == "" {
		return fmt.Errorf("command_id is required")
	}
	if strings.TrimSpace(command.AccountID) == "" {
		return fmt.Errorf("account_id is required")
	}
	if strings.TrimSpace(command.SiteID) == "" {
		return fmt.Errorf("site_id is required")
	}
	if strings.TrimSpace(command.DeviceID) == "" {
		return fmt.Errorf("device_id is required")
	}
	if command.CommandType == "" {
		return fmt.Errorf("command_type is required")
	}
	if command.Status != StatusQueued {
		return fmt.Errorf("new command must start queued")
	}
	if command.AckDeadlineAt.IsZero() {
		return fmt.Errorf("ack_deadline_at is required")
	}
	if command.ResultDeadlineAt.IsZero() || !command.ResultDeadlineAt.After(command.AckDeadlineAt) {
		return fmt.Errorf("result_deadline_at must be after ack_deadline_at")
	}
	return nil
}

func normalizePayload(payload json.RawMessage) json.RawMessage {
	if len(payload) == 0 {
		return json.RawMessage(`{}`)
	}
	clone := make(json.RawMessage, len(payload))
	copy(clone, payload)
	return clone
}

func isTerminalResultStatus(status Status) bool {
	switch status {
	case StatusSucceeded, StatusFailed, StatusTimedOut, StatusCanceled:
		return true
	default:
		return false
	}
}

func isTerminalCommandStatus(status Status) bool {
	switch status {
	case StatusSucceeded, StatusFailed, StatusTimedOut, StatusCanceled:
		return true
	default:
		return false
	}
}
