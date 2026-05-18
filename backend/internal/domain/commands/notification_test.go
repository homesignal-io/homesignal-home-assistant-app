package commands

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestDeviceNotificationPublisherMatchesContractFixture(t *testing.T) {
	issuedAt := time.Date(2026, 5, 18, 12, 0, 1, 0, time.UTC)
	expiresAt := issuedAt.Add(5 * time.Minute)
	publisher := DeviceNotificationPublisher{Publisher: &fakeMQTTPublisher{}}

	message, err := publisher.BuildMessage(NotificationRequest{
		NotificationID:   "ntf_123",
		NotificationType: NotificationTypePublishPolicyChanged,
		DeviceID:         "dev_123",
		IssuedAt:         issuedAt,
		ExpiresAt:        &expiresAt,
		Payload:          json.RawMessage(`{"publish_policy_version":"ppv_v0_default_free"}`),
	})
	if err != nil {
		t.Fatalf("build notification: %v", err)
	}
	if message.Topic != "homesignal/devices/dev_123/notifications" {
		t.Fatalf("unexpected topic %q", message.Topic)
	}
	if message.QoS != 1 {
		t.Fatalf("expected QoS 1, got %d", message.QoS)
	}
	expected, err := os.ReadFile("testdata/device_notification_publish_policy_changed.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if got := string(message.Payload); got != strings.TrimSpace(string(expected)) {
		t.Fatalf("notification payload mismatch\nwant: %s\n got: %s", strings.TrimSpace(string(expected)), got)
	}
}

func TestDeviceNotificationPublisherPublishesWithoutCommandRepository(t *testing.T) {
	issuedAt := time.Date(2026, 5, 18, 12, 0, 1, 0, time.UTC)
	fake := &fakeMQTTPublisher{}
	publisher := DeviceNotificationPublisher{Publisher: fake}

	err := publisher.Publish(context.Background(), NotificationRequest{
		NotificationID:   "ntf_123",
		NotificationType: NotificationTypePublishPolicyChanged,
		DeviceID:         "dev_123",
		IssuedAt:         issuedAt,
		Payload:          json.RawMessage(`{"publish_policy_version":"ppv_v0_default_free"}`),
	})
	if err != nil {
		t.Fatalf("publish notification: %v", err)
	}
	if len(fake.messages) != 1 {
		t.Fatalf("expected one published message, got %d", len(fake.messages))
	}
}

func TestDeviceNotificationRejectsUnsupportedType(t *testing.T) {
	issuedAt := time.Date(2026, 5, 18, 12, 0, 1, 0, time.UTC)
	publisher := DeviceNotificationPublisher{Publisher: &fakeMQTTPublisher{}}

	_, err := publisher.BuildMessage(NotificationRequest{
		NotificationID:   "ntf_123",
		NotificationType: "generic_nudge",
		DeviceID:         "dev_123",
		IssuedAt:         issuedAt,
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported notification type") {
		t.Fatalf("expected unsupported notification error, got %v", err)
	}
}

func TestDeviceNotificationRejectsCommandStateFields(t *testing.T) {
	issuedAt := time.Date(2026, 5, 18, 12, 0, 1, 0, time.UTC)
	publisher := DeviceNotificationPublisher{Publisher: &fakeMQTTPublisher{}}

	_, err := publisher.BuildMessage(NotificationRequest{
		NotificationID:   "ntf_123",
		NotificationType: NotificationTypePublishPolicyChanged,
		DeviceID:         "dev_123",
		IssuedAt:         issuedAt,
		Payload:          json.RawMessage(`{"command_id":"cmd_123"}`),
	})
	if err == nil || !strings.Contains(err.Error(), "forbidden key") {
		t.Fatalf("expected command state payload to fail, got %v", err)
	}
}

func TestDeviceNotificationRejectsSecretPayload(t *testing.T) {
	issuedAt := time.Date(2026, 5, 18, 12, 0, 1, 0, time.UTC)
	publisher := DeviceNotificationPublisher{Publisher: &fakeMQTTPublisher{}}

	_, err := publisher.BuildMessage(NotificationRequest{
		NotificationID:   "ntf_123",
		NotificationType: NotificationTypePublishPolicyChanged,
		DeviceID:         "dev_123",
		IssuedAt:         issuedAt,
		Payload:          json.RawMessage(`{"secret":"nope"}`),
	})
	if err == nil || !strings.Contains(err.Error(), "forbidden key") {
		t.Fatalf("expected secret payload to fail, got %v", err)
	}
}
