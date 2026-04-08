package httpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChain_Order(t *testing.T) {
	var order []string

	mw := func(name string) Middleware {
		return func(next http.RoundTripper) http.RoundTripper {
			return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				order = append(order, name+"-before")
				resp, err := next.RoundTrip(req)
				order = append(order, name+"-after")
				return resp, err
			})
		}
	}

	base := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		order = append(order, "base")
		return &http.Response{StatusCode: 200}, nil
	})

	chained := chain(base, []Middleware{mw("first"), mw("second")})

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	_, _ = chained.RoundTrip(req)

	expected := []string{"first-before", "second-before", "base", "second-after", "first-after"}
	assert.Equal(t, len(expected), len(order))
	for i, v := range expected {
		assert.Equal(t, v, order[i])
	}
}

func TestChain_Empty(t *testing.T) {
	called := false
	base := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		called = true
		return &http.Response{StatusCode: 200}, nil
	})

	chained := chain(base, nil)
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	_, _ = chained.RoundTrip(req)
	assert.True(t, called, "empty chain should pass through to base transport")
}

func TestRoundTripperFunc(t *testing.T) {
	called := false
	rt := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		called = true
		return &http.Response{StatusCode: 201}, nil
	})

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	resp, err := rt.RoundTrip(req)
	require.NoError(t, err)
	assert.True(t, called, "function not called")
	assert.Equal(t, 201, resp.StatusCode)
}

func TestDo_WithMiddleware_AddsHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "injected", r.Header.Get("X-Custom"))
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"data": "ok"}`))
	}))
	defer srv.Close()

	addHeader := func(next http.RoundTripper) http.RoundTripper {
		return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			req.Header.Set("X-Custom", "injected")
			return next.RoundTrip(req)
		})
	}

	_, err := Do(context.Background(), Request{
		Method:      "GET",
		URL:         srv.URL,
		APIKey:      "test",
		Middlewares: []Middleware{addHeader},
	})
	require.NoError(t, err)
}

func TestDo_WithMultipleMiddlewares(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "1", r.Header.Get("X-First"))
		assert.Equal(t, "2", r.Header.Get("X-Second"))
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"data": "ok"}`))
	}))
	defer srv.Close()

	mw := func(key, val string) Middleware {
		return func(next http.RoundTripper) http.RoundTripper {
			return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				req.Header.Set(key, val)
				return next.RoundTrip(req)
			})
		}
	}

	_, err := Do(context.Background(), Request{
		Method:      "GET",
		URL:         srv.URL,
		APIKey:      "test",
		Middlewares: []Middleware{mw("X-First", "1"), mw("X-Second", "2")},
	})
	require.NoError(t, err)
}

func TestDo_WithoutMiddleware_Unchanged(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"data": "ok"}`))
	}))
	defer srv.Close()

	resp, err := Do(context.Background(), Request{
		Method: "GET",
		URL:    srv.URL,
		APIKey: "test",
	})
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}
