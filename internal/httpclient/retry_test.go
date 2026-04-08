package httpclient

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRetry_429SucceedsOnSecondAttempt(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(429)
			_, _ = w.Write([]byte(`{"error":{"message":"rate limited"}}`))
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"data":"ok"}`))
	}))
	defer srv.Close()

	resp, err := Do(context.Background(), Request{
		Method:       "GET",
		URL:          srv.URL,
		APIKey:       "test",
		DisableRetry: false,
	})
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, 2, attempts)
}

func TestRetry_500SucceedsOnThirdAttempt(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(500)
			_, _ = w.Write([]byte(`{"error":{"message":"internal error"}}`))
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"data":"ok"}`))
	}))
	defer srv.Close()

	resp, err := Do(context.Background(), Request{
		Method: "GET",
		URL:    srv.URL,
		APIKey: "test",
	})
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, 3, attempts)
}

func TestRetry_NoRetryOnClientErrors(t *testing.T) {
	for _, code := range []int{400, 401, 403, 404} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			attempts := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				attempts++
				w.WriteHeader(code)
				_, _ = w.Write([]byte(`{"error":{"message":"client error"}}`))
			}))
			defer srv.Close()

			_, _ = Do(context.Background(), Request{
				Method: "GET",
				URL:    srv.URL,
				APIKey: "test",
			})
			assert.Equal(t, 1, attempts)
		})
	}
}

func TestRetry_RespectsRetryAfterSeconds(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(429)
			_, _ = w.Write([]byte(`{}`))
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"data":"ok"}`))
	}))
	defer srv.Close()

	start := time.Now()
	resp, err := Do(context.Background(), Request{
		Method: "GET",
		URL:    srv.URL,
		APIKey: "test",
	})
	elapsed := time.Since(start)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	// Should have waited ~1s from Retry-After header.
	assert.GreaterOrEqual(t, elapsed, 900*time.Millisecond)
}

func TestRetry_RespectsRetryAfterHTTPDate(t *testing.T) {
	// Verify parseRetryAfter handles HTTP-date format correctly.
	futureTime := time.Now().Add(2 * time.Second).UTC().Format(time.RFC1123)
	d := parseRetryAfter(futureTime)
	assert.Greater(t, d, 1*time.Second)

	pastTime := time.Now().Add(-1 * time.Second).UTC().Format(time.RFC1123)
	d = parseRetryAfter(pastTime)
	assert.Equal(t, time.Duration(0), d)
}

func TestRetry_CapsRetryAfterAtMaxBackoff(t *testing.T) {
	cfg := RetryConfig{MaxAttempts: 2, BaseDelay: 10 * time.Millisecond, MaxBackoff: 50 * time.Millisecond}

	resp := &http.Response{
		StatusCode: 429,
		Header:     http.Header{"Retry-After": []string{"300"}},
	}
	delay := retryDelay(resp, 0, cfg)
	assert.LessOrEqual(t, delay, cfg.MaxBackoff)
}

func TestRetry_StopsAfterMaxAttempts(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`{"error":{"message":"always fails"}}`))
	}))
	defer srv.Close()

	_, err := Do(context.Background(), Request{
		Method: "GET",
		URL:    srv.URL,
		APIKey: "test",
	})
	// Should get error after 3 attempts (default MaxAttempts).
	require.Error(t, err)
	assert.Equal(t, 3, attempts)
}

func TestRetry_ContextCanceled_NoRetry(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	_, err := Do(ctx, Request{
		Method: "GET",
		URL:    "http://localhost:1", // Won't connect.
		APIKey: "test",
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled))
}

func TestRetry_DisableRetry(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`{"error":{"message":"fail"}}`))
	}))
	defer srv.Close()

	_, _ = Do(context.Background(), Request{
		Method:       "GET",
		URL:          srv.URL,
		APIKey:       "test",
		DisableRetry: true,
	})
	assert.Equal(t, 1, attempts)
}

func TestRetry_NetworkError_Retries(t *testing.T) {
	attempts := 0
	// Use a middleware to simulate network error on first attempt.
	failOnce := func(next http.RoundTripper) http.RoundTripper {
		return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			attempts++
			if attempts == 1 {
				return nil, errors.New("connection refused")
			}
			return next.RoundTrip(req)
		})
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"data":"ok"}`))
	}))
	defer srv.Close()

	resp, err := Do(context.Background(), Request{
		Method:      "GET",
		URL:         srv.URL,
		APIKey:      "test",
		Middlewares: []Middleware{failOnce},
	})
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, 2, attempts)
}

func TestRetry_PostWithBody(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		body, _ := io.ReadAll(r.Body)
		assert.Equal(t, `{"key":"value"}`, string(body))
		if attempts == 1 {
			w.WriteHeader(502)
			_, _ = w.Write([]byte(`{}`))
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"data":"ok"}`))
	}))
	defer srv.Close()

	bodyBytes := []byte(`{"key":"value"}`)
	resp, err := Do(context.Background(), Request{
		Method: "POST",
		URL:    srv.URL,
		Body:   bytes.NewReader(bodyBytes),
		APIKey: "test",
	})
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, 2, attempts)
}

func TestRetry_VerboseLogsRetries(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(503)
			_, _ = w.Write([]byte(`{}`))
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"data":"ok"}`))
	}))
	defer srv.Close()

	// Capture stderr.
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	_, err := Do(context.Background(), Request{
		Method:  "GET",
		URL:     srv.URL,
		APIKey:  "test",
		Verbose: true,
	})

	_ = w.Close()
	os.Stderr = oldStderr

	require.NoError(t, err)

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	assert.Contains(t, output, "Retry 1/2")
	assert.Contains(t, output, "503")
}

func TestBackoff_CappedAtMax(t *testing.T) {
	base := 100 * time.Millisecond
	max := 500 * time.Millisecond

	// attempt=10 → base * 2^10 = 102.4s, well above max
	for range 50 {
		d := backoff(10, base, max)
		assert.LessOrEqual(t, d, max, "backoff should never exceed max")
		assert.GreaterOrEqual(t, d, time.Duration(0), "backoff should be non-negative")
	}
}

func TestBackoff_BelowCap(t *testing.T) {
	base := 100 * time.Millisecond
	max := 10 * time.Second

	// attempt=0 → base * 2^0 = 100ms, well below max
	for range 50 {
		d := backoff(0, base, max)
		assert.LessOrEqual(t, d, base, "backoff at attempt 0 should be at most base")
	}
}

func TestRetry_502And504AreRetryable(t *testing.T) {
	for _, code := range []int{502, 504} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			attempts := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				attempts++
				if attempts == 1 {
					w.WriteHeader(code)
					_, _ = w.Write([]byte(`{}`))
					return
				}
				w.WriteHeader(200)
				_, _ = w.Write([]byte(`{"data":"ok"}`))
			}))
			defer srv.Close()

			resp, err := Do(context.Background(), Request{
				Method: "GET",
				URL:    srv.URL,
				APIKey: "test",
			})
			require.NoError(t, err)
			assert.Equal(t, 200, resp.StatusCode)
			assert.Equal(t, 2, attempts)
		})
	}
}
