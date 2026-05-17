package ports

import (
	"context"
	"fmt"
	"sync"
)

type FakeAccountSiteRepository struct {
	mu       sync.Mutex
	Accounts map[AccountID]AccountRef
	Sites    map[SiteID]SiteRef
}

func NewFakeAccountSiteRepository() *FakeAccountSiteRepository {
	return &FakeAccountSiteRepository{
		Accounts: map[AccountID]AccountRef{},
		Sites:    map[SiteID]SiteRef{},
	}
}

func (r *FakeAccountSiteRepository) GetAccount(_ context.Context, accountID AccountID) (AccountRef, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	account, ok := r.Accounts[accountID]
	if !ok {
		return AccountRef{}, fmt.Errorf("account %s not found", accountID)
	}
	return account, nil
}

func (r *FakeAccountSiteRepository) GetSite(_ context.Context, siteID SiteID) (SiteRef, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	site, ok := r.Sites[siteID]
	if !ok {
		return SiteRef{}, fmt.Errorf("site %s not found", siteID)
	}
	return site, nil
}

type FakeDeviceRegistryRepository struct {
	mu          sync.Mutex
	Authorities map[CredentialIdentity]DeviceAuthority
	Credentials []DeviceCredentialRecord
}

func NewFakeDeviceRegistryRepository() *FakeDeviceRegistryRepository {
	return &FakeDeviceRegistryRepository{
		Authorities: map[CredentialIdentity]DeviceAuthority{},
	}
}

func (r *FakeDeviceRegistryRepository) ResolveCredential(_ context.Context, identity CredentialIdentity) (DeviceAuthority, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	authority, ok := r.Authorities[identity]
	if !ok {
		return DeviceAuthority{}, fmt.Errorf("credential identity not found")
	}
	return authority, nil
}

func (r *FakeDeviceRegistryRepository) RecordCredential(_ context.Context, record DeviceCredentialRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Credentials = append(r.Credentials, record)
	return nil
}

type FakeTelemetryRepository struct {
	mu           sync.Mutex
	LatestStates map[DeviceID]LatestTelemetryState
	Events       []TelemetryEvent
	Failures     []TelemetryFailure
}

func NewFakeTelemetryRepository() *FakeTelemetryRepository {
	return &FakeTelemetryRepository{
		LatestStates: map[DeviceID]LatestTelemetryState{},
	}
}

func (r *FakeTelemetryRepository) UpsertLatestState(_ context.Context, state LatestTelemetryState) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.LatestStates[state.DeviceID] = state
	return nil
}

func (r *FakeTelemetryRepository) InsertTelemetryEvent(_ context.Context, event TelemetryEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Events = append(r.Events, event)
	return nil
}

func (r *FakeTelemetryRepository) RecordFailure(_ context.Context, failure TelemetryFailure) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Failures = append(r.Failures, failure)
	return nil
}

type FakeAuditRepository struct {
	mu     sync.Mutex
	Events []AuditEvent
}

func (r *FakeAuditRepository) RecordAuditEvent(_ context.Context, event AuditEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Events = append(r.Events, event)
	return nil
}
