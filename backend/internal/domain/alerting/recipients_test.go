package alerting

import (
	"context"
	"strings"
	"testing"
)

func TestRecipientServiceCreatesVerifiedRecipientForAuthenticatedVerifiedEmail(t *testing.T) {
	repo := newFakeRecipientRepository()
	auth := &fakeRecipientAuthorization{allowed: true}
	service := recipientTestService(repo, auth)

	recipient, err := service.CreateRecipient(context.Background(), CreateRecipientRequest{
		AccountID:          "acct_123",
		SiteID:             "site_123",
		Email:              "Owner@Example.COM",
		DisplayLabel:       "Owner",
		ActorUserID:        "user_123",
		ActorEmail:         "owner@example.com",
		ActorEmailVerified: true,
		SubscribedFamilies: []SubscriptionFamily{SubscriptionDeviceDisconnected},
	})
	if err != nil {
		t.Fatalf("create recipient: %v", err)
	}
	if recipient.Status != RecipientStatusVerified || recipient.VerifiedAt == nil {
		t.Fatalf("expected verified recipient, got %#v", recipient)
	}
	if recipient.EmailNormalized != "owner@example.com" {
		t.Fatalf("expected normalized email, got %q", recipient.EmailNormalized)
	}
	if len(auth.calls) != 1 {
		t.Fatalf("expected authorization check")
	}
}

func TestRecipientServiceCreatesPendingRecipientForDifferentEmail(t *testing.T) {
	repo := newFakeRecipientRepository()
	service := recipientTestService(repo, &fakeRecipientAuthorization{allowed: true})

	recipient, err := service.CreateRecipient(context.Background(), CreateRecipientRequest{
		AccountID:          "acct_123",
		Email:              "alerts@example.com",
		ActorUserID:        "user_123",
		ActorEmail:         "owner@example.com",
		ActorEmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create recipient: %v", err)
	}
	if recipient.Status != RecipientStatusPendingVerification || recipient.VerifiedAt != nil {
		t.Fatalf("expected pending recipient, got %#v", recipient)
	}
}

func TestRecipientServiceRejectsUnauthorizedManageRequest(t *testing.T) {
	service := recipientTestService(newFakeRecipientRepository(), &fakeRecipientAuthorization{allowed: false})

	_, err := service.CreateRecipient(context.Background(), CreateRecipientRequest{
		AccountID:   "acct_123",
		Email:       "alerts@example.com",
		ActorUserID: "user_123",
	})
	if err == nil || !strings.Contains(err.Error(), "not authorized") {
		t.Fatalf("expected authorization error, got %v", err)
	}
}

func TestRecipientServiceFiltersEligibleRecipients(t *testing.T) {
	repo := newFakeRecipientRepository()
	service := recipientTestService(repo, nil)
	repo.recipients["verified_global"] = AlertRecipient{
		AlertRecipientID: "verified_global",
		AccountID:        "acct_123",
		Email:            "owner@example.com",
		EmailNormalized:  "owner@example.com",
		Channel:          "email",
		Status:           RecipientStatusVerified,
		Subscriptions:    []AlertSubscription{{Family: SubscriptionDeviceDisconnected, Enabled: true}},
	}
	repo.recipients["unverified"] = AlertRecipient{
		AlertRecipientID: "unverified",
		AccountID:        "acct_123",
		Email:            "other@example.com",
		EmailNormalized:  "other@example.com",
		Channel:          "email",
		Status:           RecipientStatusPendingVerification,
		Subscriptions:    []AlertSubscription{{Family: SubscriptionDeviceDisconnected, Enabled: true}},
	}
	repo.recipients["wrong_site"] = AlertRecipient{
		AlertRecipientID: "wrong_site",
		AccountID:        "acct_123",
		SiteID:           "site_other",
		Email:            "site@example.com",
		EmailNormalized:  "site@example.com",
		Channel:          "email",
		Status:           RecipientStatusVerified,
		Subscriptions:    []AlertSubscription{{Family: SubscriptionDeviceDisconnected, Enabled: true}},
	}
	repo.recipients["disabled_family"] = AlertRecipient{
		AlertRecipientID: "disabled_family",
		AccountID:        "acct_123",
		Email:            "disabled@example.com",
		EmailNormalized:  "disabled@example.com",
		Channel:          "email",
		Status:           RecipientStatusVerified,
		Subscriptions:    []AlertSubscription{{Family: SubscriptionDeviceDisconnected, Enabled: false}},
	}

	eligible, err := service.EligibleRecipients(context.Background(), Alert{
		AccountID: "acct_123",
		SiteID:    "site_123",
		Family:    FamilyDeviceDisconnected,
	})
	if err != nil {
		t.Fatalf("eligible recipients: %v", err)
	}
	if len(eligible) != 1 || eligible[0].AlertRecipientID != "verified_global" {
		t.Fatalf("expected only verified subscribed recipient, got %#v", eligible)
	}
}

func TestRecipientServiceMapsBackupFamiliesToCombinedSubscription(t *testing.T) {
	repo := newFakeRecipientRepository()
	service := recipientTestService(repo, nil)
	repo.recipients["backup"] = AlertRecipient{
		AlertRecipientID: "backup",
		AccountID:        "acct_123",
		Email:            "backup@example.com",
		EmailNormalized:  "backup@example.com",
		Channel:          "email",
		Status:           RecipientStatusVerified,
		Subscriptions:    []AlertSubscription{{Family: SubscriptionBackupFailedOrOverdue, Enabled: true}},
	}

	eligible, err := service.EligibleRecipients(context.Background(), Alert{
		AccountID: "acct_123",
		SiteID:    "site_123",
		Family:    FamilyBackupOverdue,
	})
	if err != nil {
		t.Fatalf("eligible recipients: %v", err)
	}
	if len(eligible) != 1 {
		t.Fatalf("expected backup subscription to match overdue alert, got %#v", eligible)
	}
}

func recipientTestService(repo *fakeRecipientRepository, auth RecipientAuthorization) RecipientService {
	return RecipientService{
		Repository:    repo,
		Authorization: auth,
		IDGenerator: func() string {
			return "recipient_123"
		},
		Clock: fixedClock,
	}
}

type fakeRecipientRepository struct {
	recipients map[string]AlertRecipient
}

func newFakeRecipientRepository() *fakeRecipientRepository {
	return &fakeRecipientRepository{recipients: map[string]AlertRecipient{}}
}

func (r *fakeRecipientRepository) SaveRecipient(_ context.Context, recipient AlertRecipient) error {
	r.recipients[recipient.AlertRecipientID] = recipient
	return nil
}

func (r *fakeRecipientRepository) GetRecipient(_ context.Context, recipientID string) (AlertRecipient, error) {
	recipient, ok := r.recipients[recipientID]
	if !ok {
		return AlertRecipient{}, ErrNotFound
	}
	return recipient, nil
}

func (r *fakeRecipientRepository) ListRecipientsForAccount(_ context.Context, accountID string) ([]AlertRecipient, error) {
	var recipients []AlertRecipient
	for _, recipient := range r.recipients {
		if recipient.AccountID == accountID {
			recipients = append(recipients, recipient)
		}
	}
	return recipients, nil
}

type fakeRecipientAuthorization struct {
	allowed bool
	calls   []CreateRecipientRequest
}

func (a *fakeRecipientAuthorization) CanManageAlertRecipients(_ context.Context, actorUserID string, accountID string, siteID string) (bool, error) {
	a.calls = append(a.calls, CreateRecipientRequest{ActorUserID: actorUserID, AccountID: accountID, SiteID: siteID})
	return a.allowed, nil
}
