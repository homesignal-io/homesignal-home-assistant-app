package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type RuntimePipeline struct {
	Parser            EnvelopeParser
	Catalog           *Catalog
	AuthorityResolver DeviceAuthorityResolver
	DedupeStore       DedupeStore
	Writer            PersistenceWriter
	FailureSink       FailureSink
	Clock             func() time.Time
}

func NewRuntimePipeline(writer PersistenceWriter, failureSink FailureSink) *RuntimePipeline {
	return &RuntimePipeline{
		Parser:      RuntimeEnvelopeParser{},
		Catalog:     NewDefaultCatalog(),
		DedupeStore: NewMemoryDedupeStore(),
		Writer:      writer,
		FailureSink: failureSink,
		Clock:       func() time.Time { return time.Now().UTC() },
	}
}

func (p *RuntimePipeline) Ingest(ctx context.Context, request IngestRequest) (IngestResult, error) {
	p = p.withDefaults()
	receivedAt := request.ReceivedAt
	if receivedAt.IsZero() {
		receivedAt = p.Clock().UTC()
	}

	envelope, err := p.Parser.Parse(request.Route, request.Body)
	if err != nil {
		p.recordFailure(ctx, request.Device, RuntimeEnvelope{}, "parse", err.Error(), receivedAt)
		return IngestResult{Accepted: false, ReceivedAt: receivedAt}, err
	}

	projection, err := p.Catalog.Validate(envelope)
	if err != nil {
		p.recordFailure(ctx, request.Device, envelope, "schema", err.Error(), receivedAt)
		return IngestResult{Accepted: false, MessageID: envelope.MessageID, SchemaType: envelope.SchemaType, ReceivedAt: receivedAt}, err
	}

	device, err := p.resolveAuthority(ctx, request, envelope)
	if err != nil {
		p.recordFailure(ctx, failureDeviceContext(request), envelope, "authority", authorityFailureReason(err), receivedAt)
		return IngestResult{Accepted: false, MessageID: envelope.MessageID, SchemaType: envelope.SchemaType, ReceivedAt: receivedAt}, err
	}
	if err := ensurePayloadIdentityMatches(envelope.Payload, device.DeviceID); err != nil {
		p.recordFailure(ctx, device, envelope, "authority", authorityFailureReason(err), receivedAt)
		return IngestResult{Accepted: false, MessageID: envelope.MessageID, SchemaType: envelope.SchemaType, ReceivedAt: receivedAt}, err
	}

	materialHash := MaterialHash(projection.Material)
	messageKey := MessageDedupeKey{DeviceID: device.DeviceID, MessageID: envelope.MessageID}
	if p.DedupeStore != nil {
		seen, err := p.DedupeStore.SeenMessage(ctx, messageKey)
		if err != nil {
			p.recordFailure(ctx, device, envelope, "dedupe", err.Error(), receivedAt)
			return IngestResult{Accepted: false, MessageID: envelope.MessageID, SchemaType: envelope.SchemaType, MaterialHash: materialHash, ReceivedAt: receivedAt}, fmt.Errorf("check message dedupe: %w", err)
		}
		if seen {
			return IngestResult{
				Accepted:          true,
				Written:           false,
				Suppressed:        true,
				SuppressionReason: "duplicate_message",
				MessageID:         envelope.MessageID,
				SchemaType:        envelope.SchemaType,
				MaterialHash:      materialHash,
				ReceivedAt:        receivedAt,
			}, nil
		}
	}

	message := ValidatedMessage{
		Device:     device,
		Envelope:   envelope,
		Projection: projection,
		ReceivedAt: receivedAt,
	}
	written := false
	suppressed := false
	suppressionReason := ""
	if p.shouldSuppressUnchangedTelemetry(ctx, message, materialHash) {
		suppressed = true
		suppressionReason = "unchanged_material"
	} else if p.Writer != nil {
		if err := p.Writer.WriteLatest(ctx, message); err != nil {
			p.recordFailure(ctx, device, envelope, "persistence", err.Error(), receivedAt)
			return IngestResult{Accepted: false, MessageID: envelope.MessageID, SchemaType: envelope.SchemaType, ReceivedAt: receivedAt}, fmt.Errorf("write latest state: %w", err)
		}
		written = true
		if p.DedupeStore != nil && envelope.MessageType == MessageTypeTelemetry {
			if err := p.DedupeStore.RecordMaterialHash(ctx, stateDedupeKey(message), materialHash); err != nil {
				p.recordFailure(ctx, device, envelope, "dedupe", err.Error(), receivedAt)
				return IngestResult{Accepted: false, MessageID: envelope.MessageID, SchemaType: envelope.SchemaType, MaterialHash: materialHash, ReceivedAt: receivedAt}, fmt.Errorf("record material hash: %w", err)
			}
		}
	}
	if p.DedupeStore != nil {
		if err := p.DedupeStore.RecordMessage(ctx, messageKey); err != nil {
			p.recordFailure(ctx, device, envelope, "dedupe", err.Error(), receivedAt)
			return IngestResult{Accepted: false, MessageID: envelope.MessageID, SchemaType: envelope.SchemaType, MaterialHash: materialHash, ReceivedAt: receivedAt}, fmt.Errorf("record message dedupe: %w", err)
		}
	}

	return IngestResult{
		Accepted:          true,
		Written:           written,
		Suppressed:        suppressed,
		SuppressionReason: suppressionReason,
		MessageID:         envelope.MessageID,
		SchemaType:        envelope.SchemaType,
		MaterialHash:      materialHash,
		ReceivedAt:        receivedAt,
	}, nil
}

