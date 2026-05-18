package authn

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCognitoVerifierAcceptsValidAccessToken(t *testing.T) {
	privateKey := generateTestKey(t)
	server := httptest.NewServer(jwksHandler(t, privateKey, "kid-1"))
	defer server.Close()

	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	verifier, err := NewCognitoVerifier(CognitoVerifierConfig{
		Issuer:   "https://cognito-idp.us-east-1.amazonaws.com/us-east-1_test",
		ClientID: "client-123",
		JWKSURL:  server.URL,
		Now:      func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewCognitoVerifier returned error: %v", err)
	}

	token := signedTestJWT(t, privateKey, "kid-1", map[string]any{
		"iss":       "https://cognito-idp.us-east-1.amazonaws.com/us-east-1_test",
		"sub":       "cognito-sub",
		"client_id": "client-123",
		"token_use": "access",
		"email":     "person@example.com",
		"exp":       now.Add(time.Hour).Unix(),
	})

	claims, err := verifier.VerifyBearerToken(context.Background(), token)
	if err != nil {
		t.Fatalf("VerifyBearerToken returned error: %v", err)
	}
	if claims.Subject != "cognito-sub" || claims.Email != "person@example.com" || claims.TokenUse != "access" {
		t.Fatalf("claims = %#v", claims)
	}
}

func TestCognitoVerifierRejectsWrongClientID(t *testing.T) {
	privateKey := generateTestKey(t)
	server := httptest.NewServer(jwksHandler(t, privateKey, "kid-1"))
	defer server.Close()

	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	verifier, err := NewCognitoVerifier(CognitoVerifierConfig{
		Issuer:   "https://cognito-idp.us-east-1.amazonaws.com/us-east-1_test",
		ClientID: "client-123",
		JWKSURL:  server.URL,
		Now:      func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewCognitoVerifier returned error: %v", err)
	}

	token := signedTestJWT(t, privateKey, "kid-1", map[string]any{
		"iss":       "https://cognito-idp.us-east-1.amazonaws.com/us-east-1_test",
		"sub":       "cognito-sub",
		"client_id": "other-client",
		"token_use": "access",
		"exp":       now.Add(time.Hour).Unix(),
	})

	if _, err := verifier.VerifyBearerToken(context.Background(), token); err == nil {
		t.Fatal("expected client id error")
	}
}

func TestCognitoVerifierRejectsExpiredToken(t *testing.T) {
	privateKey := generateTestKey(t)
	server := httptest.NewServer(jwksHandler(t, privateKey, "kid-1"))
	defer server.Close()

	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	verifier, err := NewCognitoVerifier(CognitoVerifierConfig{
		Issuer:   "https://cognito-idp.us-east-1.amazonaws.com/us-east-1_test",
		ClientID: "client-123",
		JWKSURL:  server.URL,
		Now:      func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewCognitoVerifier returned error: %v", err)
	}

	token := signedTestJWT(t, privateKey, "kid-1", map[string]any{
		"iss":       "https://cognito-idp.us-east-1.amazonaws.com/us-east-1_test",
		"sub":       "cognito-sub",
		"client_id": "client-123",
		"token_use": "access",
		"exp":       now.Add(-2 * time.Minute).Unix(),
	})

	if _, err := verifier.VerifyBearerToken(context.Background(), token); err == nil {
		t.Fatal("expected expiration error")
	}
}

func generateTestKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate test key: %v", err)
	}
	return key
}

func jwksHandler(t *testing.T, key *rsa.PrivateKey, keyID string) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]string{
				{
					"kty": "RSA",
					"kid": keyID,
					"use": "sig",
					"alg": "RS256",
					"n":   base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes()),
					"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.PublicKey.E)).Bytes()),
				},
			},
		}); err != nil {
			t.Fatalf("encode jwks: %v", err)
		}
	}
}

func signedTestJWT(t *testing.T, key *rsa.PrivateKey, keyID string, claims map[string]any) string {
	t.Helper()
	header := map[string]any{
		"alg": "RS256",
		"kid": keyID,
		"typ": "JWT",
	}
	unsigned := encodeTestJWTPart(t, header) + "." + encodeTestJWTPart(t, claims)
	digest := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("sign jwt: %v", err)
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func encodeTestJWTPart(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal jwt part: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(data)
}
