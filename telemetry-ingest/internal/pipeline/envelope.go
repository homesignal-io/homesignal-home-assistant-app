package pipeline

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

type RuntimeEnvelopeParser struct{}

func (RuntimeEnvelopeParser) Parse(route RuntimeRoute, body []byte) (RuntimeEnvelope, error) {
	if err := rejectDuplicateJSONKeys(body); err != nil {
		return RuntimeEnvelope{}, fmt.Errorf("runtime envelope json: %w", err)
	}

	var envelope RuntimeEnvelope
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&envelope); err != nil {
		return RuntimeEnvelope{}, fmt.Errorf("decode runtime envelope: %w", err)
	}
	if decoder.More() {
		return RuntimeEnvelope{}, fmt.Errorf("runtime envelope has trailing data")
	}
	if err := validateEnvelopeRoute(route, envelope.MessageType); err != nil {
		return RuntimeEnvelope{}, err
	}
	if err := validateEnvelope(envelope); err != nil {
		return RuntimeEnvelope{}, err
	}
	return envelope, nil
}

func validateEnvelopeRoute(route RuntimeRoute, messageType MessageType) error {
	switch route {
	case RouteTelemetry:
		if messageType != MessageTypeTelemetry {
			return fmt.Errorf("route telemetry requires message_type telemetry")
		}
	case RouteEvents:
		if messageType != MessageTypeEvent {
			return fmt.Errorf("route events requires message_type event")
		}
	case "":
		return nil
	default:
		return fmt.Errorf("unsupported runtime route %q", route)
	}
	return nil
}

func validateEnvelope(envelope RuntimeEnvelope) error {
	if !validMessageType(envelope.MessageType) {
		return fmt.Errorf("invalid message_type %q", envelope.MessageType)
	}
	if strings.TrimSpace(envelope.SchemaType) == "" {
		return fmt.Errorf("schema_type is required")
	}
	if envelope.SchemaVersion <= 0 {
		return fmt.Errorf("schema_version must be positive")
	}
	if strings.TrimSpace(envelope.MessageID) == "" {
		return fmt.Errorf("message_id is required")
	}
	if envelope.ObservedAt.IsZero() {
		return fmt.Errorf("observed_at is required")
	}
	if err := validateRuntimePayload(envelope.Payload); err != nil {
		return err
	}
	if strings.TrimSpace(envelope.AppliedPublishPolicyVersion) == "" {
		if envelope.MessageType == MessageTypeEvent && envelope.SchemaType == SchemaTypeAgentAlarm {
			return nil
		}
		return fmt.Errorf("applied_publish_policy_version is required")
	}
	if token := forbiddenRuntimePayloadToken(envelope.Payload); token != "" {
		return fmt.Errorf("payload contains forbidden token %q", token)
	}
	return nil
}

func validateRuntimePayload(payload json.RawMessage) error {
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return fmt.Errorf("payload is required")
	}
	if trimmed[0] != '{' {
		return fmt.Errorf("payload must be a json object")
	}
	return nil
}

func validMessageType(value MessageType) bool {
	switch value {
	case MessageTypeTelemetry, MessageTypeEvent, MessageTypeCommandACK, MessageTypeCommandResult:
		return true
	default:
		return false
	}
}

func forbiddenRuntimePayloadToken(payload []byte) string {
	lower := strings.ToLower(string(payload))
	for _, token := range []string{
		"-----begin rsa private key-----",
		"-----begin private key-----",
		"authorization:",
		"bearer ",
		"pairing_code",
		"poll_token",
		"signed_url",
		"client_secret",
		"set-cookie:",
	} {
		if strings.Contains(lower, token) {
			return token
		}
	}
	return ""
}
