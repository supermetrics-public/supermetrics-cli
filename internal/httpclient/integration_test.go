package httpclient

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/supermetrics-public/supermetrics-cli/internal/output"
)

// TestIntegration_SuccessEnvelopeToJSON tests the full flow from HTTP request
// through envelope unwrapping to JSON output.
func TestIntegration_SuccessEnvelopeToJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		w.Write([]byte(`{
			"meta": {"request_id": "req123"},
			"data": [
				{"id": "1", "name": "Alice"},
				{"id": "2", "name": "Bob"}
			]
		}`))
	}))
	defer srv.Close()

	resp, err := Do(context.Background(), Request{
		Method: "GET",
		URL:    srv.URL,
		APIKey: "test-key",
	})
	require.NoError(t, err)

	assert.Equal(t, "req123", resp.RequestID)

	result, err := resp.ParseJSON()
	require.NoError(t, err)

	// Verify data was unwrapped (should be array, not envelope)
	arr, ok := result.([]any)
	require.True(t, ok, "expected []any")
	assert.Equal(t, 2, len(arr))

	// Render as JSON
	var buf bytes.Buffer
	err = output.Print(&buf, result, output.PrintOptions{Format: "json"})
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "Alice")
	assert.Contains(t, out, "Bob")
	// Envelope fields should NOT be in output
	assert.NotContains(t, out, "request_id")
	assert.NotContains(t, out, "meta")
}

// TestIntegration_SuccessEnvelopeToTable tests envelope unwrapping to table output.
func TestIntegration_SuccessEnvelopeToTable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{
			"meta": {"request_id": "req456"},
			"data": [
				{"account_id": "123", "account_name": "Test Account"},
				{"account_id": "456", "account_name": "Other Account"}
			]
		}`))
	}))
	defer srv.Close()

	resp, err := Do(context.Background(), Request{
		Method: "GET",
		URL:    srv.URL,
		APIKey: "key",
	})
	require.NoError(t, err)

	result, err := resp.ParseJSON()
	require.NoError(t, err)

	var buf bytes.Buffer
	err = output.Print(&buf, result, output.PrintOptions{Format: "table"})
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "ACCOUNT_ID")
	assert.Contains(t, out, "Test Account")
}

// TestIntegration_EnvelopeErrorReturnsError tests that an API-level error
// in the envelope is surfaced as a Go error.
func TestIntegration_EnvelopeErrorReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"meta": {"request_id": "req789"},
			"error": {
				"code": "INVALID_PARAM",
				"message": "ds_id is required",
				"description": "The ds_id parameter must be provided"
			}
		}`))
	}))
	defer srv.Close()

	resp, err := Do(context.Background(), Request{
		Method: "GET",
		URL:    srv.URL,
		APIKey: "key",
	})
	require.NoError(t, err)

	_, err = resp.ParseJSON()
	require.Error(t, err)

	apiErr, ok := err.(*APIError)
	require.True(t, ok, "expected *APIError")
	assert.Equal(t, "ds_id is required", apiErr.Message)
	assert.Equal(t, "INVALID_PARAM", apiErr.Code)
	assert.Equal(t, "req789", apiErr.RequestID)
}

// TestIntegration_HTTPErrorWithEnvelope tests that HTTP errors with structured
// envelope bodies produce clean error messages.
func TestIntegration_HTTPErrorWithEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{
			"meta": {"request_id": "reqForbidden"},
			"error": {
				"code": "FORBIDDEN",
				"message": "API key does not have access to this resource"
			}
		}`))
	}))
	defer srv.Close()

	_, err := Do(context.Background(), Request{
		Method: "GET",
		URL:    srv.URL,
		APIKey: "key",
	})
	require.Error(t, err)

	apiErr, ok := err.(*APIError)
	require.True(t, ok, "expected *APIError")
	assert.Equal(t, "API key does not have access to this resource", apiErr.Message)
	assert.Equal(t, "reqForbidden", apiErr.RequestID)
	assert.Equal(t, 403, apiErr.StatusCode)
}

// TestIntegration_NestedDataInTable tests that nested arrays in table output
// show counts instead of raw Go syntax.
func TestIntegration_NestedDataInTable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{
			"meta": {"request_id": "reqNested"},
			"data": [
				{
					"ds_user": "alice@example.com",
					"accounts": [{"id": "1"}, {"id": "2"}, {"id": "3"}]
				}
			]
		}`))
	}))
	defer srv.Close()

	resp, err := Do(context.Background(), Request{
		Method: "GET",
		URL:    srv.URL,
		APIKey: "key",
	})
	require.NoError(t, err)

	result, err := resp.ParseJSON()
	require.NoError(t, err)

	var buf bytes.Buffer
	err = output.Print(&buf, result, output.PrintOptions{Format: "table"})
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "3 items")
	assert.NotContains(t, out, "map[")
}
