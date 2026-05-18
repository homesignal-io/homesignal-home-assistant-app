package authorization

import (
	"context"
	"testing"

	"github.com/homesignal-io/homesignal-home-assistant-app/backend/internal/domain/ports"
)

func TestServiceCanAllowsMatchingPermission(t *testing.T) {
	authRepo := ports.NewFakeAuthRepository()
	authRepo.Permissions["user_123"] = []string{"site:view"}

	decision, err := Service{Auth: authRepo}.Can(context.Background(), Subject{Type: "user", ID: "user_123"}, "site:view", Resource{
		AccountID: "acct_123",
		SiteID:    "site_123",
	})
	if err != nil {
		t.Fatalf("Can returned error: %v", err)
	}
	if !decision.Allowed {
		t.Fatalf("expected allowed decision, got %#v", decision)
	}
}

func TestServiceCanDeniesMissingPermission(t *testing.T) {
	authRepo := ports.NewFakeAuthRepository()
	authRepo.Permissions["user_123"] = []string{"site:view"}

	decision, err := Service{Auth: authRepo}.Can(context.Background(), Subject{Type: "user", ID: "user_123"}, "site:update", Resource{})
	if err != nil {
		t.Fatalf("Can returned error: %v", err)
	}
	if decision.Allowed || decision.Reason != "permission_missing" {
		t.Fatalf("expected permission_missing denial, got %#v", decision)
	}
}

func TestServiceCanRejectsInvalidActionShape(t *testing.T) {
	decision, err := Service{Auth: ports.NewFakeAuthRepository()}.Can(context.Background(), Subject{Type: "user", ID: "user_123"}, "Site:View", Resource{})
	if err != nil {
		t.Fatalf("Can returned error: %v", err)
	}
	if decision.Allowed || decision.Reason != "invalid_action" {
		t.Fatalf("expected invalid_action denial, got %#v", decision)
	}
}

func TestServiceCanRejectsUnsupportedSubject(t *testing.T) {
	decision, err := Service{Auth: ports.NewFakeAuthRepository()}.Can(context.Background(), Subject{Type: "api_key", ID: "user_123"}, "site:view", Resource{})
	if err != nil {
		t.Fatalf("Can returned error: %v", err)
	}
	if decision.Allowed || decision.Reason != "unsupported_subject" {
		t.Fatalf("expected unsupported_subject denial, got %#v", decision)
	}
}

func TestValidAction(t *testing.T) {
	tests := []struct {
		action string
		want   bool
	}{
		{action: "site:view", want: true},
		{action: "device_claim_invite:create", want: true},
		{action: "site-view", want: false},
		{action: "site:View", want: false},
		{action: "site:", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.action, func(t *testing.T) {
			if got := ValidAction(tt.action); got != tt.want {
				t.Fatalf("ValidAction(%q) = %v, want %v", tt.action, got, tt.want)
			}
		})
	}
}
