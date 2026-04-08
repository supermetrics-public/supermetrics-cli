package httpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDo_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Accept"))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	resp, err := Do(context.Background(), Request{
		Method: "GET",
		URL:    srv.URL,
		APIKey: "test-key",
	})
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	result, err := resp.ParseJSON()
	require.NoError(t, err)
	m, ok := result.(map[string]any)
	require.True(t, ok, "expected map[string]any")
	assert.Equal(t, "ok", m["status"])
}

func TestDo_AuthError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, err := Do(context.Background(), Request{
		Method: "GET",
		URL:    srv.URL,
		APIKey: "bad-key",
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "authentication failed")
}

func TestDo_RateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	_, err := Do(context.Background(), Request{
		Method: "GET",
		URL:    srv.URL,
		APIKey: "key",
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "rate limit")
}

func TestDo_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	_, err := Do(context.Background(), Request{
		Method:  "GET",
		URL:     srv.URL,
		APIKey:  "key",
		Timeout: 50 * time.Millisecond,
	})
	require.Error(t, err)
}

func TestDo_PostWithBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	_, err := Do(context.Background(), Request{
		Method: "POST",
		URL:    srv.URL,
		Body:   strings.NewReader(`{"key":"value"}`),
		APIKey: "key",
	})
	require.NoError(t, err)
}

func TestDo_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	_, err := Do(context.Background(), Request{
		Method: "GET",
		URL:    srv.URL,
		APIKey: "key",
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "500")
}

func TestDo_RequestIDExtracted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"meta":{"request_id":"abc123"},"data":{"key":"val"}}`))
	}))
	defer srv.Close()

	resp, err := Do(context.Background(), Request{
		Method: "GET",
		URL:    srv.URL,
		APIKey: "key",
	})
	require.NoError(t, err)
	assert.Equal(t, "abc123", resp.RequestID)
}

func TestParseJSON_EnvelopeWithDataObject(t *testing.T) {
	resp := &Response{
		Body: []byte(`{"meta":{"request_id":"req1"},"data":{"name":"test"}}`),
	}
	result, err := resp.ParseJSON()
	require.NoError(t, err)
	m, ok := result.(map[string]any)
	require.True(t, ok, "expected map[string]any")
	assert.Equal(t, "test", m["name"])
}

func TestParseJSON_EnvelopeWithDataArray(t *testing.T) {
	resp := &Response{
		Body: []byte(`{"meta":{"request_id":"req2"},"data":[{"id":1},{"id":2}]}`),
	}
	result, err := resp.ParseJSON()
	require.NoError(t, err)
	arr, ok := result.([]any)
	require.True(t, ok, "expected []any")
	assert.Equal(t, 2, len(arr))
}

func TestParseJSON_EnvelopeError(t *testing.T) {
	resp := &Response{
		Body:      []byte(`{"meta":{"request_id":"req3"},"error":{"code":"INVALID","message":"Bad input","description":"Field X is required"}}`),
		RequestID: "req3",
	}
	_, err := resp.ParseJSON()
	require.Error(t, err)

	apiErr, ok := err.(*APIError)
	require.True(t, ok, "expected *APIError")
	assert.Equal(t, "Bad input", apiErr.Message)
	assert.Equal(t, "INVALID", apiErr.Code)
	assert.Equal(t, "req3", apiErr.RequestID)
	assert.Equal(t, "Field X is required", apiErr.Description)
}

func TestParseJSON_NoEnvelope(t *testing.T) {
	resp := &Response{
		Body: []byte(`{"status":"ok"}`),
	}
	result, err := resp.ParseJSON()
	require.NoError(t, err)
	m, ok := result.(map[string]any)
	require.True(t, ok, "expected map[string]any")
	assert.Equal(t, "ok", m["status"])
}

func TestParseJSONWithMeta_ReturnsMeta(t *testing.T) {
	resp := &Response{
		Body: []byte(`{"meta":{"request_id":"req1","status_code":"pending"},"data":{"name":"test"}}`),
	}
	data, meta, err := resp.ParseJSONWithMeta()
	require.NoError(t, err)
	m, ok := data.(map[string]any)
	require.True(t, ok, "expected map[string]any for data")
	assert.Equal(t, "test", m["name"])
	require.NotNil(t, meta)
	assert.Equal(t, "req1", meta["request_id"])
	assert.Equal(t, "pending", meta["status_code"])
}

func TestParseJSONWithMeta_CompletedWithData(t *testing.T) {
	resp := &Response{
		Body: []byte(`{"meta":{"request_id":"req2","status_code":"completed"},"data":[["a","b"],["c","d"]]}`),
	}
	data, meta, err := resp.ParseJSONWithMeta()
	require.NoError(t, err)
	arr, ok := data.([]any)
	require.True(t, ok, "expected []any for data")
	assert.Equal(t, 2, len(arr))
	assert.Equal(t, "completed", meta["status_code"])
}

func TestParseJSONWithMeta_NoMeta(t *testing.T) {
	resp := &Response{
		Body: []byte(`{"data":{"name":"test"}}`),
	}
	data, meta, err := resp.ParseJSONWithMeta()
	require.NoError(t, err)
	require.NotNil(t, data)
	assert.Nil(t, meta)
}

func TestParseJSONWithMeta_Error(t *testing.T) {
	resp := &Response{
		Body:      []byte(`{"meta":{"request_id":"req3"},"error":{"code":"INVALID","message":"Bad input"}}`),
		RequestID: "req3",
	}
	_, _, err := resp.ParseJSONWithMeta()
	require.Error(t, err)
	apiErr, ok := err.(*APIError)
	require.True(t, ok)
	assert.Equal(t, "Bad input", apiErr.Message)
}

func TestDo_HTTPErrorWithEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"meta":{"request_id":"req4"},"error":{"code":"VALIDATION","message":"Invalid parameter"}}`))
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
	assert.Equal(t, "Invalid parameter", apiErr.Message)
	assert.Equal(t, "VALIDATION", apiErr.Code)
	assert.Equal(t, "req4", apiErr.RequestID)
	assert.Equal(t, 400, apiErr.StatusCode)
}
