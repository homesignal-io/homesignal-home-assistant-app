package releases

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/homesignal-io/homesignal-home-assistant-app/backend/internal/domain/edgestate"
)

func TestUpdateServiceCreatesAuditedRollout(t *testing.T) {
	repo := newFakeUpdateRepository()
	service := UpdateService{
		Repository:  repo,
		IDGenerator: sequenceIDs("rollout_123"),
		Clock:       fixedClock,
	}

	rollout, err := service.CreateRollout(context.Background(), CreateRolloutRequest{
		ChannelKey:      ChannelStable,
		TargetVersion:   "0.1.4",
		TargetType:      RolloutTargetCohort,
		CohortKey:       "candidate_canary",
		Status:          RolloutStatusActive,
		CreatedByUserID: "user_123",
		Metadata:        json.RawMessage(`{"ticket":"REL-1"}`),
	})
	if err != nil {
		t.Fatalf("create rollout: %v", err)
	}
	if rollout.RolloutID != "rollout_123" || rollout.ActivatedAt == nil {
		t.Fatalf("unexpected rollout: %#v", rollout)
	}
	if len(repo.auditEvents) != 1 || repo.auditEvents[0].Action != "rollout.create" {
		t.Fatalf("expected rollout create audit event, got %#v", repo.auditEvents)
	}
}

func TestUpdateServiceAssignsDeviceAndProjectsEdgeUpdate(t *testing.T) {
	repo := newFakeUpdateRepository()
	edge := &fakeEdgeStateWriter{}
	service := UpdateService{
		Repository:  repo,
		EdgeState:   edge,
		IDGenerator: sequenceIDs("assignment_123"),
		Clock:       fixedClock,
	}
	repo.rollouts["rollout_123"] = Rollout{
		RolloutID:     "rollout_123",
		ChannelKey:    ChannelStable,
		TargetVersion: "0.1.4",
		TargetType:    RolloutTargetDevice,
		Status:        RolloutStatusActive,
	}

	assignment, err := service.AssignDevice(context.Background(), AssignDeviceRequest{
		RolloutID:        "rollout_123",
		AccountID:        "acct_123",
		SiteID:           "site_123",
		DeviceID:         "dev_123",
		AssignedByUserID: "user_123",
	})
	if err != nil {
		t.Fatalf("assign device: %v", err)
	}
	if assignment.Status != AssignmentStatusProjected {
		t.Fatalf("expected projected assignment, got %q", assignment.Status)
	}
	if len(repo.assignments) != 1 {
		t.Fatalf("expected DB assignment first, got %d", len(repo.assignments))
	}
	if len(edge.desired) != 1 {
		t.Fatalf("expected one edge desired write, got %d", len(edge.desired))
	}
	desired := edge.desired[0]
	if desired.StateKey != edgestate.StateKeyUpdate || desired.DeviceID != "dev_123" || desired.DesiredVersion != "0.1.4" {
		t.Fatalf("unexpected edge desired state: %#v", desired)
	}
	if !strings.Contains(string(desired.Desired), `"rollout_id":"rollout_123"`) {
		t.Fatalf("desired update payload missing rollout id: %s", desired.Desired)
	}
	if repo.projectionStatuses["assignment_123"] != AssignmentStatusProjected {
		t.Fatalf("expected projected status save, got %q", repo.projectionStatuses["assignment_123"])
	}
}

func TestUpdateServiceKeepsAssignmentWhenEdgeProjectionFails(t *testing.T) {
	repo := newFakeUpdateRepository()
	edge := &fakeEdgeStateWriter{err: errors.New("shadow unavailable")}
	service := UpdateService{
		Repository:  repo,
		EdgeState:   edge,
		IDGenerator: sequenceIDs("assignment_123"),
		Clock:       fixedClock,
	}
	repo.rollouts["rollout_123"] = Rollout{
		RolloutID:     "rollout_123",
		ChannelKey:    ChannelStable,
		TargetVersion: "0.1.4",
		TargetType:    RolloutTargetDevice,
		Status:        RolloutStatusActive,
	}

	assignment, err := service.AssignDevice(context.Background(), AssignDeviceRequest{
		RolloutID: "rollout_123",
		AccountID: "acct_123",
		SiteID:    "site_123",
		DeviceID:  "dev_123",
	})
	if err == nil || !strings.Contains(err.Error(), "publish update desired state") {
		t.Fatalf("expected projection error, got %v", err)
	}
	if assignment.Status != AssignmentStatusProjectionFailed {
		t.Fatalf("expected projection_failed assignment, got %q", assignment.Status)
	}
	if len(repo.assignments) != 1 {
		t.Fatalf("expected assignment to remain stored, got %d", len(repo.assignments))
	}
	if repo.projectionStatuses["assignment_123"] != AssignmentStatusProjectionFailed {
		t.Fatalf("expected projection_failed status save, got %q", repo.projectionStatuses["assignment_123"])
	}
}

