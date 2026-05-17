package api

import (
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
	"unicode"
)

const (
	RequestIDHeader     = "X-Request-ID"
	CorrelationIDHeader = "X-Correlation-ID"
	maxCorrelationIDLen = 128
)

type CorrelationIDSource string

const (
	CorrelationIDSourceGenerated CorrelationIDSource = "generated"
	CorrelationIDSourceCaller    CorrelationIDSource = "caller"
)

type RequestContext struct {
	RequestID           string
	CorrelationID       string
	CorrelationIDSource CorrelationIDSource
	SourceIP            string
	UserAgent           string
	RouteTemplate       string
	IdempotencyKey      string
}

var requestCounter atomic.Uint64

func NewRequestContext(r *http.Request) RequestContext {
	requestID := NewID("req")
	correlationID := NewID("corr")
	correlationSource := CorrelationIDSourceGenerated
	if r != nil {
		if provided := r.Header.Get(CorrelationIDHeader); ValidCorrelationID(provided) {
			correlationID = provided
			correlationSource = CorrelationIDSourceCaller
		}
	}

	ctx := RequestContext{
		RequestID:           requestID,
		CorrelationID:       correlationID,
		CorrelationIDSource: correlationSource,
	}
	if r != nil {
		ctx.SourceIP = r.RemoteAddr
		ctx.UserAgent = r.UserAgent()
		ctx.IdempotencyKey = r.Header.Get("Idempotency-Key")
	}
	return ctx
}

func NewSyntheticRequestContext() RequestContext {
	return RequestContext{
		RequestID:           NewID("req"),
		CorrelationID:       NewID("corr"),
		CorrelationIDSource: CorrelationIDSourceGenerated,
	}
}

func NewID(prefix string) string {
	return fmt.Sprintf("%s_%d_%d", prefix, time.Now().UTC().UnixNano(), requestCounter.Add(1))
}

func ValidCorrelationID(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > maxCorrelationIDLen {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return false
		}
	}
	return true
}
