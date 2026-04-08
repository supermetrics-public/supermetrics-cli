package httpclient

import "net/http"

// Middleware wraps an http.RoundTripper to add transport-level behavior
// such as retry, rate limiting, or request tracing.
type Middleware func(http.RoundTripper) http.RoundTripper

// roundTripperFunc adapts a function to the http.RoundTripper interface.
type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// chain applies middlewares in order: first middleware is outermost (runs first).
func chain(transport http.RoundTripper, middlewares []Middleware) http.RoundTripper {
	for i := len(middlewares) - 1; i >= 0; i-- {
		transport = middlewares[i](transport)
	}
	return transport
}
