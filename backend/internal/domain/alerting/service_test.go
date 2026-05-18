package alerting

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestServiceDeduplicatesAlertCandidates(t *testing.T) {
	repo := newFakeRepository()
	service := testService(repo, fakeVerifier{verification: disconnectedVerification()})
	candidate := disconnectedCandidate()

	first, err := service.IntakeCandidate(context.Background(), candidate)
	if err != nil {
		t.Fatalf("first intake: %v", err)
	}
	second, err := service.IntakeCandidate(context.Background(), candidate)
	if err != nil {
		t.Fatalf("second intake: %v", err)
	}
	if first.Duplicate || !second.Duplicate || len(repo.alerts) != 1 {
		t.Fatalf("expected one alert and duplicate second result, first=%#v second=%#v alerts=%d", first, second, len(repo.alerts))
	}
}

func TestServiceIgnoresStaleHealthyCandidate(t *testing.T) {
	repo := newFakeRepository()
	service := testService(repo, fakeVerifier{verification: CandidateVerification{
		Healthy:  true,
		AlertKey: "device_disconnected:dev_123",
	}})

	result, err := service.IntakeCandidate(context.Background(), disconnectedCandidate())
	if err != nil {
		t.Fatalf("intake: %v", err)
	}
	if !result.Ignored || len(repo.alerts) != 0 {
		t.Fatalf("expected stale healthy candidate to be ignored, result=%#v alerts=%d", result, len(repo.alerts))
	}
}

func TestServiceCreatesDisconnectedAlert(t *testing.T) {
	repo := newFakeRepository()
	service := testService(repo, fakeVerifier{verification: disconnectedVerification()})

	result, err := service.IntakeCandidate(context.Background(), disconnectedCandidate())
	if err != nil {
		t.Fatalf("intake: %v", err)
	}
	if result.Alert.AlertID != "alert_123" || result.Alert.Family != FamilyDeviceDisconnected {
		t.Fatalf("unexpected alert %#v", result.Alert)
	}
	if len(repo.events) != 1 || repo.events[0].EventType != EventCreated {
		t.Fatalf("expected created event, got %#v", repo.events)
	}
}

func TestServiceResolvesActiveAlertOnRecoveryCandidate(t *testing.T) {
	repo := newFakeRepository()
	active := Alert{
		AlertID:         "alert_123",
		AlertKey:        "device_disconnected:dev_123",
		AccountID:       "acct_123",
		SiteID:          "site_123",
		DeviceID:        "dev_123",
		Family:          FamilyDeviceDisconnected,
		Severity:        SeverityCritical,
		Status:          StatusActive,
		Title:           "Device disconnected",
		Detail:          "Device is offline.",
		ReasonCode:      "presence_offline",
		OccurrenceCount: 1,
	}
	repo.alerts[active.AlertID] = active
	repo.activeByKey[active.AlertKey] = active.AlertID
	service := testService(repo, fakeVerifier{verification: CandidateVerification{
		Healthy:  true,
		AlertKey: active.AlertKey,
	}})

	result, err := service.IntakeCandidate(context.Background(), recoveryCandidate())
	if err != nil {
		t.Fatalf("intake recovery: %v", err)
	}
	if !result.Resolved || repo.alerts[active.AlertID].Status != StatusResolved {
		t.Fatalf("expected resolved alert, result=%#v alert=%#v", result, repo.alerts[active.AlertID])
	}
	if len(repo.events) != 1 || repo.events[0].EventType != EventResolved {
		t.Fatalf("expected resolved event, got %#v", repo.events)
	}
}

func TestServiceAcknowledgesAndSnoozesAlert(t *testing.T) {
	repo := newFakeRepository()
	active := disconnectedAlert()
	repo.alerts[active.AlertID] = active
	service := testService(repo, fakeVerifier{})

	acked, err := service.AcknowledgeAlert(context.Background(), AlertActionRequest{
		AlertID:        active.AlertID,
		ActorSubjectID: "user_123",
		ActorType:      "user",
		ReasonCode:     "seen",
	})
	if err != nil {
		t.Fatalf("acknowledge: %v", err)
	}
	if acked.AcknowledgedAt == nil || acked.AcknowledgedByUserID != "user_123" {
		t.Fatalf("expected ack fields, got %#v", acked)
	}
	snoozedUntil := fixedClock().Add(time.Hour)
	snoozed, err := service.SnoozeAlert(context.Background(), AlertActionRequest{
		AlertID:        active.AlertID,
		ActorSubjectID: "user_123",
		ActorType:      "user",
		SnoozedUntil:   snoozedUntil,
		ReasonCode:     "maintenance",
	})
	if err != nil {
		t.Fatalf("snooze: %v", err)
	}
	if snoozed.SnoozedUntil == nil || !snoozed.SnoozedUntil.Equal(snoozedUntil) {
		t.Fatalf("expected snooze fields, got %#v", snoozed)
	}
}

