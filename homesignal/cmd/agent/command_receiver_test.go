package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestCommandReceiverAcceptsKnownAllowedCommand(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 2, 0, time.UTC)
	client := &fakeAgentCommandClient{
		details: map[string]AgentCommandDetail{
			"cmd_123": {
				CommandID:   "cmd_123",
				CommandType: commandTypeRefreshPublishPolicy,
				DeviceID:    "dev_123",
				Payload:     json.RawMessage(`{"reason":"over_budget"}`),
			},
		},
	}
	receiver := CommandReceiver{
		Client: client,
		Policy: LocalCommandPolicy{AllowedTypes: map[string]bool{
			commandTypeRefreshPublishPolicy: true,
		}},
		Now: func() time.Time { return now },
	}

	decision, err := receiver.HandleMQTTCommandNotice(
		context.Background(),
		"homesignal/devices/dev_123/commands",
		commandNoticePayload(t, commandTypeRefreshPublishPolicy, now.Add(-time.Second), now.Add(14*time.Second)),
		"dev_123",
	)
	if err != nil {
		t.Fatalf("handle command notice: %v", err)
	}
	if decision.Status != commandACKAccepted {
		t.Fatalf("expected accepted decision, got %#v", decision)
	}
	if len(client.fetches) != 1 || client.fetches[0].CommandID != "cmd_123" {
		t.Fatalf("expected command detail fetch, got %#v", client.fetches)
	}
	if len(client.acks) != 1 || client.acks[0].Status != commandACKAccepted {
		t.Fatalf("expected accepted ack, got %#v", client.acks)
	}
}

func TestCommandReceiverRejectsUnknownCommandType(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 2, 0, time.UTC)
	client := &fakeAgentCommandClient{details: map[string]AgentCommandDetail{}}
	receiver := CommandReceiver{
		Client: client,
		Policy: LocalCommandPolicy{AllowedTypes: map[string]bool{
			commandTypeRefreshPublishPolicy: true,
		}},
		Now: func() time.Time { return now },
	}

	decision, err := receiver.HandleMQTTCommandNotice(
		context.Background(),
		"homesignal/devices/dev_123/commands",
		commandNoticePayload(t, "apply_update", now.Add(-time.Second), now.Add(14*time.Second)),
		"dev_123",
	)
	if err != nil {
		t.Fatalf("handle command notice: %v", err)
	}
	if decision.Status != commandACKRejected || decision.ReasonCode != "unknown_command_type" {
		t.Fatalf("expected unknown command rejection, got %#v", decision)
	}
	if len(client.fetches) != 0 {
		t.Fatalf("unknown command should not fetch detail, got %#v", client.fetches)
	}
	if len(client.acks) != 1 || client.acks[0].ReasonCode != "unknown_command_type" {
		t.Fatalf("expected rejection ack, got %#v", client.acks)
	}
}

func TestCommandReceiverRejectsLocallyDisallowedCommand(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 2, 0, time.UTC)
	client := &fakeAgentCommandClient{details: map[string]AgentCommandDetail{}}
	receiver := CommandReceiver{
		Client: client,
		Policy: LocalCommandPolicy{AllowedTypes: map[string]bool{
			commandTypeRefreshPublishPolicy: true,
		}},
		Now: func() time.Time { return now },
	}

	decision, err := receiver.HandleMQTTCommandNotice(
		context.Background(),
		"homesignal/devices/dev_123/commands",
		commandNoticePayload(t, commandTypeTriggerBackup, now.Add(-time.Second), now.Add(14*time.Second)),
		"dev_123",
	)
	if err != nil {
		t.Fatalf("handle command notice: %v", err)
	}
	if decision.Status != commandACKRejected || decision.ReasonCode != "local_policy_denied" {
		t.Fatalf("expected local policy rejection, got %#v", decision)
	}
	if len(client.fetches) != 0 {
		t.Fatalf("locally denied command should not fetch detail, got %#v", client.fetches)
	}
}

func TestCommandReceiverRejectsCommandDetailMismatch(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 2, 0, time.UTC)
	client := &fakeAgentCommandClient{
		details: map[string]AgentCommandDetail{
			"cmd_123": {
				CommandID:   "cmd_123",
				CommandType: commandTypeTriggerBackup,
				DeviceID:    "dev_123",
			},
		},
	}
	receiver := CommandReceiver{
		Client: client,
		Policy: LocalCommandPolicy{AllowedTypes: map[string]bool{
			commandTypeRefreshPublishPolicy: true,
			commandTypeTriggerBackup:        true,
		}},
		Now: func() time.Time { return now },
	}

	decision, err := receiver.HandleMQTTCommandNotice(
		context.Background(),
		"homesignal/devices/dev_123/commands",
		commandNoticePayload(t, commandTypeRefreshPublishPolicy, now.Add(-time.Second), now.Add(14*time.Second)),
		"dev_123",
	)
	if err != nil {
		t.Fatalf("handle command notice: %v", err)
	}
	if decision.Status != commandACKRejected || decision.ReasonCode != "command_detail_mismatch" {
		t.Fatalf("expected mismatch rejection, got %#v", decision)
	}
}

func TestCommandReceiverFailsClosedForWrongTopicOrExpiredNotice(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 2, 0, time.UTC)
	receiver := CommandReceiver{
		Client: &fakeAgentCommandClient{},
		Policy: LocalCommandPolicy{AllowedTypes: map[string]bool{
			commandTypeRefreshPublishPolicy: true,
		}},
		Now: func() time.Time { return now },
	}

	_, err := receiver.HandleMQTTCommandNotice(
		context.Background(),
		"homesignal/devices/other/commands",
		commandNoticePayload(t, commandTypeRefreshPublishPolicy, now.Add(-time.Second), now.Add(14*time.Second)),
		"dev_123",
	)
	if err == nil || !strings.Contains(err.Error(), "topic device_id mismatch") {
		t.Fatalf("expected topic mismatch error, got %v", err)
	}

	_, err = receiver.HandleMQTTCommandNotice(
		context.Background(),
		"homesignal/devices/dev_123/commands",
		commandNoticePayload(t, commandTypeRefreshPublishPolicy, now.Add(-time.Minute), now.Add(-time.Second)),
		"dev_123",
	)
	if err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("expected expired notice error, got %v", err)
	}
}

func commandNoticePayload(t *testing.T, commandType string, issuedAt time.Time, expiresAt time.Time) []byte {
	t.Helper()
	payload, err := json.Marshal(MQTTCommandNotice{
		CommandID:   "cmd_123",
		CommandType: commandType,
		DeviceID:    "dev_123",
		IssuedAt:    issuedAt,
		ExpiresAt:   expiresAt,
		Payload:     json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("marshal command notice: %v", err)
	}
	return payload
}

type fakeAgentCommandClient struct {
	details map[string]AgentCommandDetail
	fetches []FetchCommandDetailRequest
	acks    []CommandACKRequest
}

func (c *fakeAgentCommandClient) FetchCommandDetail(_ context.Context, req FetchCommandDetailRequest) (AgentCommandDetail, error) {
	c.fetches = append(c.fetches, req)
	detail, ok := c.details[req.CommandID]
	if !ok {
		return AgentCommandDetail{}, errFakeCommandNotFound{}
	}
	return detail, nil
}

func (c *fakeAgentCommandClient) RecordCommandACK(_ context.Context, req CommandACKRequest) error {
	c.acks = append(c.acks, req)
	return nil
}

type errFakeCommandNotFound struct{}

func (errFakeCommandNotFound) Error() string {
	return "command not found"
}
