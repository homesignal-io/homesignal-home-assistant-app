package api

import (
	"encoding/json"
	"net/http"
)

type ErrorBody struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	RequestID string         `json:"request_id"`
	Details   map[string]any `json:"details,omitempty"`
}

type ErrorEnvelope struct {
	Error ErrorBody `json:"error"`
}

func NewErrorEnvelope(ctx RequestContext, code string, message string, details map[string]any) ErrorEnvelope {
	return ErrorEnvelope{
		Error: ErrorBody{
			Code:      code,
			Message:   message,
			RequestID: ctx.RequestID,
			Details:   details,
		},
	}
}

func JSONResponse(status int, body any) (int, map[string]string, []byte) {
	payload, err := json.Marshal(body)
	if err != nil {
		status = http.StatusInternalServerError
		payload = []byte(`{"error":{"code":"INTERNAL_ERROR","message":"Internal error.","request_id":"unknown"}}`)
	}

	return status, map[string]string{
		"Content-Type": "application/json; charset=utf-8",
	}, payload
}
