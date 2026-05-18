package edgestate

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type StateKey string

const (
	StateKeyPublishPolicy StateKey = "publish_policy"
	StateKeyUpdate        StateKey = "update"
)

type DesiredState struct {
	DeviceID              string
	StateKey              StateKey
	DesiredVersion        string
	Desired               json.RawMessage
	ConvergenceDeadlineAt *time.Time
}

type Projection struct {
	DeviceID          string
	ProjectionKey     StateKey
	DesiredVersion    string
	ReportedVersion   string
	Projection        json.RawMessage
	ConvergenceStatus string
	LastReportedAt    *time.Time
}

type Repository interface {
	UpsertDesiredState(ctx context.Context, state DesiredState) error
	UpsertProjection(ctx context.Context, projection Projection) error
}

type ShadowAdapter interface {
	PutDesired(ctx context.Context, state DesiredState) error
}

type Service struct {
	Repository Repository
	Adapter    ShadowAdapter
}

func (s Service) PutDesired(ctx context.Context, state DesiredState) error {
	if err := validateDesiredState(state); err != nil {
		return err
	}
	if s.Repository == nil {
		return fmt.Errorf("edge state repository is required")
	}
	if err := s.Repository.UpsertDesiredState(ctx, state); err != nil {
		return fmt.Errorf("store desired edge state: %w", err)
	}
	if s.Adapter == nil {
		return nil
	}
	if err := s.Adapter.PutDesired(ctx, state); err != nil {
		return fmt.Errorf("publish desired edge state: %w", err)
	}
	return nil
}

func validateDesiredState(state DesiredState) error {
	if strings.TrimSpace(state.DeviceID) == "" {
		return fmt.Errorf("device_id is required")
	}
	switch state.StateKey {
	case StateKeyPublishPolicy, StateKeyUpdate:
	default:
		return fmt.Errorf("unsupported edge state key %q", state.StateKey)
	}
	if strings.TrimSpace(state.DesiredVersion) == "" {
		return fmt.Errorf("desired_version is required")
	}
	if len(state.Desired) == 0 || !json.Valid(state.Desired) {
		return fmt.Errorf("desired_json must be valid JSON")
	}
	return nil
}
