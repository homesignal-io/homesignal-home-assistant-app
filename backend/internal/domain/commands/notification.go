package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	NotificationTypePublishPolicyChanged NotificationType = "publish_policy_changed"
)

type NotificationType string

type DeviceNotification struct {
	NotificationID   string           `json:"notification_id"`
	NotificationType NotificationType `json:"notification_type"`
	DeviceID         string           `json:"device_id"`
	IssuedAt         time.Time        `json:"issued_at"`
	ExpiresAt        *time.Time       `json:"expires_at,omitempty"`
	Payload          json.RawMessage  `json:"payload,omitempty"`
}

type NotificationRequest struct {
	NotificationID   string
	NotificationType NotificationType
	DeviceID         string
	IssuedAt         time.Time
	ExpiresAt        *time.Time
	Payload          json.RawMessage
}

type DeviceNotificationPublisher struct {
	Publisher         MQTTMessagePublisher
	NotificationTypes map[NotificationType]bool
	MaxNoticeBytes    int
	MaxPayloadBytes   int
}

func DefaultNotificationTypes() map[NotificationType]bool {
	return map[NotificationType]bool{
		NotificationTypePublishPolicyChanged: true,
	}
}

func (p DeviceNotificationPublisher) Publish(ctx context.Context, req NotificationRequest) error {
	if p.Publisher == nil {
		return fmt.Errorf("mqtt publisher is required")
	}
	message, err := p.BuildMessage(req)
	if err != nil {
		return err
	}
	if err := p.Publisher.Publish(ctx, message); err != nil {
		return fmt.Errorf("publish device notification: %w", err)
	}
	return nil
}

func (p DeviceNotificationPublisher) BuildMessage(req NotificationRequest) (MQTTMessage, error) {
	notification, err := p.BuildNotification(req)
	if err != nil {
		return MQTTMessage{}, err
	}
	payload, err := json.Marshal(notification)
	if err != nil {
		return MQTTMessage{}, fmt.Errorf("marshal device notification: %w", err)
	}
	maxNoticeBytes := p.MaxNoticeBytes
	if maxNoticeBytes <= 0 {
		maxNoticeBytes = DefaultNoticeMaxBytes
	}
	if len(payload) > maxNoticeBytes {
		return MQTTMessage{}, fmt.Errorf("device notification exceeds %d bytes", maxNoticeBytes)
	}
	topic, err := NotificationTopic(req.DeviceID)
	if err != nil {
		return MQTTMessage{}, err
	}
	return MQTTMessage{
		Topic:       topic,
		Payload:     payload,
		QoS:         DefaultCommandNoticeQoS,
		ContentType: DefaultNoticeContentType,
	}, nil
}

func (p DeviceNotificationPublisher) BuildNotification(req NotificationRequest) (DeviceNotification, error) {
	notificationID := strings.TrimSpace(req.NotificationID)
	if notificationID == "" {
		return DeviceNotification{}, fmt.Errorf("notification_id is required")
	}
	deviceID := strings.TrimSpace(req.DeviceID)
	if deviceID == "" {
		return DeviceNotification{}, fmt.Errorf("device_id is required")
	}
	if !p.notificationTypeAllowed(req.NotificationType) {
		return DeviceNotification{}, fmt.Errorf("unsupported notification type %q", req.NotificationType)
	}
	issuedAt := req.IssuedAt
	if issuedAt.IsZero() {
		return DeviceNotification{}, fmt.Errorf("issued_at is required")
	}
	if req.ExpiresAt != nil && !req.ExpiresAt.After(issuedAt) {
		return DeviceNotification{}, fmt.Errorf("expires_at must be after issued_at")
	}
	payload := normalizePayload(req.Payload)
	maxPayloadBytes := p.MaxPayloadBytes
	if maxPayloadBytes <= 0 {
		maxPayloadBytes = DefaultNoticePayloadMax
	}
	if err := validateNoticePayload(payload, maxPayloadBytes); err != nil {
		return DeviceNotification{}, err
	}
	var expiresAt *time.Time
	if req.ExpiresAt != nil {
		expires := req.ExpiresAt.UTC()
		expiresAt = &expires
	}
	return DeviceNotification{
		NotificationID:   notificationID,
		NotificationType: req.NotificationType,
		DeviceID:         deviceID,
		IssuedAt:         issuedAt.UTC(),
		ExpiresAt:        expiresAt,
		Payload:          payload,
	}, nil
}

func NotificationTopic(deviceID string) (string, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return "", fmt.Errorf("device_id is required")
	}
	if strings.ContainsAny(deviceID, "/#+") {
		return "", fmt.Errorf("device_id contains MQTT topic separators or wildcards")
	}
	return fmt.Sprintf("%s/%s/notifications", CommandTopicPrefix, deviceID), nil
}

func (p DeviceNotificationPublisher) notificationTypeAllowed(notificationType NotificationType) bool {
	notificationTypes := p.NotificationTypes
	if notificationTypes == nil {
		notificationTypes = DefaultNotificationTypes()
	}
	return notificationTypes[notificationType]
}
