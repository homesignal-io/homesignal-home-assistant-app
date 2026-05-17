package authn

import (
	"context"
	"fmt"
	"sync"
)

type FakeVerifier struct {
	mu     sync.Mutex
	Tokens map[string]Claims
}

func NewFakeVerifier() *FakeVerifier {
	return &FakeVerifier{
		Tokens: map[string]Claims{},
	}
}

func (v *FakeVerifier) VerifyBearerToken(_ context.Context, token string) (Claims, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	claims, ok := v.Tokens[token]
	if !ok {
		return Claims{}, fmt.Errorf("token not recognized")
	}
	return claims, nil
}