func TestUpdateServiceChangesRolloutStatusWithAuditEvent(t *testing.T) {
	repo := newFakeUpdateRepository()
	service := UpdateService{Repository: repo, Clock: fixedClock}

	err := service.ChangeRolloutStatus(context.Background(), ChangeRolloutStatusRequest{
		RolloutID:        "rollout_123",
		Status:           RolloutStatusPaused,
		ActorSubjectType: "user",
		ActorSubjectID:   "user_123",
		ReasonCode:       "pause_canary",
	})
	if err != nil {
		t.Fatalf("change status: %v", err)
	}
	if repo.rolloutStatuses["rollout_123"] != RolloutStatusPaused {
		t.Fatalf("expected paused status save")
	}
	if len(repo.auditEvents) != 1 || repo.auditEvents[0].Action != "rollout.status_change" {
		t.Fatalf("expected status audit event, got %#v", repo.auditEvents)
	}
}

func TestUpdateServiceRejectsFloatingRolloutTarget(t *testing.T) {
	repo := newFakeUpdateRepository()
	service := UpdateService{
		Repository:  repo,
		IDGenerator: sequenceIDs("rollout_123"),
		Clock:       fixedClock,
	}

	_, err := service.CreateRollout(context.Background(), CreateRolloutRequest{
		ChannelKey:    ChannelStable,
		TargetVersion: "latest",
		TargetType:    RolloutTargetDevice,
	})
	if err == nil || !strings.Contains(err.Error(), "immutable") {
		t.Fatalf("expected immutable target version error, got %v", err)
	}
}

func sequenceIDs(ids ...string) IDGenerator {
	index := 0
	return func() string {
		if index >= len(ids) {
			return ""
		}
		id := ids[index]
		index++
		return id
	}
}

type fakeUpdateRepository struct {
	rollouts           map[string]Rollout
	rolloutStatuses    map[string]RolloutStatus
	assignments        map[string]DeviceUpdateAssignment
	projectionStatuses map[string]AssignmentStatus
	auditEvents        []RolloutAuditEvent
}

func newFakeUpdateRepository() *fakeUpdateRepository {
	return &fakeUpdateRepository{
		rollouts:           map[string]Rollout{},
		rolloutStatuses:    map[string]RolloutStatus{},
		assignments:        map[string]DeviceUpdateAssignment{},
		projectionStatuses: map[string]AssignmentStatus{},
	}
}

func (r *fakeUpdateRepository) CreateRollout(_ context.Context, rollout Rollout) error {
	r.rollouts[rollout.RolloutID] = rollout
	return nil
}

func (r *fakeUpdateRepository) GetRollout(_ context.Context, rolloutID string) (Rollout, error) {
	rollout, ok := r.rollouts[rolloutID]
	if !ok {
		return Rollout{}, ErrNotFound
	}
	return rollout, nil
}

func (r *fakeUpdateRepository) SaveRolloutStatus(_ context.Context, rolloutID string, status RolloutStatus, _ time.Time, _ string) error {
	r.rolloutStatuses[rolloutID] = status
	return nil
}

func (r *fakeUpdateRepository) CreateDeviceAssignment(_ context.Context, assignment DeviceUpdateAssignment) error {
	r.assignments[assignment.DeviceUpdateAssignmentID] = assignment
	return nil
}

func (r *fakeUpdateRepository) SaveDeviceAssignmentProjection(_ context.Context, assignmentID string, status AssignmentStatus, _ *time.Time, _ string) error {
	r.projectionStatuses[assignmentID] = status
	return nil
}

func (r *fakeUpdateRepository) RecordRolloutAudit(_ context.Context, event RolloutAuditEvent) error {
	r.auditEvents = append(r.auditEvents, event)
	return nil
}

type fakeEdgeStateWriter struct {
	desired []edgestate.DesiredState
	err     error
}

func (w *fakeEdgeStateWriter) PutDesired(_ context.Context, state edgestate.DesiredState) error {
	w.desired = append(w.desired, state)
	return w.err
}
