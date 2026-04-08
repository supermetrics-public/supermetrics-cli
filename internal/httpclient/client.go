package httpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	DefaultTimeout = 30 * time.Second
	LongTimeout    = 60 * time.Minute
)

// Request holds the parameters for an API request.
type Request struct {
	Method       string
	URL          string
	Body         io.Reader
	APIKey       string
	Timeout      time.Duration
	Verbose      bool
	Client       *http.Client // optional; defaults to http.DefaultClient
	Middlewares  []Middleware // optional; applied in order (first is outermost)
	DisableRetry bool         // optional; disables automatic retry on transient errors
}

// Response holds the parsed API response.
type Response struct {
	StatusCode int
	Body       []byte
	RequestID  string
}

// Do executes an HTTP request with timeout, auth headers, verbose logging,
// and centralized error handling for Supermetrics API responses.
func Do(ctx context.Context, r Request) (*Response, error) {
	timeout := r.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, r.Method, r.URL, r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+r.APIKey)
	req.Header.Set("Accept", "application/json")
	if r.Body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if r.Verbose {
		fmt.Fprintf(os.Stderr, "> %s %s\n", req.Method, req.URL.String())
	}

	client := r.Client
	if client == nil {
		client = http.DefaultClient
	}
	middlewares := r.Middlewares
	if !r.DisableRetry {
		retryMW := Retry(RetryConfig{
			MaxAttempts: 3,
			BaseDelay:   500 * time.Millisecond,
			MaxBackoff:  20 * time.Second,
			Verbose:     r.Verbose,
		})
		middlewares = append([]Middleware{retryMW}, middlewares...)
	}
	if len(middlewares) > 0 {
		transport := client.Transport
		if transport == nil {
			transport = http.DefaultTransport
		}
		client = &http.Client{
			Transport:     chain(transport, middlewares),
			CheckRedirect: client.CheckRedirect,
			Jar:           client.Jar,
			Timeout:       client.Timeout,
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Extract request ID from envelope (best-effort).
	requestID := extractRequestID(body)

	if r.Verbose {
		if requestID != "" {
			fmt.Fprintf(os.Stderr, "< %d %s (request_id: %s)\n", resp.StatusCode, resp.Status, requestID)
		} else {
			fmt.Fprintf(os.Stderr, "< %d %s\n", resp.StatusCode, resp.Status)
		}
	}

	if resp.StatusCode >= 400 {
		return nil, apiError(resp.StatusCode, body, requestID)
	}

	return &Response{
		StatusCode: resp.StatusCode,
		Body:       body,
		RequestID:  requestID,
	}, nil
}

// ParseJSON unmarshals the response body, unwrapping the Supermetrics API
// envelope if present. It returns the "data" field contents for success
// responses, or an error for envelope-level errors.
//
// Responses without an envelope (no "data" or "error" key) are returned as-is.
func (r *Response) ParseJSON() (any, error) {
	data, _, err := r.parseEnvelope()
	return data, err
}

// ParseJSONWithMeta unmarshals the response body and returns the data payload
// alongside the raw envelope metadata. Callers can extract the specific meta
// fields they need (e.g. status_code for async polling, paginate for pagination).
func (r *Response) ParseJSONWithMeta() (data any, meta map[string]any, err error) {
	return r.parseEnvelope()
}

// parseEnvelope is the shared implementation for ParseJSON and ParseJSONWithMeta.
func (r *Response) parseEnvelope() (any, map[string]any, error) {
	var raw any
	if err := json.Unmarshal(r.Body, &raw); err != nil {
		return nil, nil, fmt.Errorf("failed to parse response: %w", err)
	}

	envelope, ok := raw.(map[string]any)
	if !ok {
		return raw, nil, nil
	}

	// Check for API-level error in the envelope.
	if errObj, ok := envelope["error"].(map[string]any); ok {
		return nil, nil, envelopeError(errObj, r.RequestID)
	}

	meta, _ := envelope["meta"].(map[string]any)

	// Unwrap the data field if present.
	if data, ok := envelope["data"]; ok {
		return data, meta, nil
	}

	// No envelope structure — return as-is.
	return envelope, meta, nil
}

// APIError is a structured error from the Supermetrics API.
// It carries enough context for rich error rendering.
type APIError struct {
	Message     string `json:"message"`
	Description string `json:"description,omitempty"`
	Code        string `json:"code,omitempty"`
	RequestID   string `json:"request_id,omitempty"`
	StatusCode  int    `json:"-"`
}

func (e *APIError) Error() string {
	return e.Message
}

func envelopeError(errObj map[string]any, requestID string) error {
	code, _ := errObj["code"].(string)
	message, _ := errObj["message"].(string)
	description, _ := errObj["description"].(string)

	if message == "" {
		message = "unknown API error"
	}

	return &APIError{
		Message:     message,
		Description: description,
		Code:        code,
		RequestID:   requestID,
	}
}

func apiError(statusCode int, body []byte, requestID string) error {
	// Try to extract a structured error from the response body.
	var envelope map[string]any
	if json.Unmarshal(body, &envelope) == nil {
		if errObj, ok := envelope["error"].(map[string]any); ok {
			e := envelopeError(errObj, requestID).(*APIError)
			e.StatusCode = statusCode
			return e
		}
	}

	var message string
	switch statusCode {
	case 401:
		message = "authentication failed. Check your API key"
	case 403:
		message = "insufficient permissions for this operation"
	case 404:
		message = "resource not found"
	case 429:
		message = "rate limit exceeded. Try again later"
	default:
		message = fmt.Sprintf("API error (HTTP %d)", statusCode)
	}

	return &APIError{
		Message:    message,
		RequestID:  requestID,
		StatusCode: statusCode,
	}
}

func extractRequestID(body []byte) string {
	var envelope struct {
		Meta struct {
			RequestID string `json:"request_id"`
		} `json:"meta"`
	}
	if json.Unmarshal(body, &envelope) == nil {
		return envelope.Meta.RequestID
	}
	return ""
}
