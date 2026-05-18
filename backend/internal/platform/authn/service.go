package authn

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrMissingServiceCredential = errors.New("missing service credential")
	ErrUnknownServicePrincipal  = errors.New("unknown service principal")
)

type ServiceSubject struct {
	Type       string
	ID         string
	Principal  string
	AuthMethod string
}

type ServiceAuthenticator interface {
	AuthenticateService(ctx context.Context, credential string) (ServiceSubject, error)
}

type StaticServiceAuthenticator struct {
	Principals map[string]string
}

func DefaultStaticServiceAuthenticator() StaticServiceAuthenticator {
	return StaticServiceAuthenticator{
		Principals: map[string]string{
			"service:telemetry-ingest": "service:telemetry-ingest",
		},
	}
}

func (a StaticServiceAuthenticator) AuthenticateService(_ context.Context, credential string) (ServiceSubject, error) {
	principal := strings.TrimSpace(credential)
	if principal == "" {
		return ServiceSubject{}, ErrMissingServiceCredential
	}
	subjectID, ok := a.Principals[principal]
	if !ok || strings.TrimSpace(subjectID) == "" {
		return ServiceSubject{}, fmt.Errorf("%w: %s", ErrUnknownServicePrincipal, principal)
	}
	return ServiceSubject{
		Type:       "service",
		ID:         subjectID,
		Principal:  principal,
		AuthMethod: "static_service_principal",
	}, nil
}
