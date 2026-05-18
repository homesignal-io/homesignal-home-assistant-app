package controlplane

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/homesignal-io/homesignal-home-assistant-app/backend/internal/platform/api"
)

type idempotencyStore struct {
	mu      sync.Mutex
	ttl     time.Duration
	entries map[string]idempotencyEntry
}

type idempotencyEntry struct {
	requestHash string
	expiresAt   time.Time
	response    Response
}

func newIdempotencyStore(ttl time.Duration) *idempotencyStore {
	return &idempotencyStore{
		ttl:     ttl,
		entries: map[string]idempotencyEntry{},
	}
}

func (s *idempotencyStore) getOrStore(
	ctx api.RequestContext,
	scope string,
	key string,
	requestHash string,
	produce func() Response,
) Response {
	now := time.Now().UTC()
	cacheKey := scope + "|" + key

	s.mu.Lock()
	if existing, ok := s.entries[cacheKey]; ok {
		if now.After(existing.expiresAt) {
			delete(s.entries, cacheKey)
		} else if existing.requestHash != requestHash {
			s.mu.Unlock()
			return errorResponse(ctx, http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "Idempotency key was reused with a different request.")
		} else {
			response := cloneResponse(existing.response)
			response.Headers["X-HomeSignal-Idempotency-Replayed"] = "true"
			s.mu.Unlock()
			return response
		}
	}
	s.mu.Unlock()

	response := produce()

	s.mu.Lock()
	s.entries[cacheKey] = idempotencyEntry{
		requestHash: requestHash,
		expiresAt:   now.Add(s.ttl),
		response:    cloneResponse(response),
	}
	s.mu.Unlock()

	return response
}

type rateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	buckets map[string]rateBucket
}

type rateBucket struct {
	resetAt time.Time
	count   int
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		limit:   limit,
		window:  window,
		buckets: map[string]rateBucket{},
	}
}

func (l *rateLimiter) allow(scope string) (bool, int) {
	now := time.Now().UTC()

	l.mu.Lock()
	defer l.mu.Unlock()

	bucket := l.buckets[scope]
	if bucket.resetAt.IsZero() || !now.Before(bucket.resetAt) {
		bucket = rateBucket{resetAt: now.Add(l.window)}
	}
	bucket.count++
	l.buckets[scope] = bucket

	if bucket.count <= l.limit {
		return true, 0
	}

	retryAfter := int(time.Until(bucket.resetAt).Seconds())
	if retryAfter < 1 {
		retryAfter = 1
	}
	return false, retryAfter
}

func rateLimitedResponse(ctx api.RequestContext, retryAfter int) Response {
	response := errorResponse(ctx, http.StatusTooManyRequests, "RATE_LIMITED", "Too many requests.")
	response.Headers["Retry-After"] = strconv.Itoa(retryAfter)
	return response
}

func requestHash(method string, path string, body []byte) string {
	hash := sha256.New()
	hash.Write([]byte(method))
	hash.Write([]byte{'\n'})
	hash.Write([]byte(path))
	hash.Write([]byte{'\n'})
	hash.Write(body)
	return hex.EncodeToString(hash.Sum(nil))
}

func cloneResponse(response Response) Response {
	headers := make(map[string]string, len(response.Headers))
	for key, value := range response.Headers {
		headers[key] = value
	}
	body := append([]byte(nil), response.Body...)
	return Response{
		StatusCode: response.StatusCode,
		Headers:    headers,
		Body:       body,
	}
}
