package authn

import (
	"context"
	"errors"
	"testing"

	"github.com/homesignal-io/homesignal-home-assistant-app/backend/internal/domain/ports"
)

func TestBearerToken(t *testing.T) {
	token, err := BearerToken("Bearer abc123")
	if err != nil {
		t.Fatalf("BearerToken returned error: %v", err)
	}
	if token != "abc123" {
		t.Fatalf("token = %q", token)
	}
}

func TestBearerTokenRejectsInvalidHeader(t *testing.T) {
	_, err := BearerToken("Basic abc123")
	if !errors.Is(err, ErrMissingCredential) {
		t.Fatalf("expected ErrMissingCredential, got %v", err)
	}
}

func TestHumanAuthenticatorMapsVerifiedClaimsToLocalUser(t *testing.T) {
	verifier := NewFakeVerifier()
	verifier.Tokens["token"] = Claims{Subject: "cognito-sub", Email: "person@example.com", TokenUse: "access"}
	users := ports.NewFakeAuthRepository()
	users.UsersBySubject["cognito-sub"] = ports.UserSubject{
		ID:         "user_123",
		CognitoSub: "cognito-sub",
		Email:      "person@example.com",
		Status:     "active",
	}

	subject, err := HumanAuthenticator{Verifier: verifier, Users: users}.Authenticate(context.Background(), "Bearer token")
	if err != nil {
		t.Fatalf("Authenticate returned error: %v", err)
	}
	if subject.Type != "user" || subject.ID != "user_123" || subject.AuthMethod != "cognito_jwt" {
		t.Fatalf("subject = %#v", subject)
	}
}

func TestHumanAuthenticatorRejectsUnknownToken(t *testing.T) {
	_, err := HumanAuthenticator{
		Verifier: NewFakeVerifier(),
		Users:    ports.NewFakeAuthRepository(),
	}.Authenticate(context.Background(), "Bearer nope")
	if !errors.Is(err, ErrInvalidCredential) {
		t.Fatalf("expected ErrInvalidCredential, got %v", err)
	}
}

func TestHumanAuthenticatorRejectsDisabledUser(t *testing.T) {
	verifier := NewFakeVerifier()
	verifier.Tokens["token"] = Claims{Subject: "cognito-sub"}
	users := ports.NewFakeAuthRepository()
	users.UsersBySubject["cognito-sub"] = ports.UserSubject{
		ID:         "user_123",
		CognitoSub: "cognito-sub",
		Status:     "disabled",
	}

	_, err := HumanAuthenticator{Verifier: verifier, Users: users}.Authenticate(context.Background(), "Bearer token")
	if !errors.Is(err, ErrUserDisabled) {
		t.Fatalf("expected ErrUserDisabled, got %v", err)
	}
}
