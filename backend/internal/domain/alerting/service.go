package alerting

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrNotFound = errors.New("alert record not found")

const (
	FamilyDeviceDisconnected Family = "device_disconnected"
	FamilyBackupFailed       Family = "backup_failed"
	FamilyBackupOverdue      Family = "backup_overdue"
	FamilyAppUpdateAttention Family = "app_update_attention"

	SeverityCritical Severity = "critical"
	SeverityWarning  Severity = "warning"
	SeverityInfo     Severity = "info"

	StatusActive   Status = "active"
	StatusResolved Status = "resolved"

	EventCreated      EventType = "created"
	EventUpdated      EventType = "updated"
	EventResolved     EventType = "resolved"
	EventAcknowledged EventType = "acknowledged"
	EventSnoozed      EventType = "snoozed"
)

type Family string
type Severity string
type Status string
type EventType string

type Candidate struct {
	CandidateID   string
	EventType     string
	AccountID     string
	SiteID        string
	DeviceID      string
	Source        string
	PreviousState json.RawMessage
	NewState      json.RawMessage
	Metadata      json.RawMessage
	OccurredAt    time.Time
	ReceivedAt    time.Time
}

type Alert struct {
	AlertID              string
	AlertKey             string
	AccountID            string
	SiteID               string
	DeviceID             string
	Family               Family
	Severity             Severity
	Status               Status
	Title                string
	Detail               string
	ReasonCode           string
	FirstCandidateID     string
	LastCandidateID      string
	OccurrenceCount      int
	AcknowledgedByUserID string
	AcknowledgedAt       *time.Time
	SnoozedUntil         *time.Time
	ResolvedAt           *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type AlertEvent struct {
	AlertID          string
	CandidateID      string
	EventType        EventType
	ActorSubjectType string
	ActorSubjectID   string
	Metadata         json.RawMessage
	OccurredAt       time.Time
}

type CandidateVerification struct {
	Healthy    bool
	AlertKey   string
	Family     Family
	Severity   Severity
	Title      string
	Detail     string
	ReasonCode string
	Metadata   json.RawMessage
}

type CandidateResult struct {
	Duplicate bool
	Ignored   bool
	Resolved  bool
	Alert     Alert
}

type AlertActionRequest struct {
	AlertID        string
	ActorSubjectID string
	ActorType      string
	ReasonCode     string
	SnoozedUntil   time.Time
}

type Repository interface {
	TryRecordCandidate(ctx context.Context, candidate Candidate) (bool, error)
	GetActiveAlertByKey(ctx context.Context, alertKey string) (Alert, error)
	GetAlert(ctx context.Context, alertID string) (Alert, error)
	CreateAlert(ctx context.Context, alert Alert) error
	SaveAlert(ctx context.Context, alert Alert) error
	RecordAlertEvent(ctx context.Context, event AlertEvent) error
}

type StateVerifier interface {
	VerifyCandidate(ctx context.Context, candidate Candidate) (CandidateVerification, error)
}

type IDGenerator func() string
type Clock func() time.Time

type Service struct {
	Repository  Repository
	Verifier    StateVerifier
	IDGenerator IDGenerator
	Clock       Clock
}

func (s Service) IntakeCandidate(ctx context.Context, candidate Candidate) (CandidateResult, error) {
	if s.Repository == nil {
		return CandidateResult{}, fmt.Errorf("alert repository is required")
	}
	if s.Verifier == nil {
		return CandidateResult{}, fmt.Errorf("alert candidate verifier is required")
	}
	candidate = normalizeCandidate(candidate, s.now())
	if err := validateCandidate(candidate); err != nil {
		return CandidateResult{}, err
	}
	recorded, err := s.Repository.TryRecordCandidate(ctx, candidate)
	if err != nil {
		return CandidateResult{}, fmt.Errorf("record alert candidate: %w", err)
	}
	if !recorded {
		return CandidateResult{Duplicate: true, Ignored: true}, nil
	}
	verification, err := s.Verifier.VerifyCandidate(ctx, candidate)
	if err != nil {
		return CandidateResult{}, fmt.Errorf("verify alert candidate: %w", err)
	}
	if err := validateVerification(verification); err != nil {
		return CandidateResult{}, err
	}
	active, err := s.Repository.GetActiveAlertByKey(ctx, verification.AlertKey)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return CandidateResult{}, fmt.Errorf("load active alert: %w", err)
	}
	if verification.Healthy {
		if errors.Is(err, ErrNotFound) {
			return CandidateResult{Ignored: true}, nil
		}
		resolved, err := s.resolveAlert(ctx, active, candidate.CandidateID, "candidate_recovered", "system", "system")
		if err != nil {
			return CandidateResult{}, err
		}
		return CandidateResult{Resolved: true, Alert: resolved}, nil
	}
	if errors.Is(err, ErrNotFound) {
		alert, err := s.createAlert(ctx, candidate, verification)
		if err != nil {
			return CandidateResult{}, err
		}
		return CandidateResult{Alert: alert}, nil
	}
	alert := active
	alert.Severity = verification.Severity
	alert.Title = verification.Title
	alert.Detail = verification.Detail
	alert.ReasonCode = verification.ReasonCode
	alert.LastCandidateID = candidate.CandidateID
	alert.OccurrenceCount++
	alert.UpdatedAt = s.now()
	if err := s.Repository.SaveAlert(ctx, alert); err != nil {
		return CandidateResult{}, fmt.Errorf("update alert: %w", err)
	}
	if err := s.recordEvent(ctx, alert, candidate.CandidateID, EventUpdated, "system", "system", verification.Metadata); err != nil {
		return CandidateResult{}, err
	}
	return CandidateResult{Alert: alert}, nil
}

