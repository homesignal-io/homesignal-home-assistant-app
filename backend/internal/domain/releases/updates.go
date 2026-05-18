package releases

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/homesignal-io/homesignal-home-assistant-app/backend/internal/domain/edgestate"
)

const (
	RolloutTargetDevice RolloutTargetType = "device"
	RolloutTargetCohort RolloutTargetType = "cohort"

	RolloutStatusDraft     RolloutStatus = "draft"
	RolloutStatusActive    RolloutStatus = "active"
	RolloutStatusPaused    RolloutStatus = "paused"
	RolloutStatusCompleted RolloutStatus = "completed"
	RolloutStatusCanceled  RolloutStatus = "canceled"

	AssignmentStatusAssigned         AssignmentStatus = "assigned"
	AssignmentStatusProjected        AssignmentStatus = "projected"
	AssignmentStatusProjectionFailed AssignmentStatus = "projection_failed"
	AssignmentStatusConverged        AssignmentStatus = "converged"
	AssignmentStatusFailed           AssignmentStatus = "failed"
	AssignmentStatusCanceled         AssignmentStatus = "canceled"
)

type RolloutTargetType string
type RolloutStatus string
type AssignmentStatus string
type IDGenerator func() string

type Rollout struct {
	RolloutID               string
	ChannelKey              ChannelKey
	TargetReleaseArtifactID string
	TargetVersion           string
	TargetType              RolloutTargetType
	CohortKey               string
	Status                  RolloutStatus
	CreatedByUserID         string
	ReasonCode              string
	Metadata                json.RawMessage
	ActivatedAt             *time.Time
	CompletedAt             *time.Time
	CanceledAt              *time.Time
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

type DeviceUpdateAssignment struct {
	DeviceUpdateAssignmentID string
	RolloutID                string
	AccountID                string
	SiteID                   string
	DeviceID                 string
	ChannelKey               ChannelKey
	DesiredVersion           string
	EdgeDesiredVersion       string
	Status                   AssignmentStatus
	ReasonCode               string
	AssignedByUserID         string
	Metadata                 json.RawMessage
	AssignedAt               time.Time
	ProjectedAt              *time.Time
	ConvergedAt              *time.Time
	FailedAt                 *time.Time
	UpdatedAt                time.Time
}

type RolloutAuditEvent struct {
	RolloutID                string
	DeviceUpdateAssignmentID string
	ActorSubjectType         string
	ActorSubjectID           string
	Action                   string
	Result                   string
	Metadata                 json.RawMessage
	CreatedAt                time.Time
}

type CreateRolloutRequest struct {
	RolloutID               string
	ChannelKey              ChannelKey
	TargetReleaseArtifactID string
	TargetVersion           string
	TargetType              RolloutTargetType
	CohortKey               string
	Status                  RolloutStatus
	CreatedByUserID         string
	ReasonCode              string
	Metadata                json.RawMessage
}

type AssignDeviceRequest struct {
	DeviceUpdateAssignmentID string
	RolloutID                string
	AccountID                string
	SiteID                   string
	DeviceID                 string
	AssignedByUserID         string
	Metadata                 json.RawMessage
}

type ChangeRolloutStatusRequest struct {
	RolloutID        string
	Status           RolloutStatus
	ActorSubjectType string
	ActorSubjectID   string
	ReasonCode       string
}

type UpdateRepository interface {
	CreateRollout(ctx context.Context, rollout Rollout) error
	GetRollout(ctx context.Context, rolloutID string) (Rollout, error)
	SaveRolloutStatus(ctx context.Context, rolloutID string, status RolloutStatus, changedAt time.Time, reasonCode string) error
	CreateDeviceAssignment(ctx context.Context, assignment DeviceUpdateAssignment) error
	SaveDeviceAssignmentProjection(ctx context.Context, assignmentID string, status AssignmentStatus, projectedAt *time.Time, reasonCode string) error
	UpsertDeviceUpdateStatus(ctx context.Context, status DeviceUpdateStatus) error
	UpsertUpdateProjection(ctx context.Context, projection edgestate.Projection) error
	RecordRolloutAudit(ctx context.Context, event RolloutAuditEvent) error
}

type EdgeStateWriter interface {
	PutDesired(ctx context.Context, state edgestate.DesiredState) error
}

type UpdateService struct {
	Repository     UpdateRepository
	EdgeState      EdgeStateWriter
	CommandCreator UpdateCommandCreator
	IDGenerator    IDGenerator
	Clock          Clock
}

func (s UpdateService) CreateRollout(ctx context.Context, req CreateRolloutRequest) (Rollout, error) {
	if s.Repository == nil {
		return Rollout{}, fmt.Errorf("update repository is required")
	}
	now := s.now()
	rolloutID, err := s.resolveID(req.RolloutID, "rollout id")
	if err != nil {
		return Rollout{}, err
	}
	status := req.Status
	if status == "" {
		status = RolloutStatusDraft
	}
	rollout := Rollout{
		RolloutID:               rolloutID,
		ChannelKey:              ChannelKey(strings.TrimSpace(string(req.ChannelKey))),
		TargetReleaseArtifactID: strings.TrimSpace(req.TargetReleaseArtifactID),
		TargetVersion:           strings.TrimSpace(req.TargetVersion),
		TargetType:              req.TargetType,
		CohortKey:               strings.TrimSpace(req.CohortKey),
		Status:                  status,
		CreatedByUserID:         strings.TrimSpace(req.CreatedByUserID),
		ReasonCode:              strings.TrimSpace(req.ReasonCode),
		Metadata:                normalizeJSON(req.Metadata),
		CreatedAt:               now,
		UpdatedAt:               now,
	}
	if rollout.Status == RolloutStatusActive {
		rollout.ActivatedAt = &now
	}
	if err := validateRollout(rollout); err != nil {
		return Rollout{}, err
	}
	if err := s.Repository.CreateRollout(ctx, rollout); err != nil {
		return Rollout{}, fmt.Errorf("create rollout: %w", err)
	}
	if err := s.audit(ctx, RolloutAuditEvent{
		RolloutID:        rollout.RolloutID,
		ActorSubjectType: actorTypeForUser(rollout.CreatedByUserID),
		ActorSubjectID:   actorIDForUser(rollout.CreatedByUserID),
		Action:           "rollout.create",
		Result:           "success",
		Metadata:         rollout.Metadata,
		CreatedAt:        now,
	}); err != nil {
		return Rollout{}, err
	}
	return rollout, nil
}

func (s UpdateService) AssignDevice(ctx context.Context, req AssignDeviceRequest) (DeviceUpdateAssignment, error) {
	if s.Repository == nil {
		return DeviceUpdateAssignment{}, fmt.Errorf("update repository is required")
	}
	if s.EdgeState == nil {
		return DeviceUpdateAssignment{}, fmt.Errorf("edge state writer is required")
	}
	now := s.now()
	rollout, err := s.Repository.GetRollout(ctx, strings.TrimSpace(req.RolloutID))
	if err != nil {
		return DeviceUpdateAssignment{}, fmt.Errorf("load rollout: %w", err)
	}
	if rollout.Status != RolloutStatusActive {
		return DeviceUpdateAssignment{}, fmt.Errorf("cannot assign device from %s rollout", rollout.Status)
	}
	assignmentID, err := s.resolveID(req.DeviceUpdateAssignmentID, "device update assignment id")
	if err != nil {
		return DeviceUpdateAssignment{}, err
	}
	assignment := DeviceUpdateAssignment{
		DeviceUpdateAssignmentID: assignmentID,
		RolloutID:                rollout.RolloutID,
		AccountID:                strings.TrimSpace(req.AccountID),
		SiteID:                   strings.TrimSpace(req.SiteID),
		DeviceID:                 strings.TrimSpace(req.DeviceID),
		ChannelKey:               rollout.ChannelKey,
		DesiredVersion:           rollout.TargetVersion,
		EdgeDesiredVersion:       rollout.TargetVersion,
		Status:                   AssignmentStatusAssigned,
		AssignedByUserID:         strings.TrimSpace(req.AssignedByUserID),
		Metadata:                 normalizeJSON(req.Metadata),
		AssignedAt:               now,
		UpdatedAt:                now,
	}
	if err := validateAssignment(assignment); err != nil {
		return DeviceUpdateAssignment{}, err
	}
	if err := s.Repository.CreateDeviceAssignment(ctx, assignment); err != nil {
		return DeviceUpdateAssignment{}, fmt.Errorf("create device update assignment: %w", err)
	}
	desired, err := marshalUpdateDesired(rollout)
	if err != nil {
		return DeviceUpdateAssignment{}, err
	}
	err = s.EdgeState.PutDesired(ctx, edgestate.DesiredState{
		DeviceID:       assignment.DeviceID,
		StateKey:       edgestate.StateKeyUpdate,
		DesiredVersion: assignment.EdgeDesiredVersion,
		Desired:        desired,
	})
	if err != nil {
		reasonCode := "edge_projection_failed"
		if saveErr := s.Repository.SaveDeviceAssignmentProjection(ctx, assignment.DeviceUpdateAssignmentID, AssignmentStatusProjectionFailed, nil, reasonCode); saveErr != nil {
			return assignment, fmt.Errorf("publish update desired state: %w; mark projection failed: %v", err, saveErr)
		}
		assignment.Status = AssignmentStatusProjectionFailed
		assignment.ReasonCode = reasonCode
		_ = s.audit(ctx, RolloutAuditEvent{
			RolloutID:                rollout.RolloutID,
			DeviceUpdateAssignmentID: assignment.DeviceUpdateAssignmentID,
			ActorSubjectType:         actorTypeForUser(assignment.AssignedByUserID),
			ActorSubjectID:           actorIDForUser(assignment.AssignedByUserID),
			Action:                   "device_update_assignment.project",
			Result:                   "failed",
			Metadata:                 json.RawMessage(`{"reason_code":"edge_projection_failed"}`),
			CreatedAt:                now,
		})
		return assignment, fmt.Errorf("publish update desired state: %w", err)
	}
	projectedAt := now
	if err := s.Repository.SaveDeviceAssignmentProjection(ctx, assignment.DeviceUpdateAssignmentID, AssignmentStatusProjected, &projectedAt, ""); err != nil {
		return assignment, fmt.Errorf("mark update assignment projected: %w", err)
	}
	assignment.Status = AssignmentStatusProjected
	assignment.ProjectedAt = &projectedAt
	if err := s.audit(ctx, RolloutAuditEvent{
		RolloutID:                rollout.RolloutID,
		DeviceUpdateAssignmentID: assignment.DeviceUpdateAssignmentID,
		ActorSubjectType:         actorTypeForUser(assignment.AssignedByUserID),
		ActorSubjectID:           actorIDForUser(assignment.AssignedByUserID),
		Action:                   "device_update_assignment.project",
		Result:                   "success",
		Metadata:                 assignment.Metadata,
		CreatedAt:                now,
	}); err != nil {
		return assignment, err
	}
	return assignment, nil
}

func (s UpdateService) ChangeRolloutStatus(ctx context.Context, req ChangeRolloutStatusRequest) error {
	if s.Repository == nil {
		return fmt.Errorf("update repository is required")
	}
	status := req.Status
	if status == "" {
		return fmt.Errorf("rollout status is required")
	}
	if !validRolloutStatus(status) {
		return fmt.Errorf("unsupported rollout status %q", status)
	}
	now := s.now()
	rolloutID := strings.TrimSpace(req.RolloutID)
	if rolloutID == "" {
		return fmt.Errorf("rollout_id is required")
	}
	if err := s.Repository.SaveRolloutStatus(ctx, rolloutID, status, now, strings.TrimSpace(req.ReasonCode)); err != nil {
		return fmt.Errorf("save rollout status: %w", err)
	}
	return s.audit(ctx, RolloutAuditEvent{
		RolloutID:        rolloutID,
		ActorSubjectType: strings.TrimSpace(req.ActorSubjectType),
		ActorSubjectID:   strings.TrimSpace(req.ActorSubjectID),
		Action:           "rollout.status_change",
		Result:           "success",
		Metadata:         mustMarshalJSON(map[string]string{"status": string(status), "reason_code": strings.TrimSpace(req.ReasonCode)}),
		CreatedAt:        now,
	})
}

func validateRollout(rollout Rollout) error {
	if rollout.RolloutID == "" || rollout.ChannelKey == "" || rollout.TargetVersion == "" {
		return fmt.Errorf("rollout_id, channel_key, and target_version are required")
	}
	if err := validateImmutableVersion(rollout.TargetVersion); err != nil {
		return err
	}
	switch rollout.TargetType {
	case RolloutTargetDevice:
	case RolloutTargetCohort:
		if rollout.CohortKey == "" {
			return fmt.Errorf("cohort_key is required for cohort rollouts")
		}
	default:
		return fmt.Errorf("unsupported rollout target type %q", rollout.TargetType)
	}
	if !validRolloutStatus(rollout.Status) {
		return fmt.Errorf("unsupported rollout status %q", rollout.Status)
	}
	if !json.Valid(rollout.Metadata) {
		return fmt.Errorf("rollout metadata must be valid JSON")
	}
	return nil
}

func validateAssignment(assignment DeviceUpdateAssignment) error {
	if assignment.DeviceUpdateAssignmentID == "" || assignment.RolloutID == "" {
		return fmt.Errorf("device_update_assignment_id and rollout_id are required")
	}
	if assignment.AccountID == "" || assignment.SiteID == "" || assignment.DeviceID == "" {
		return fmt.Errorf("account_id, site_id, and device_id are required")
	}
	if assignment.ChannelKey == "" || assignment.DesiredVersion == "" || assignment.EdgeDesiredVersion == "" {
		return fmt.Errorf("channel_key, desired_version, and edge_desired_version are required")
	}
	if err := validateImmutableVersion(assignment.DesiredVersion); err != nil {
		return err
	}
	if assignment.Status != AssignmentStatusAssigned {
		return fmt.Errorf("new assignments must start assigned")
	}
	if !json.Valid(assignment.Metadata) {
		return fmt.Errorf("assignment metadata must be valid JSON")
	}
	return nil
}

func validRolloutStatus(status RolloutStatus) bool {
	switch status {
	case RolloutStatusDraft, RolloutStatusActive, RolloutStatusPaused, RolloutStatusCompleted, RolloutStatusCanceled:
		return true
	default:
		return false
	}
}

func marshalUpdateDesired(rollout Rollout) (json.RawMessage, error) {
	payload, err := json.Marshal(map[string]any{
		"update": map[string]any{
			"desired_version": rollout.TargetVersion,
			"channel":         rollout.ChannelKey,
			"rollout_id":      rollout.RolloutID,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal update desired state: %w", err)
	}
	return payload, nil
}

func (s UpdateService) audit(ctx context.Context, event RolloutAuditEvent) error {
	if s.Repository == nil {
		return fmt.Errorf("update repository is required")
	}
	if event.ActorSubjectType == "" || event.ActorSubjectID == "" {
		return fmt.Errorf("audit actor is required")
	}
	event.Metadata = normalizeJSON(event.Metadata)
	if !json.Valid(event.Metadata) {
		return fmt.Errorf("audit metadata must be valid JSON")
	}
	if err := s.Repository.RecordRolloutAudit(ctx, event); err != nil {
		return fmt.Errorf("record rollout audit: %w", err)
	}
	return nil
}

func (s UpdateService) resolveID(candidate string, label string) (string, error) {
	candidate = strings.TrimSpace(candidate)
	if candidate != "" {
		return candidate, nil
	}
	if s.IDGenerator == nil {
		return "", fmt.Errorf("%s is required", label)
	}
	candidate = strings.TrimSpace(s.IDGenerator())
	if candidate == "" {
		return "", fmt.Errorf("%s is required", label)
	}
	return candidate, nil
}

func (s UpdateService) now() time.Time {
	if s.Clock != nil {
		return s.Clock().UTC()
	}
	return time.Now().UTC()
}

func actorTypeForUser(userID string) string {
	if strings.TrimSpace(userID) == "" {
		return "system"
	}
	return "user"
}

func actorIDForUser(userID string) string {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return "system"
	}
	return userID
}

func mustMarshalJSON(value any) json.RawMessage {
	payload, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return payload
}
