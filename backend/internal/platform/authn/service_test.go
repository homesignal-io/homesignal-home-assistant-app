package authn

import (
	"context"
	"errors"
	"testing"
)

func TestStaticServiceAuthenticatorMapsKnownPrincipal(t *testing.T) {
	subject, err := DefaultStaticServiceAuthenticator().AuthenticateService(context.Background(), "service:telemetry-ingest")
	if err != nil {
		t.Fatalf("AuthenticateService returned error: %v", err)
	}
	if subject.Type != "service" || subject.ID != "service:telemetry-ingest" || subject.AuthMethod != "static_service_principal" {
		t.Fatalf("subject = %#v", subject)
	}
}

func TestStaticServiceAuthenticatorRejectsUnknownPrincipal(t *testing.T) {
	_, err := DefaultStaticServiceAuthenticator().AuthenticateService(context.Background(), "service:unknown")
	if !errors.Is(err, ErrUnknownServicePrincipal) {
		t.Fatalf("expected ErrUnknownServicePrincipal, got %v", err)
	}
}

func TestStaticServiceAuthenticatorRejectsMissingCredential(t *testing.T) {
	_, err := DefaultStaticServiceAuthenticator().AuthenticateService(context.Background(), " ")
	if !errors.Is(err, ErrMissingServiceCredential) {
		t.Fatalf("expected ErrMissingServiceCredential, got %v", err)
	}
}