func (s Service) AcknowledgeAlert(ctx context.Context, req AlertActionRequest) (Alert, error) {
	alert, err := s.loadAlert(ctx, req.AlertID)
	if err != nil {
		return Alert{}, err
	}
	now := s.now()
	alert.AcknowledgedAt = &now
	alert.AcknowledgedByUserID = strings.TrimSpace(req.ActorSubjectID)
	alert.UpdatedAt = now
	if err := s.Repository.SaveAlert(ctx, alert); err != nil {
		return Alert{}, fmt.Errorf("acknowledge alert: %w", err)
	}
	if err := s.recordEvent(ctx, alert, "", EventAcknowledged, actorType(req.ActorType), actorID(req.ActorSubjectID), reasonMetadata(req.ReasonCode)); err != nil {
		return Alert{}, err
	}
	return alert, nil
}

func (s Service) SnoozeAlert(ctx context.Context, req AlertActionRequest) (Alert, error) {
	alert, err := s.loadAlert(ctx, req.AlertID)
	if err != nil {
		return Alert{}, err
	}
	if !req.SnoozedUntil.After(s.now()) {
		return Alert{}, fmt.Errorf("snoozed_until must be in the future")
	}
	snoozedUntil := req.SnoozedUntil.UTC()
	alert.SnoozedUntil = &snoozedUntil
	alert.UpdatedAt = s.now()
	if err := s.Repository.SaveAlert(ctx, alert); err != nil {
		return Alert{}, fmt.Errorf("snooze alert: %w", err)
	}
	if err := s.recordEvent(ctx, alert, "", EventSnoozed, actorType(req.ActorType), actorID(req.ActorSubjectID), mustJSON(map[string]string{
		"reason_code":   strings.TrimSpace(req.ReasonCode),
		"snoozed_until": snoozedUntil.Format(time.RFC3339),
	})); err != nil {
		return Alert{}, err
	}
	return alert, nil
}

func (s Service) ResolveAlert(ctx context.Context, req AlertActionRequest) (Alert, error) {
	alert, err := s.loadAlert(ctx, req.AlertID)
	if err != nil {
		return Alert{}, err
	}
	return s.resolveAlert(ctx, alert, "", strings.TrimSpace(req.ReasonCode), actorType(req.ActorType), actorID(req.ActorSubjectID))
}

