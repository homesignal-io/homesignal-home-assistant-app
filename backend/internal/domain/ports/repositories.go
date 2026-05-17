package ports

import (
	"context"
	"encoding/json"
	"time"
)

type AccountID string
type SiteID string
type DeviceID string
type UserID string

type AccountRef struct {
	ID          AccountID
	DisplayName string
	Status      string
}

type SiteRef struct {
	ID          SiteID
	AccountID   AccountID
	DisplayName string
	Status      string
}

type UserSubject struct {
	ID         UserID
	CognitoSub string
	Email      string
	Status     string
}

type AccountSiteRepository interface {
	GetAccount(ctx context.Context, accountID AccountID) (AccountRef, error)
	GetSite(ctx context.Context, siteID SiteID) (SiteRef, error)
}

type AuthRepository interface {
	GetUserByCognitoSub(ctx context.Context, cognitoSub string) (UserSubject, error)
	ListPermissionKeys(ctx context.Context, userID UserID, accountID AccountID, siteID SiteID) ([]string, error)
}

type CredentialIdentity struct {
	CertificateFingerprint string
	CertificateSerial      string
	CertificateIssuer      string
}

type DeviceAuthority struct {
	DeviceID               DeviceID
	AccountID              AccountID
	SiteID                 SiteID
	IoTThingName           string
	CredentialID           string
	CredentialStatus       string
	CertificateFingerprint string
	CertificateSerial      string
}

type DeviceCredentialRecord struct {
	DeviceID               DeviceID
	CertificateFingerprint string
	CertificateSerial      string
	CertificateIssuer      string
	CertificateID          string
	CertificateARN         string
	PrincipalIdentifier    string
	Status                 string
	IssuedAt               time.Time
}

type DeviceRegistryRepository interface {
	ResolveCredential(ctx context.Context, identity CredentialIdentity) (DeviceAuthority, error)
	RecordCredential(ctx context.Context, record DeviceCredentialRecord) error
}

type LatestTelemetryState struct {
	DeviceID      DeviceID
	AccountID     AccountID
	SiteID        SiteID
	MessageID     string
	MessageType   string
	SchemaType    string
	SchemaVersion int
	MaterialHash  string
	Material      json.RawMessage
	Sidecar       json.RawMessage
	Payload       json.RawMessage
	ObservedAt    time.Time
	ReceivedAt    time.Time
}

type TelemetryEvent struct {
	LatestTelemetryState
}

type TelemetryFailure struct {
	DeviceID   DeviceID
	MessageID  string
	SchemaType string
	Stage      string
	Reason     string
	Context    json.RawMessage
	ReceivedAt time.Time
}

type TelemetryRepository interface {
	UpsertLatestState(ctx context.Context, state LatestTelemetryState) error
	InsertTelemetryEvent(ctx context.Context, event TelemetryEvent) error
	RecordFailure(ctx context.Context, failure TelemetryFailure) error
}

type AuditEvent struct {
	ActorType  string
	ActorID    string
	Action     string
	Resource   string
	AccountID  AccountID
	SiteID     SiteID
	DeviceID   DeviceID
	Metadata   json.RawMessage
	OccurredAt time.Time
}

type AuditRepository interface {
	RecordAuditEvent(ctx context.Context, event AuditEvent) error
}
