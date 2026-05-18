package alerting

import (
	"context"
	"fmt"
	"net/mail"
	"strings"
	"time"
)

const (
	RecipientStatusPendingVerification RecipientStatus = "pending_verification"
	RecipientStatusVerified            RecipientStatus = "verified"
	RecipientStatusDisabled            RecipientStatus = "disabled"
	RecipientStatusDeleted             RecipientStatus = "deleted"

	SubscriptionDeviceDisconnected    SubscriptionFamily = "device_disconnected"
	SubscriptionBackupFailedOrOverdue SubscriptionFamily = "backup_failed_or_overdue"
	SubscriptionAppUpdateAttention    SubscriptionFamily = "app_update_attention"
)

type RecipientStatus string
type SubscriptionFamily string

type AlertRecipient struct {
	AlertRecipientID string
	AccountID        string
	SiteID           string
	Email            string
	EmailNormalized  string
	DisplayLabel     string
	Channel          string
	Status           RecipientStatus
	CreatedByUserID  string
	VerifiedAt       *time.Time
	DisabledAt       *time.Time
	DeletedAt        *time.Time
	Subscriptions    []AlertSubscription
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type AlertSubscription struct {
	Family  SubscriptionFamily
	Enabled bool
}

type CreateRecipientRequest struct {
	AccountID          string
	SiteID             string
	Email              string
	DisplayLabel       string
	ActorUserID        string
	ActorEmail         string
	ActorEmailVerified bool
	SubscribedFamilies []SubscriptionFamily
}

type UpdateRecipientRequest struct {
	AlertRecipientID   string
	AccountID          string
	SiteID             string
	DisplayLabel       string
	Status             RecipientStatus
	ActorUserID        string
	SubscribedFamilies []SubscriptionFamily
}

type DeleteRecipientRequest struct {
	AlertRecipientID string
	AccountID        string
	SiteID           string
	ActorUserID      string
}

type RecipientRepository interface {
	SaveRecipient(ctx context.Context, recipient AlertRecipient) error
	GetRecipient(ctx context.Context, recipientID string) (AlertRecipient, error)
	ListRecipientsForAccount(ctx context.Context, accountID string) ([]AlertRecipient, error)
}

type RecipientAuthorization interface {
	CanManageAlertRecipients(ctx context.Context, actorUserID string, accountID string, siteID string) (bool, error)
}

type RecipientService struct {
	Repository    RecipientRepository
	Authorization RecipientAuthorization
	IDGenerator   IDGenerator
	Clock         Clock
}

func (s RecipientService) CreateRecipient(ctx context.Context, req CreateRecipientRequest) (AlertRecipient, error) {
	if s.Repository == nil {
		return AlertRecipient{}, fmt.Errorf("alert recipient repository is required")
	}
	if err := s.authorize(ctx, req.ActorUserID, req.AccountID, req.SiteID); err != nil {
		return AlertRecipient{}, err
	}
	now := s.now()
	recipientID, err := s.newRecipientID()
	if err != nil {
		return AlertRecipient{}, err
	}
	email, normalized, err := normalizeEmail(req.Email)
	if err != nil {
		return AlertRecipient{}, err
	}
	status := RecipientStatusPendingVerification
	var verifiedAt *time.Time
	if req.ActorEmailVerified {
		_, actorEmail, actorErr := normalizeEmail(req.ActorEmail)
		if actorErr == nil && actorEmail == normalized {
			status = RecipientStatusVerified
			verifiedAt = &now
		}
	}
	recipient := AlertRecipient{
		AlertRecipientID: recipientID,
		AccountID:        strings.TrimSpace(req.AccountID),
		SiteID:           strings.TrimSpace(req.SiteID),
		Email:            email,
		EmailNormalized:  normalized,
		DisplayLabel:     strings.TrimSpace(req.DisplayLabel),
		Channel:          "email",
		Status:           status,
		CreatedByUserID:  strings.TrimSpace(req.ActorUserID),
		VerifiedAt:       verifiedAt,
		Subscriptions:    normalizeSubscriptions(req.SubscribedFamilies),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := validateRecipient(recipient); err != nil {
		return AlertRecipient{}, err
	}
	if err := s.Repository.SaveRecipient(ctx, recipient); err != nil {
		return AlertRecipient{}, fmt.Errorf("save alert recipient: %w", err)
	}
	return recipient, nil
}

func (s RecipientService) UpdateRecipient(ctx context.Context, req UpdateRecipientRequest) (AlertRecipient, error) {
	if s.Repository == nil {
		return AlertRecipient{}, fmt.Errorf("alert recipient repository is required")
	}
	existing, err := s.Repository.GetRecipient(ctx, strings.TrimSpace(req.AlertRecipientID))
	if err != nil {
		return AlertRecipient{}, fmt.Errorf("load alert recipient: %w", err)
	}
	if err := s.authorize(ctx, req.ActorUserID, existing.AccountID, existing.SiteID); err != nil {
		return AlertRecipient{}, err
	}
	if strings.TrimSpace(req.AccountID) != "" && strings.TrimSpace(req.AccountID) != existing.AccountID {
		return AlertRecipient{}, fmt.Errorf("account_id cannot be changed")
	}
	if strings.TrimSpace(req.SiteID) != "" && strings.TrimSpace(req.SiteID) != existing.SiteID {
		return AlertRecipient{}, fmt.Errorf("site_id cannot be changed")
	}
	if strings.TrimSpace(req.DisplayLabel) != "" {
		existing.DisplayLabel = strings.TrimSpace(req.DisplayLabel)
	}
	if req.Status != "" {
		switch req.Status {
		case RecipientStatusPendingVerification, RecipientStatusVerified, RecipientStatusDisabled:
			existing.Status = req.Status
		default:
			return AlertRecipient{}, fmt.Errorf("unsupported recipient status %q", req.Status)
		}
	}
	if len(req.SubscribedFamilies) > 0 {
		existing.Subscriptions = normalizeSubscriptions(req.SubscribedFamilies)
	}
	existing.UpdatedAt = s.now()
	if err := validateRecipient(existing); err != nil {
		return AlertRecipient{}, err
	}
	if err := s.Repository.SaveRecipient(ctx, existing); err != nil {
		return AlertRecipient{}, fmt.Errorf("save alert recipient: %w", err)
	}
	return existing, nil
}

func (s RecipientService) DeleteRecipient(ctx context.Context, req DeleteRecipientRequest) (AlertRecipient, error) {
	if s.Repository == nil {
		return AlertRecipient{}, fmt.Errorf("alert recipient repository is required")
	}
	existing, err := s.Repository.GetRecipient(ctx, strings.TrimSpace(req.AlertRecipientID))
	if err != nil {
		return AlertRecipient{}, fmt.Errorf("load alert recipient: %w", err)
	}
	if err := s.authorize(ctx, req.ActorUserID, existing.AccountID, existing.SiteID); err != nil {
		return AlertRecipient{}, err
	}
	now := s.now()
	existing.Status = RecipientStatusDeleted
	existing.DeletedAt = &now
	existing.UpdatedAt = now
	if err := s.Repository.SaveRecipient(ctx, existing); err != nil {
		return AlertRecipient{}, fmt.Errorf("delete alert recipient: %w", err)
	}
	return existing, nil
}

func (s RecipientService) EligibleRecipients(ctx context.Context, alert Alert) ([]AlertRecipient, error) {
	if s.Repository == nil {
		return nil, fmt.Errorf("alert recipient repository is required")
	}
	family, ok := subscriptionFamilyForAlert(alert.Family)
	if !ok {
		return nil, nil
	}
	recipients, err := s.Repository.ListRecipientsForAccount(ctx, alert.AccountID)
	if err != nil {
		return nil, fmt.Errorf("list alert recipients: %w", err)
	}
	eligible := make([]AlertRecipient, 0, len(recipients))
	for _, recipient := range recipients {
		if recipient.Status != RecipientStatusVerified || recipient.Channel != "email" {
			continue
		}
		if recipient.SiteID != "" && recipient.SiteID != alert.SiteID {
			continue
		}
		if subscriptionEnabled(recipient.Subscriptions, family) {
			eligible = append(eligible, recipient)
		}
	}
	return eligible, nil
}

func (s RecipientService) authorize(ctx context.Context, actorUserID string, accountID string, siteID string) error {
	actorUserID = strings.TrimSpace(actorUserID)
	accountID = strings.TrimSpace(accountID)
	siteID = strings.TrimSpace(siteID)
	if actorUserID == "" || accountID == "" {
		return fmt.Errorf("actor_user_id and account_id are required")
	}
	if s.Authorization == nil {
		return nil
	}
	allowed, err := s.Authorization.CanManageAlertRecipients(ctx, actorUserID, accountID, siteID)
	if err != nil {
		return fmt.Errorf("authorize alert recipient management: %w", err)
	}
	if !allowed {
		return fmt.Errorf("actor is not authorized to manage alert recipients")
	}
	return nil
}

func (s RecipientService) newRecipientID() (string, error) {
	if s.IDGenerator == nil {
		return "", fmt.Errorf("alert recipient id generator is required")
	}
	id := strings.TrimSpace(s.IDGenerator())
	if id == "" {
		return "", fmt.Errorf("alert recipient id is required")
	}
	return id, nil
}

func (s RecipientService) now() time.Time {
	if s.Clock != nil {
		return s.Clock().UTC()
	}
	return time.Now().UTC()
}

func validateRecipient(recipient AlertRecipient) error {
	if recipient.AlertRecipientID == "" || recipient.AccountID == "" {
		return fmt.Errorf("alert_recipient_id and account_id are required")
	}
	if recipient.Channel != "email" {
		return fmt.Errorf("unsupported alert recipient channel %q", recipient.Channel)
	}
	if recipient.Email == "" || recipient.EmailNormalized == "" {
		return fmt.Errorf("recipient email is required")
	}
	switch recipient.Status {
	case RecipientStatusPendingVerification, RecipientStatusVerified, RecipientStatusDisabled, RecipientStatusDeleted:
	default:
		return fmt.Errorf("unsupported recipient status %q", recipient.Status)
	}
	if len(recipient.Subscriptions) == 0 {
		return fmt.Errorf("at least one alert subscription is required")
	}
	for _, subscription := range recipient.Subscriptions {
		if _, ok := validSubscriptionFamilies()[subscription.Family]; !ok {
			return fmt.Errorf("unsupported subscription family %q", subscription.Family)
		}
	}
	return nil
}

func normalizeEmail(value string) (string, string, error) {
	value = strings.TrimSpace(value)
	parsed, err := mail.ParseAddress(value)
	if err != nil {
		return "", "", fmt.Errorf("recipient email must be valid")
	}
	email := strings.TrimSpace(parsed.Address)
	normalized := strings.ToLower(email)
	if normalized == "" {
		return "", "", fmt.Errorf("recipient email is required")
	}
	return email, normalized, nil
}

func normalizeSubscriptions(families []SubscriptionFamily) []AlertSubscription {
	if len(families) == 0 {
		families = []SubscriptionFamily{SubscriptionDeviceDisconnected, SubscriptionBackupFailedOrOverdue, SubscriptionAppUpdateAttention}
	}
	seen := map[SubscriptionFamily]bool{}
	subscriptions := make([]AlertSubscription, 0, len(families))
	for _, family := range families {
		family = SubscriptionFamily(strings.TrimSpace(string(family)))
		if _, ok := validSubscriptionFamilies()[family]; !ok || seen[family] {
			continue
		}
		seen[family] = true
		subscriptions = append(subscriptions, AlertSubscription{Family: family, Enabled: true})
	}
	return subscriptions
}

func validSubscriptionFamilies() map[SubscriptionFamily]bool {
	return map[SubscriptionFamily]bool{
		SubscriptionDeviceDisconnected:    true,
		SubscriptionBackupFailedOrOverdue: true,
		SubscriptionAppUpdateAttention:    true,
	}
}

func subscriptionFamilyForAlert(family Family) (SubscriptionFamily, bool) {
	switch family {
	case FamilyDeviceDisconnected:
		return SubscriptionDeviceDisconnected, true
	case FamilyBackupFailed, FamilyBackupOverdue:
		return SubscriptionBackupFailedOrOverdue, true
	case FamilyAppUpdateAttention:
		return SubscriptionAppUpdateAttention, true
	default:
		return "", false
	}
}

func subscriptionEnabled(subscriptions []AlertSubscription, family SubscriptionFamily) bool {
	for _, subscription := range subscriptions {
		if subscription.Family == family && subscription.Enabled {
			return true
		}
	}
	return false
}