func (s Service) createAlert(ctx context.Context, candidate Candidate, verification CandidateVerification) (Alert, error) {
	alertID := ""
	if s.IDGenerator != nil {
		alertID = strings.TrimSpace(s.IDGenerator())
	}
	if alertID == "" {
		return Alert{}, fmt.Errorf("alert id is required")
	}
	now := s.now()
	alert := Alert{
		AlertID:          alertID,
		AlertKey:         verification.AlertKey,
		AccountID:        candidate.AccountID,
		SiteID:           candidate.SiteID,
		DeviceID:         candidate.DeviceID,
		Family:           verification.Family,
		Severity:         verification.Severity,
		Status:           StatusActive,
		Title:            verification.Title,
		Detail:           verification.Detail,
		ReasonCode:       verification.ReasonCode,
		FirstCandidateID: candidate.CandidateID,
		LastCandidateID:  candidate.CandidateID,
		OccurrenceCount:  1,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := validateAlert(alert); err != nil {
		return Alert{}, err
	}
	if err := s.Repository.CreateAlert(ctx, alert); err != nil {
		return Alert{}, fmt.Errorf("create alert: %w", err)
	}
	if err := s.recordEvent(ctx, alert, candidate.CandidateID, EventCreated, "system", "system", verification.Metadata); err != nil {
		return Alert{}, err
	}
	return alert, nil
}

func (s Service) resolveAlert(ctx context.Context, alert Alert, candidateID string, reasonCode string, actorSubjectType string, actorSubjectID string) (Alert, error) {
	if alert.Status == StatusResolved {
		return alert, nil
	}
	now := s.now()
	alert.Status = StatusResolved
	alert.ResolvedAt = &now
	alert.UpdatedAt = now
	candidateID = strings.TrimSpace(candidateID)
	if candidateID != "" {
		alert.LastCandidateID = candidateID
	}
	if strings.TrimSpace(reasonCode) != "" {
		alert.ReasonCode = strings.TrimSpace(reasonCode)
	}
	if err := s.Repository.SaveAlert(ctx, alert); err != nil {
		return Alert{}, fmt.Errorf("resolve alert: %w", err)
	}
	if err := s.recordEvent(ctx, alert, candidateID, EventResolved, actorSubjectType, actorSubjectID, reasonMetadata(reasonCode)); err != nil {
		return Alert{}, err
	}
	return alert, nil
}

func (s Service) loadAlert(ctx context.Context, alertID string) (Alert, error) {
	if s.Repository == nil {
		return Alert{}, fmt.Errorf("alert repository is required")
	}
	alertID = strings.TrimSpace(alertID)
	if alertID == "" {
		return Alert{}, fmt.Errorf("alert_id is required")
	}
	alert, err := s.Repository.GetAlert(ctx, alertID)
	if err != nil {
		return Alert{}, fmt.Errorf("load alert: %w", err)
	}
	return alert, nil
}

func (s Service) recordEvent(ctx context.Context, alert Alert, candidateID string, eventType EventType, actorSubjectType string, actorSubjectID string, metadata json.RawMessage) error {
	event := AlertEvent{
		AlertID:          alert.AlertID,
		CandidateID:      strings.TrimSpace(candidateID),
		EventType:        eventType,
		ActorSubjectType: actorType(actorSubjectType),
		ActorSubjectID:   actorID(actorSubjectID),
		Metadata:         normalizeJSON(metadata),
		OccurredAt:       s.now(),
	}
	if err := validateAlertEvent(event); err != nil {
		return err
	}
	if err := s.Repository.RecordAlertEvent(ctx, event); err != nil {
		return fmt.Errorf("record alert event: %w", err)
	}
	return nil
}

func normalizeCandidate(candidate Candidate, now time.Time) Candidate {
	candidate.CandidateID = strings.TrimSpace(candidate.CandidateID)
	candidate.EventType = strings.TrimSpace(candidate.EventType)
	candidate.AccountID = strings.TrimSpace(candidate.AccountID)
	candidate.SiteID = strings.TrimSpace(candidate.SiteID)
	candidate.DeviceID = strings.TrimSpace(candidate.DeviceID)
	candidate.Source = strings.TrimSpace(candidate.Source)
	candidate.PreviousState = normalizeJSON(candidate.PreviousState)
	candidate.NewState = normalizeJSON(candidate.NewState)
	candidate.Metadata = normalizeJSON(candidate.Metadata)
	if candidate.ReceivedAt.IsZero() {
		candidate.ReceivedAt = now
	}
	return candidate
}

func validateCandidate(candidate Candidate) error {
	if candidate.CandidateID == "" || candidate.EventType == "" {
		return fmt.Errorf("candidate_id and event_type are required")
	}
	if candidate.AccountID == "" || candidate.SiteID == "" {
		return fmt.Errorf("account_id and site_id are required")
	}
	if candidate.Source == "" {
		return fmt.Errorf("candidate source is required")
	}
	if candidate.OccurredAt.IsZero() {
		return fmt.Errorf("occurred_at is required")
	}
	if !json.Valid(candidate.PreviousState) || !json.Valid(candidate.NewState) || !json.Valid(candidate.Metadata) {
		return fmt.Errorf("candidate JSON fields must be valid")
	}
	return nil
}

func validateVerification(verification CandidateVerification) error {
	if strings.TrimSpace(verification.AlertKey) == "" {
		return fmt.Errorf("alert_key is required")
	}
	if verification.Healthy {
		return nil
	}
	if verification.Family == "" || verification.Severity == "" || strings.TrimSpace(verification.Title) == "" || strings.TrimSpace(verification.Detail) == "" {
		return fmt.Errorf("unhealthy alert verification requires family, severity, title, and detail")
	}
	if !json.Valid(normalizeJSON(verification.Metadata)) {
		return fmt.Errorf("verification metadata must be valid JSON")
	}
	return nil
}

func validateAlert(alert Alert) error {
	if alert.AlertID == "" || alert.AlertKey == "" {
		return fmt.Errorf("alert_id and alert_key are required")
	}
	if alert.AccountID == "" || alert.SiteID == "" {
		return fmt.Errorf("account_id and site_id are required")
	}
	switch alert.Family {
	case FamilyDeviceDisconnected, FamilyBackupFailed, FamilyBackupOverdue, FamilyAppUpdateAttention:
	default:
		return fmt.Errorf("unsupported alert family %q", alert.Family)
	}
	switch alert.Severity {
	case SeverityCritical, SeverityWarning, SeverityInfo:
	default:
		return fmt.Errorf("unsupported alert severity %q", alert.Severity)
	}
	if alert.Status != StatusActive && alert.Status != StatusResolved {
		return fmt.Errorf("unsupported alert status %q", alert.Status)
	}
	if alert.Title == "" || alert.Detail == "" || alert.ReasonCode == "" {
		return fmt.Errorf("title, detail, and reason_code are required")
	}
	if alert.OccurrenceCount <= 0 {
		return fmt.Errorf("occurrence_count must be positive")
	}
	return nil
}

func validateAlertEvent(event AlertEvent) error {
	if event.AlertID == "" {
		return fmt.Errorf("alert_id is required")
	}
	switch event.EventType {
	case EventCreated, EventUpdated, EventResolved, EventAcknowledged, EventSnoozed:
	default:
		return fmt.Errorf("unsupported alert event type %q", event.EventType)
	}
	if event.ActorSubjectType == "" || event.ActorSubjectID == "" {
		return fmt.Errorf("alert event actor is required")
	}
	if !json.Valid(event.Metadata) {
		return fmt.Errorf("alert event metadata must be valid JSON")
	}
	return nil
}

func normalizeJSON(value json.RawMessage) json.RawMessage {
	if len(value) == 0 {
		return json.RawMessage(`{}`)
	}
	return value
}

func reasonMetadata(reasonCode string) json.RawMessage {
	return mustJSON(map[string]string{"reason_code": strings.TrimSpace(reasonCode)})
}

func mustJSON(value any) json.RawMessage {
	payload, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return payload
}

func actorType(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "system"
	}
	return value
}

func actorID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "system"
	}
	return value
}

func (s Service) now() time.Time {
	if s.Clock != nil {
		return s.Clock().UTC()
	}
	return time.Now().UTC()
}
