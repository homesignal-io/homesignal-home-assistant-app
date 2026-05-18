package controlplane

import (
	"context"

	"github.com/homesignal-io/homesignal-home-assistant-app/backend/internal/platform/authn"
)

type PublicReadModelProvider interface {
	Dashboard(ctx context.Context, subject authn.Subject) (any, error)
	Devices(ctx context.Context, subject authn.Subject) (any, error)
	Activity(ctx context.Context, subject authn.Subject) (any, error)
}