func (p *RuntimePipeline) withDefaults() *RuntimePipeline {
	if p == nil {
		return NewRuntimePipeline(nil, nil)
	}
	if p.Parser == nil {
		p.Parser = RuntimeEnvelopeParser{}
	}
	if p.Catalog == nil {
		p.Catalog = NewDefaultCatalog()
	}
	if p.DedupeStore == nil {
		p.DedupeStore = NewMemoryDedupeStore()
	}
	if p.Clock == nil {
		p.Clock = func() time.Time { return time.Now().UTC() }
	}
	return p
}

func (p *RuntimePipeline) shouldSuppressUnchangedTelemetry(ctx context.Context, message ValidatedMessage, materialHash string) bool {
	if p.DedupeStore == nil || message.Envelope.MessageType != MessageTypeTelemetry {
		return false
	}
	previous, ok, err := p.DedupeStore.LastMaterialHash(ctx, stateDedupeKey(message))
	if err != nil {
		return false
	}
	return ok && previous == materialHash
}

func stateDedupeKey(message ValidatedMessage) StateDedupeKey {
	return StateDedupeKey{
		DeviceID:      message.Device.DeviceID,
		SchemaType:    message.Envelope.SchemaType,
		SchemaVersion: message.Envelope.SchemaVersion,
	}
}

func (p *RuntimePipeline) resolveAuthority(ctx context.Context, request IngestRequest, envelope RuntimeEnvelope) (AuthenticatedDeviceContext, error) {
	if p.AuthorityResolver == nil {
		return request.Device, nil
	}
	credential := request.Credential
	if credential.CertificateFingerprint == "" {
		credential.CertificateFingerprint = request.Device.CertificateFingerprint
	}
	if credential.CertificateSerial == "" {
		credential.CertificateSerial = request.Device.CertificateSerial
	}
	if strings.TrimSpace(credential.CertificateFingerprint) == "" {
		return AuthenticatedDeviceContext{}, fmt.Errorf("%w: certificate fingerprint is required", ErrMissingTransportCredential)
	}
	device, err := p.AuthorityResolver.ResolveDevice(ctx, credential)
	if err != nil {
		return AuthenticatedDeviceContext{}, fmt.Errorf("resolve device authority: %w", err)
	}
	if strings.TrimSpace(device.DeviceID) == "" {
		return AuthenticatedDeviceContext{}, fmt.Errorf("%w: resolver returned no device_id for message %s", ErrIdentityDrift, envelope.MessageID)
	}
	if device.CertificateFingerprint == "" {
		device.CertificateFingerprint = credential.CertificateFingerprint
	}
	if device.CertificateSerial == "" {
		device.CertificateSerial = credential.CertificateSerial
	}
	return device, nil
}

func ensurePayloadIdentityMatches(payload json.RawMessage, resolvedDeviceID string) error {
	if len(payload) == 0 || strings.TrimSpace(resolvedDeviceID) == "" {
		return nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(payload, &raw); err != nil {
		return fmt.Errorf("decode payload identity annotations: %w", err)
	}
	for _, deviceID := range payloadDeviceIDs(raw) {
		if deviceID != "" && deviceID != resolvedDeviceID {
			return fmt.Errorf("%w: payload device id %q does not match resolved device id %q", ErrIdentityDrift, deviceID, resolvedDeviceID)
		}
	}
	return nil
}

func payloadDeviceIDs(raw map[string]json.RawMessage) []string {
	var ids []string
	appendStringField := func(field string) {
		var value string
		if payload, ok := raw[field]; ok && json.Unmarshal(payload, &value) == nil {
			ids = append(ids, strings.TrimSpace(value))
		}
	}
	appendStringField("device_id")
	appendStringField("homesignal_device_id")

	var device map[string]json.RawMessage
	if payload, ok := raw["device"]; ok && json.Unmarshal(payload, &device) == nil {
		for _, field := range []string{"homesignal_device_id", "device_id"} {
			var value string
			if nested, ok := device[field]; ok && json.Unmarshal(nested, &value) == nil {
				ids = append(ids, strings.TrimSpace(value))
			}
		}
	}
	return ids
}

func failureDeviceContext(request IngestRequest) AuthenticatedDeviceContext {
	device := request.Device
	if device.CertificateFingerprint == "" {
		device.CertificateFingerprint = request.Credential.CertificateFingerprint
	}
	if device.CertificateSerial == "" {
		device.CertificateSerial = request.Credential.CertificateSerial
	}
	return device
}

func authorityFailureReason(err error) string {
	switch {
	case errors.Is(err, ErrIdentityDrift):
		return "identity_drift"
	case errors.Is(err, ErrMissingTransportCredential):
		return "missing_transport_credential"
	default:
		return "authority_resolution_failed"
	}
}

func (p *RuntimePipeline) recordFailure(ctx context.Context, device AuthenticatedDeviceContext, envelope RuntimeEnvelope, stage, reason string, receivedAt time.Time) {
	if p.FailureSink == nil {
		return
	}
	_ = p.FailureSink.RecordFailure(ctx, IngestFailure{
		Device:     device,
		MessageID:  envelope.MessageID,
		SchemaType: envelope.SchemaType,
		Stage:      stage,
		Reason:     reason,
		ReceivedAt: receivedAt,
	})
}
