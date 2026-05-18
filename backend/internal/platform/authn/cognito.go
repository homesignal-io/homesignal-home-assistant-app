package authn

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	defaultCognitoTokenUse = "access"
	jwksCacheTTL           = time.Hour
	jwtClockSkew           = time.Minute
)

type CognitoVerifierConfig struct {
	Issuer   string
	ClientID string
	TokenUse string
	JWKSURL  string
	Now      func() time.Time
}

func LoadCognitoConfigFromEnv() CognitoVerifierConfig {
	return CognitoVerifierConfig{
		Issuer:   os.Getenv("HOMESIGNAL_COGNITO_ISSUER"),
		ClientID: os.Getenv("HOMESIGNAL_COGNITO_CLIENT_ID"),
		TokenUse: getenvDefault("HOMESIGNAL_COGNITO_TOKEN_USE", defaultCognitoTokenUse),
		JWKSURL:  os.Getenv("HOMESIGNAL_COGNITO_JWKS_URL"),
	}
}

func (c CognitoVerifierConfig) Enabled() bool {
	return strings.TrimSpace(c.Issuer) != "" || strings.TrimSpace(c.ClientID) != "" || strings.TrimSpace(c.JWKSURL) != ""
}

func (c CognitoVerifierConfig) normalized() (CognitoVerifierConfig, error) {
	c.Issuer = strings.TrimRight(strings.TrimSpace(c.Issuer), "/")
	c.ClientID = strings.TrimSpace(c.ClientID)
	c.TokenUse = strings.TrimSpace(c.TokenUse)
	c.JWKSURL = strings.TrimSpace(c.JWKSURL)
	if c.TokenUse == "" {
		c.TokenUse = defaultCognitoTokenUse
	}
	if c.Issuer == "" {
		return CognitoVerifierConfig{}, fmt.Errorf("HOMESIGNAL_COGNITO_ISSUER is required")
	}
	if c.ClientID == "" {
		return CognitoVerifierConfig{}, fmt.Errorf("HOMESIGNAL_COGNITO_CLIENT_ID is required")
	}
	if c.JWKSURL == "" {
		c.JWKSURL = c.Issuer + "/.well-known/jwks.json"
	}
	if c.Now == nil {
		c.Now = time.Now
	}
	return c, nil
}

type CognitoVerifier struct {
	cfg  CognitoVerifierConfig
	jwks *remoteJWKSet
}

func NewCognitoVerifier(cfg CognitoVerifierConfig) (*CognitoVerifier, error) {
	cfg, err := cfg.normalized()
	if err != nil {
		return nil, err
	}
	return &CognitoVerifier{
		cfg: cfg,
		jwks: &remoteJWKSet{
			url:    cfg.JWKSURL,
			client: http.DefaultClient,
		},
	}, nil
}

func (v *CognitoVerifier) VerifyBearerToken(ctx context.Context, token string) (Claims, error) {
	if v == nil {
		return Claims{}, fmt.Errorf("cognito verifier is required")
	}

	header, body, signed, signature, err := parseJWT(token)
	if err != nil {
		return Claims{}, err
	}
	if header.Algorithm != "RS256" {
		return Claims{}, fmt.Errorf("unsupported jwt algorithm %q", header.Algorithm)
	}
	if strings.TrimSpace(header.KeyID) == "" {
		return Claims{}, fmt.Errorf("jwt key id is required")
	}

	key, err := v.jwks.publicKey(ctx, header.KeyID)
	if err != nil {
		return Claims{}, err
	}
	digest := sha256.Sum256([]byte(signed))
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], signature); err != nil {
		return Claims{}, fmt.Errorf("verify jwt signature: %w", err)
	}

	now := v.cfg.Now().UTC()
	if body.Issuer != v.cfg.Issuer {
		return Claims{}, fmt.Errorf("issuer mismatch")
	}
	if body.Subject == "" {
		return Claims{}, fmt.Errorf("subject is required")
	}
	if body.TokenUse != v.cfg.TokenUse {
		return Claims{}, fmt.Errorf("token_use mismatch")
	}
	if !body.matchesClientID(v.cfg.ClientID) {
		return Claims{}, fmt.Errorf("client id mismatch")
	}
	if body.ExpiresAt == 0 || now.After(time.Unix(body.ExpiresAt, 0).Add(jwtClockSkew)) {
		return Claims{}, fmt.Errorf("token expired")
	}
	if body.NotBefore != 0 && now.Add(jwtClockSkew).Before(time.Unix(body.NotBefore, 0)) {
		return Claims{}, fmt.Errorf("token is not valid yet")
	}

	return Claims{
		Subject:  body.Subject,
		Email:    body.Email,
		Issuer:   body.Issuer,
		Audience: body.audienceString(),
		TokenUse: body.TokenUse,
	}, nil
}

type jwtHeader struct {
	Algorithm string `json:"alg"`
	KeyID     string `json:"kid"`
}

