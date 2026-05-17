package lambdaproxy

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/homesignal-io/homesignal-home-assistant/backend/internal/controlplane"
)

type Handler interface {
	Serve(method string, path string, headers map[string]string, body []byte) controlplane.Response
}

type apiGatewayV2Event struct {
	Version         string            `json:"version"`
	RawPath         string            `json:"rawPath"`
	Headers         map[string]string `json:"headers"`
	Body            string            `json:"body"`
	IsBase64Encoded bool              `json:"isBase64Encoded"`
	RequestContext  struct {
		RequestID string `json:"requestId"`
		HTTP struct {
			Method string `json:"method"`
			Path   string `json:"path"`
		} `json:"http"`
	} `json:"requestContext"`
}

type apiGatewayV2Response struct {
	StatusCode      int               `json:"statusCode"`
	Headers         map[string]string `json:"headers"`
	Body            string            `json:"body"`
	IsBase64Encoded bool              `json:"isBase64Encoded"`
}

type invocationError struct {
	ErrorMessage string `json:"errorMessage"`
	ErrorType    string `json:"errorType"`
}

type requestSummary struct {
	Method     string
	Path       string
	RequestID  string
	StatusCode int
	Duration   time.Duration
}

func Run(ctx context.Context, runtimeAPI string, handler Handler, logger *slog.Logger) error {
	if runtimeAPI == "" {
		return fmt.Errorf("missing AWS Lambda runtime API")
	}
	if logger == nil {
		logger = slog.Default()
	}

	endpoint := "http://" + strings.TrimPrefix(runtimeAPI, "http://") + "/2018-06-01/runtime/invocation"
	client := &http.Client{}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		invocation, requestID, err := nextInvocation(ctx, client, endpoint)
		if err != nil {
			return err
		}

		response, summary, err := handleInvocation(handler, invocation)
		if err != nil {
			logger.Error("invocation failed", "error", err, "request_id", requestID)
			if postErr := postInvocationError(ctx, client, endpoint, requestID, err); postErr != nil {
				return postErr
			}
			continue
		}
		if summary.RequestID == "" {
			summary.RequestID = requestID
		}
		logger.Info(
			"request completed",
			"method", summary.Method,
			"path", summary.Path,
			"status", summary.StatusCode,
			"duration_ms", summary.Duration.Milliseconds(),
			"request_id", summary.RequestID,
		)

		if err := postInvocationResponse(ctx, client, endpoint, requestID, response); err != nil {
			return err
		}
	}
}

func nextInvocation(ctx context.Context, client *http.Client, endpoint string) ([]byte, string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"/next", nil)
	if err != nil {
		return nil, "", err
	}

	response, err := client.Do(request)
	if err != nil {
		return nil, "", err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, "", err
	}
	if response.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("lambda runtime next returned %d: %s", response.StatusCode, string(body))
	}

	requestID := response.Header.Get("Lambda-Runtime-Aws-Request-Id")
	if requestID == "" {
		return nil, "", fmt.Errorf("lambda runtime omitted request id")
	}
	return body, requestID, nil
}

func handleInvocation(handler Handler, payload []byte) (apiGatewayV2Response, requestSummary, error) {
	var event apiGatewayV2Event
	if err := json.Unmarshal(payload, &event); err != nil {
		return apiGatewayV2Response{}, requestSummary{}, fmt.Errorf("decode API Gateway event: %w", err)
	}

	method := event.RequestContext.HTTP.Method
	path := event.RawPath
	if path == "" {
		path = event.RequestContext.HTTP.Path
	}
	if method == "" {
		method = http.MethodGet
	}
	if path == "" {
		path = "/"
	}

	body := []byte(event.Body)
	if event.IsBase64Encoded {
		decoded, err := base64.StdEncoding.DecodeString(event.Body)
		if err != nil {
			return apiGatewayV2Response{}, requestSummary{}, fmt.Errorf("decode base64 body: %w", err)
		}
		body = decoded
	}

	start := time.Now()
	appResponse := handler.Serve(method, path, event.Headers, body)
	headers := appResponse.Headers
	if headers == nil {
		headers = map[string]string{}
	}
	if event.RequestContext.RequestID != "" && headers["X-Request-Id"] == "" {
		headers["X-Request-Id"] = event.RequestContext.RequestID
	}

	return apiGatewayV2Response{
		StatusCode:      appResponse.StatusCode,
		Headers:         headers,
		Body:            string(appResponse.Body),
		IsBase64Encoded: false,
	}, requestSummary{
		Method:     method,
		Path:       path,
		RequestID:  event.RequestContext.RequestID,
		StatusCode: appResponse.StatusCode,
		Duration:   time.Since(start),
	}, nil
}

func postInvocationResponse(
	ctx context.Context,
	client *http.Client,
	endpoint string,
	requestID string,
	response apiGatewayV2Response,
) error {
	payload, err := json.Marshal(response)
	if err != nil {
		return err
	}

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		endpoint+"/"+requestID+"/response",
		bytes.NewReader(payload),
	)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")

	httpResponse, err := client.Do(request)
	if err != nil {
		return err
	}
	defer httpResponse.Body.Close()

	body, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return err
	}
	if httpResponse.StatusCode/100 != 2 {
		return fmt.Errorf("lambda runtime response returned %d: %s", httpResponse.StatusCode, string(body))
	}
	return nil
}

func postInvocationError(ctx context.Context, client *http.Client, endpoint string, requestID string, err error) error {
	payload, marshalErr := json.Marshal(invocationError{
		ErrorMessage: err.Error(),
		ErrorType:    "InvocationError",
	})
	if marshalErr != nil {
		return marshalErr
	}

	request, requestErr := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		endpoint+"/"+requestID+"/error",
		bytes.NewReader(payload),
	)
	if requestErr != nil {
		return requestErr
	}
	request.Header.Set("Content-Type", "application/json")

	response, requestErr := client.Do(request)
	if requestErr != nil {
		return requestErr
	}
	defer response.Body.Close()

	body, requestErr := io.ReadAll(response.Body)
	if requestErr != nil {
		return requestErr
	}
	if response.StatusCode/100 != 2 {
		return fmt.Errorf("lambda runtime error response returned %d: %s", response.StatusCode, string(body))
	}
	return nil
}
