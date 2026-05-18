package authn

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/homesignal-io/homesignal-home-assistant-app/backend/internal/domain/ports"
)

var (
	ErrMissingCredential = errors.New("missing credential")
	ErrInvalidCredential = errors.New("invalid credential")
	ErrUserNotFound      = errors.New("user not found")
	ErrUserDisabled      = errors.New("user disabled")
)

type Claims struct {
	Subject  string
	Email    string
	Issuer   string
	Audience string
	TokenUse string
}

type Verifier interface {
	VerifyBearerToken(ctx context.Context, token string) (Claims, error)
}

type Subject struct {
	Type       string
	ID         ports.UserID
	AuthMethod string
	Email      string
}

type HumanAuthenticator struct {
	Verifier Verifier
	Users    ports.AuthRepository
}

func (a HumanAuthenticator) Authenticate(ctx context.Context, authorizationHeader string) (Subject, error) {
	if a.Verifier == nil {
		return Subject{}, fmt.Errorf("auth verifier is required")
	}
	if a.Users == nil {
		return Subject{}, fmt.Errorf("auth repository is required")
	}

	token, err := BearerToken(authorizationHeader)
	if err != nil {
		return Subject{}, err
	}
	claims, err := a.Verifier.VerifyBearerToken(ctx, token)
	if err != nil {
		return Subject{}, fmt.Errorf("%w: %v", ErrInvalidCredential, err)
	}
	if strings.TrimSpace(claims.Subject) == "" {
		return Subject{}, fmt.Errorf("%w: subject is empty", ErrInvalidCredential)
	}

	user, err := a.Users.GetUserByCognitoSub(ctx, claims.Subject)
	if err != nil {
		return Subject{}, fmt.Errorf("%w: %v", ErrUserNotFound, err)
	}
	if user.Status != "active" {
		return Subject{}, ErrUserDisabled
	}

	return Subject{
		Type:       "user",
		ID:         user.ID,
		AuthMethod: "cognito_jwt",
		Email:      user.Email,
	}, nil
}

func BearerToken(authorizationHeader string) (string, error) {
	parts := strings.Fields(authorizationHeader)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
		return "", ErrMissingCredential
	}
	return parts[1], nil
}
