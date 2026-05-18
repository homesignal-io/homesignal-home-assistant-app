package edgestate

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestServiceStoresDesiredStateBeforeAdapterWrite(t *testing.T) {
	var calls []string
	repo := &fakeRepository{calls: &calls}
	adapter := &fakeAdapter{calls: &calls}
	service := Service{Repository: repo, Adapter: adapter}
	state := DesiredState{
		DeviceID:       "dev_123",
		StateKey:       StateKeyPublishPolicy,
		DesiredVersion: "ppv_v0_default_free",
		Desired:        json.RawMessage(`{"publish_policy":{"version":"ppv_v0_default_free"}}`),
	}

	if err := service.PutDesired(context.Background(), state); err != nil {
		t.Fatalf("put desired state: %v", err)
	}
	if len(repo.desired) != 1 {
		t.Fatalf("expected one stored desired state, got %d", len(repo.desired))
	}
	if len(adapter.desired) != 1 {
		t.Fatalf("expected one adapter write, got %d", len(adapter.desired))
	}
	if got := strings.Join(calls, ","); got != "repo.upsert,adapter.put" {
		t.Fatalf("unexpected call order %q", got)
	}
}

func TestServiceKeepsStoredDesiredStateWhenAdapterFails(t *testing.T) {
	repo := &fakeRepository{}
	adapter := &fakeAdapter{err: errors.New("shadow unavailable")}
	service := Service{Repository: repo, Adapter: adapter}

	err := service.PutDesired(context.Background(), DesiredState{
		DeviceID:       "dev_123",
		StateKey:       StateKeyUpdate,
		DesiredVersion: "0.1.4",
		Desired:        json.RawMessage(`{"update":{"desired_version":"0.1.4","channel":"stable"}}`),
	})
	if err == nil || !strings.Contains(err.Error(), "publish desired edge state") {
		t.Fatalf("expected adapter error, got %v", err)
	}
	if len(repo.desired) != 1 {
		t.Fatalf("expected desired state to remain stored, got %d records", len(repo.desired))
	}
}

func TestServiceRejectsInvalidDesiredState(t *testing.T) {
	service := Service{Repository: &fakeRepository{}}
	tests := []DesiredState{
		{StateKey: StateKeyPublishPolicy, DesiredVersion: "v1", Desired: json.RawMessage(`{}`)},
		{DeviceID: "dev_123", StateKey: "debug", DesiredVersion: "v1", Desired: json.RawMessage(`{}`)},
		{DeviceID: "dev_123", StateKey: StateKeyPublishPolicy, Desired: json.RawMessage(`{}`)},
		{DeviceID: "dev_123", StateKey: StateKeyPublishPolicy, DesiredVersion: "v1", Desired: json.RawMessage(`{`)},
	}

	for _, tt := range tests {
		if err := service.PutDesired(context.Background(), tt); err == nil {
			t.Fatalf("expected invalid state %#v to fail", tt)
		}
	}
}

type fakeRepository struct {
	calls       *[]string
	desired     []DesiredState
	projections []Projection
}

func (r *fakeRepository) UpsertDesiredState(_ context.Context, state DesiredState) error {
	if r.calls != nil {
		*r.calls = append(*r.calls, "repo.upsert")
	}
	r.desired = append(r.desired, state)
	return nil
}

func (r *fakeRepository) UpsertProjection(_ context.Context, projection Projection) error {
	if r.calls != nil {
		*r.calls = append(*r.calls, "repo.projection")
	}
	r.projections = append(r.projections, projection)
	return nil
}

type fakeAdapter struct {
	calls   *[]string
	desired []DesiredState
	err     error
}

func (a *fakeAdapter) PutDesired(_ context.Context, state DesiredState) error {
	if a.calls != nil {
		*a.calls = append(*a.calls, "adapter.put")
	}
	a.desired = append(a.desired, state)
	return a.err
}