func testService(repo *fakeRepository, verifier fakeVerifier) Service {
	return Service{
		Repository: repo,
		Verifier:   verifier,
		IDGenerator: func() string {
			return "alert_123"
		},
		Clock: fixedClock,
	}
}

func fixedClock() time.Time {
	return time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
}

func disconnectedCandidate() Candidate {
	return Candidate{
		CandidateID:   "presence-dev_123-2026-05-18T12:00:00Z",
		EventType:     "device.presence_changed",
		AccountID:     "acct_123",
		SiteID:        "site_123",
		DeviceID:      "dev_123",
		Source:        "telemetry_ingest",
		PreviousState: json.RawMessage(`{"status":"ONLINE"}`),
		NewState:      json.RawMessage(`{"status":"OFFLINE"}`),
		OccurredAt:    fixedClock(),
	}
}

func recoveryCandidate() Candidate {
	candidate := disconnectedCandidate()
	candidate.CandidateID = "presence-dev_123-2026-05-18T12:05:00Z"
	candidate.PreviousState = json.RawMessage(`{"status":"OFFLINE"}`)
	candidate.NewState = json.RawMessage(`{"status":"ONLINE"}`)
	candidate.OccurredAt = fixedClock().Add(5 * time.Minute)
	return candidate
}

func disconnectedVerification() CandidateVerification {
	return CandidateVerification{
		Healthy:    false,
		AlertKey:   "device_disconnected:dev_123",
		Family:     FamilyDeviceDisconnected,
		Severity:   SeverityCritical,
		Title:      "Device disconnected",
		Detail:     "HomeSignal has not seen this device recently.",
		ReasonCode: "presence_offline",
		Metadata:   json.RawMessage(`{"source":"presence"}`),
	}
}

func disconnectedAlert() Alert {
	return Alert{
		AlertID:         "alert_123",
		AlertKey:        "device_disconnected:dev_123",
		AccountID:       "acct_123",
		SiteID:          "site_123",
		DeviceID:        "dev_123",
		Family:          FamilyDeviceDisconnected,
		Severity:        SeverityCritical,
		Status:          StatusActive,
		Title:           "Device disconnected",
		Detail:          "HomeSignal has not seen this device recently.",
		ReasonCode:      "presence_offline",
		OccurrenceCount: 1,
	}
}

type fakeVerifier struct {
	verification CandidateVerification
	err          error
}

func (v fakeVerifier) VerifyCandidate(context.Context, Candidate) (CandidateVerification, error) {
	if v.err != nil {
		return CandidateVerification{}, v.err
	}
	return v.verification, nil
}

type fakeRepository struct {
	candidates  map[string]Candidate
	alerts      map[string]Alert
	activeByKey map[string]string
	events      []AlertEvent
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{
		candidates:  map[string]Candidate{},
		alerts:      map[string]Alert{},
		activeByKey: map[string]string{},
	}
}

func (r *fakeRepository) TryRecordCandidate(_ context.Context, candidate Candidate) (bool, error) {
	if _, ok := r.candidates[candidate.CandidateID]; ok {
		return false, nil
	}
	r.candidates[candidate.CandidateID] = candidate
	return true, nil
}

func (r *fakeRepository) GetActiveAlertByKey(_ context.Context, alertKey string) (Alert, error) {
	alertID, ok := r.activeByKey[alertKey]
	if !ok {
		return Alert{}, ErrNotFound
	}
	alert := r.alerts[alertID]
	if alert.Status != StatusActive {
		return Alert{}, ErrNotFound
	}
	return alert, nil
}

func (r *fakeRepository) GetAlert(_ context.Context, alertID string) (Alert, error) {
	alert, ok := r.alerts[alertID]
	if !ok {
		return Alert{}, ErrNotFound
	}
	return alert, nil
}

func (r *fakeRepository) CreateAlert(_ context.Context, alert Alert) error {
	if _, ok := r.alerts[alert.AlertID]; ok {
		return errors.New("duplicate alert")
	}
	r.alerts[alert.AlertID] = alert
	if alert.Status == StatusActive {
		r.activeByKey[alert.AlertKey] = alert.AlertID
	}
	return nil
}

func (r *fakeRepository) SaveAlert(_ context.Context, alert Alert) error {
	r.alerts[alert.AlertID] = alert
	if alert.Status == StatusActive {
		r.activeByKey[alert.AlertKey] = alert.AlertID
	} else {
		delete(r.activeByKey, alert.AlertKey)
	}
	return nil
}

func (r *fakeRepository) RecordAlertEvent(_ context.Context, event AlertEvent) error {
	r.events = append(r.events, event)
	return nil
}
