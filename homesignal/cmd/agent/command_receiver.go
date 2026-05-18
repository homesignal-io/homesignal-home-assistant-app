package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	commandTopicPrefix = "homesignal/devices"

	commandTypeRefreshPublishPolicy = "refresh_publish_policy"
	commandTypeTriggerBackup        = "trigger_backup"
	commandTypeUploadArtifact       = "upload_artifact"

	commandACKAccepted = "accepted"
	commandACKRejected = "rejected"
)

type AgentCommandClient interface {
	FetchCommandDetail(context.Context, FetchCommandDetailRequest) (AgentCommandDetail, error)
	RecordCommandACK(context.Context, CommandACKRequest) error
}

type FetchCommandDetailRequest struct {
	CommandID string
}

type AgentCommandDetail struct {
	CommandID   string          `json:"command_id"`
	CommandType string          `json:"command_type"`
	DeviceID    string          `json:"device_id"`
	Payload     json.RawMessage `json:"payload,omitempty"`
}

type CommandACKRequest struct {
	CommandID   string    `json:"command_id"`
	Status      string    `json:"status"`
	DeviceID    string    `json:"device_id"`
	CommandType string    `json:"command_type"`
	ReasonCode  string    `json:"reason_code,omitempty"`
	ReportedAt  time.Time `json:"reported_at"`
}

type MQTTCommandNotice struct {
	CommandID   string          `json:"command_id"`
	CommandType string          `json:"command_type"`
	DeviceID    string          `json:"device_id"`
	IssuedAt    time.Time       `json:"issued_at"`
	ExpiresAt   time.Time       `json:"expires_at"`
	Payload     json.RawMessage `json:"payload,omitempty"`
}

type LocalCommandPolicy struct {
	AllowedTypes map[string]bool
}

type CommandReceiver struct {
	Client AgentCommandClient
	Policy LocalCommandPolicy
	Now    func() time.Time
}

type CommandDecision struct {
	CommandID  string
	Status     string
	ReasonCode string
}

func (r CommandReceiver) HandleMQTTCommandNotice(ctx context.Context, topic string, payload []byte, localDeviceID string) (CommandDecision, error) {
	if r.Client == nil {
		return CommandDecision{}, fmt.Errorf("agent command client is required")
	}
	now := r.now()
	notice, err := parseMQTTCommandNotice(topic, payload, localDeviceID, now)
	if err != nil {
		return CommandDecision{}, err
	}
	if !knownLocalCommandType(notice.CommandType) {
		return r.reject(ctx, notice, "unknown_command_type", now)
	}
	if !r.Policy.Allows(notice.CommandType) {
		return r.reject(ctx, notice, "local_policy_denied", now)
	}
	detail, err := r.Client.FetchCommandDetail(ctx, FetchCommandDetailRequest{CommandID: notice.CommandID})
	if err != nil {
		return CommandDecision{}, fmt.Errorf("fetch command detail: %w", err)
	}
	if err := validateCommandDetailMatchesNotice(detail, notice); err != nil {
		return r.reject(ctx, notice, "command_detail_mismatch", now)
	}
	ack := CommandACKRequest{
		CommandID:   notice.CommandID,
		Status:      commandACKAccepted,
		DeviceID:    localDeviceID,
		CommandType: notice.CommandType,
		ReportedAt:  now,
	}
	if err := r.Client.RecordCommandACK(ctx, ack); err != nil {
		return CommandDecision{}, fmt.Errorf("record command ack: %w", err)
	}
	return CommandDecision{CommandID: notice.CommandID, Status: commandACKAccepted}, nil
}

func (r CommandReceiver) reject(ctx context.Context, notice MQTTCommandNotice, reasonCode string, reportedAt time.Time) (CommandDecision, error) {
	ack := CommandACKRequest{
		CommandID:   notice.CommandID,
		Status:      commandACKRejected,
		DeviceID:    notice.DeviceID,
		CommandType: notice.CommandType,
		ReasonCode:  reasonCode,
		ReportedAt:  reportedAt,
	}
	if err := r.Client.RecordCommandACK(ctx, ack); err != nil {
		return CommandDecision{}, fmt.Errorf("record command rejection: %w", err)
	}
	return CommandDecision{CommandID: notice.CommandID, Status: commandACKRejected, ReasonCode: reasonCode}, nil
}

func (r CommandReceiver) now() time.Time {
	if r.Now != nil {
		return r.Now().UTC()
	}
	return time.Now().UTC()
}

func (p LocalCommandPolicy) Allows(commandType string) bool {
	if p.AllowedTypes == nil {
		return false
	}
	return p.AllowedTypes[commandType]
}

func parseMQTTCommandNotice(topic string, payload []byte, localDeviceID string, now time.Time) (MQTTCommandNotice, error) {
	localDeviceID = strings.TrimSpace(localDeviceID)
	if localDeviceID == "" {
		return MQTTCommandNotice{}, fmt.Errorf("local device_id is required")
	}
	topicDeviceID, err := deviceIDFromCommandTopic(topic)
	if err != nil {
		return MQTTCommandNotice{}, err
	}
	if topicDeviceID != localDeviceID {
		return MQTTCommandNotice{}, fmt.Errorf("command topic device_id mismatch")
	}
	var notice MQTTCommandNotice
	if err := json.Unmarshal(payload, &notice); err != nil {
		return MQTTCommandNotice{}, fmt.Errorf("decode command notice: %w", err)
	}
	notice.CommandID = strings.TrimSpace(notice.CommandID)
	notice.CommandType = strings.TrimSpace(notice.CommandType)
	notice.DeviceID = strings.TrimSpace(notice.DeviceID)
	if notice.CommandID == "" {
		return MQTTCommandNotice{}, fmt.Errorf("command_id is required")
	}
	if notice.CommandType == "" {
		return MQTTCommandNotice{}, fmt.Errorf("command_type is required")
	}
	if notice.DeviceID != localDeviceID {
		return MQTTCommandNotice{}, fmt.Errorf("command notice device_id mismatch")
	}
	if notice.IssuedAt.IsZero() {
		return MQTTCommandNotice{}, fmt.Errorf("issued_at is required")
	}
	if notice.ExpiresAt.IsZero() || !notice.ExpiresAt.After(now) {
		return MQTTCommandNotice{}, fmt.Errorf("command notice expired")
	}
	return notice, nil
}

func deviceIDFromCommandTopic(topic string) (string, error) {
	parts := strings.Split(strings.TrimSpace(topic), "/")
	if len(parts) != 4 || parts[0] != "homesignal" || parts[1] != "devices" || parts[3] != "commands" {
		return "", fmt.Errorf("unexpected command topic")
	}
	if strings.TrimSpace(parts[2]) == "" {
		return "", fmt.Errorf("command topic device_id is required")
	}
	return parts[2], nil
}

func validateCommandDetailMatchesNotice(detail AgentCommandDetail, notice MQTTCommandNotice) error {
	if strings.TrimSpace(detail.CommandID) != notice.CommandID {
		return fmt.Errorf("command_id mismatch")
	}
	if strings.TrimSpace(detail.CommandType) != notice.CommandType {
		return fmt.Errorf("command_type mismatch")
	}
	if strings.TrimSpace(detail.DeviceID) != notice.DeviceID {
		return fmt.Errorf("device_id mismatch")
	}
	return nil
}

func knownLocalCommandType(commandType string) bool {
	switch commandType {
	case commandTypeRefreshPublishPolicy, commandTypeTriggerBackup, commandTypeUploadArtifact:
		return true
	default:
		return false
	}
}