type jwtBody struct {
	Subject   string      `json:"sub"`
	Issuer    string      `json:"iss"`
	Audience  jwtAudience `json:"aud"`
	ClientID  string      `json:"client_id"`
	TokenUse  string      `json:"token_use"`
	Email     string      `json:"email"`
	ExpiresAt int64       `json:"exp"`
	NotBefore int64       `json:"nbf"`
}

type jwtAudience []string

func (a *jwtAudience) UnmarshalJSON(data []byte) error {
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		*a = []string{single}
		return nil
	}
	var many []string
	if err := json.Unmarshal(data, &many); err != nil {
		return err
	}
	*a = many
	return nil
}

func (b jwtBody) matchesClientID(clientID string) bool {
	if b.ClientID == clientID {
		return true
	}
	for _, audience := range b.Audience {
		if audience == clientID {
			return true
		}
	}
	return false
}

func (b jwtBody) audienceString() string {
	if len(b.Audience) == 0 {
		return b.ClientID
	}
	return strings.Join(b.Audience, ",")
}

func parseJWT(token string) (jwtHeader, jwtBody, string, []byte, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return jwtHeader{}, jwtBody{}, "", nil, fmt.Errorf("jwt must have three segments")
	}

	var header jwtHeader
	if err := decodeJSONSegment(parts[0], &header); err != nil {
		return jwtHeader{}, jwtBody{}, "", nil, fmt.Errorf("decode jwt header: %w", err)
	}
	var body jwtBody
	if err := decodeJSONSegment(parts[1], &body); err != nil {
		return jwtHeader{}, jwtBody{}, "", nil, fmt.Errorf("decode jwt body: %w", err)
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return jwtHeader{}, jwtBody{}, "", nil, fmt.Errorf("decode jwt signature: %w", err)
	}

	return header, body, parts[0] + "." + parts[1], signature, nil
}

func decodeJSONSegment(segment string, target any) error {
	data, err := base64.RawURLEncoding.DecodeString(segment)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

type remoteJWKSet struct {
	url       string
	client    *http.Client
	mu        sync.Mutex
	keys      map[string]*rsa.PublicKey
	expiresAt time.Time
}

func (s *remoteJWKSet) publicKey(ctx context.Context, keyID string) (*rsa.PublicKey, error) {
	if keyID == "" {
		return nil, fmt.Errorf("key id is required")
	}
	if key, ok, err := s.cachedKey(ctx, keyID, false); err != nil || ok {
		return key, err
	}
	key, ok, err := s.cachedKey(ctx, keyID, true)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("jwt key %q not found", keyID)
	}
	return key, nil
}

func (s *remoteJWKSet) cachedKey(ctx context.Context, keyID string, forceRefresh bool) (*rsa.PublicKey, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	if forceRefresh || s.keys == nil || now.After(s.expiresAt) {
		keys, err := s.fetch(ctx)
		if err != nil {
			return nil, false, err
		}
		s.keys = keys
		s.expiresAt = now.Add(jwksCacheTTL)
	}
	key, ok := s.keys[keyID]
	return key, ok, nil
}

func (s *remoteJWKSet) fetch(ctx context.Context) (map[string]*rsa.PublicKey, error) {
	if s.client == nil {
		s.client = http.DefaultClient
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
	if err != nil {
		return nil, fmt.Errorf("create jwks request: %w", err)
	}
	response, err := s.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch jwks: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch jwks: status %d", response.StatusCode)
	}

	var doc jwksDocument
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&doc); err != nil {
		return nil, fmt.Errorf("decode jwks: %w", err)
	}
	keys := map[string]*rsa.PublicKey{}
	for _, key := range doc.Keys {
		publicKey, err := key.publicKey()
		if err != nil {
			return nil, err
		}
		keys[key.KeyID] = publicKey
	}
	return keys, nil
}

type jwksDocument struct {
	Keys []jwk `json:"keys"`
}

type jwk struct {
	KeyType   string `json:"kty"`
	KeyID     string `json:"kid"`
	Use       string `json:"use"`
	Algorithm string `json:"alg"`
	Modulus   string `json:"n"`
	Exponent  string `json:"e"`
}

func (k jwk) publicKey() (*rsa.PublicKey, error) {
	if k.KeyID == "" {
		return nil, fmt.Errorf("jwks key id is required")
	}
	if k.KeyType != "RSA" {
		return nil, fmt.Errorf("unsupported jwks key type %q", k.KeyType)
	}
	if k.Algorithm != "" && k.Algorithm != "RS256" {
		return nil, fmt.Errorf("unsupported jwks key algorithm %q", k.Algorithm)
	}

	nBytes, err := base64.RawURLEncoding.DecodeString(k.Modulus)
	if err != nil {
		return nil, fmt.Errorf("decode jwks modulus: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(k.Exponent)
	if err != nil {
		return nil, fmt.Errorf("decode jwks exponent: %w", err)
	}
	exponent := 0
	for _, b := range eBytes {
		exponent = exponent<<8 + int(b)
	}
	if exponent == 0 {
		return nil, fmt.Errorf("jwks exponent is empty")
	}

	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: exponent,
	}, nil
}

func getenvDefault(name string, fallback string) string {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	return value
}
