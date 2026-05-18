package authorization

import (
	"context"
	"fmt"
	"regexp"

	"github.com/homesignal-io/homesignal-home-assistant-app/backend/internal/domain/ports"
)

var actionPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*:[a-z][a-z0-9_]*$`)

type Subject struct {
	Type string
	ID   ports.UserID
}

type Resource struct {
	Type      string
	ID        string
	AccountID ports.AccountID
	SiteID    ports.SiteID
}

type Decision struct {
	Allowed bool
	Reason  string
}

type Service struct {
	Auth ports.AuthRepository
}

func (s Service) Can(ctx context.Context, subject Subject, action string, resource Resource) (Decision, error) {
	if !ValidAction(action) {
		return Decision{Allowed: false, Reason: "invalid_action"}, nil
	}
	if subject.Type != "user" || subject.ID == "" {
		return Decision{Allowed: false, Reason: "unsupported_subject"}, nil
	}
	if s.Auth == nil {
		return Decision{}, fmt.Errorf("auth repository is required")
	}

	permissions, err := s.Auth.ListPermissionKeys(ctx, subject.ID, resource.AccountID, resource.SiteID)
	if err != nil {
		return Decision{}, fmt.Errorf("list permissions: %w", err)
	}
	for _, permission := range permissions {
		if permission == action {
			return Decision{Allowed: true, Reason: "permission_granted"}, nil
		}
	}
	return Decision{Allowed: false, Reason: "permission_missing"}, nil
}

func ValidAction(action string) bool {
	return actionPattern.MatchString(action)
}
