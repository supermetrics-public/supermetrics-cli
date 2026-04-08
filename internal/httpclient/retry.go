package httpclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"net/http"
	"os"
	"strconv"
	"time"
)

// RetryConfig controls the retry middleware behavior.
type RetryConfig struct {
	MaxAttempts int           // total attempts including initial (default 3)
	BaseDelay   time.Duration // initial backoff delay (default 500ms)
	MaxBackoff  time.Duration // maximum backoff delay (default 20s)
	Verbose     bool          // log retry attempts to stderr
}

func (c RetryConfig) withDefaults() RetryConfig {
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = 3
	}
	if c.BaseDelay <= 0 {
		c.BaseDelay = 500 * time.Millisecond
	}
	if c.MaxBackoff <= 0 {
		c.MaxBackoff = 20 * time.Second
	}
	return c
}

// Retry returns a Middleware that retries transient failures with exponential
// backoff and full jitter. Retryable: 429, 500, 502, 503, 504, and network
// errors (excluding context cancellation/deadline). Respects Retry-After header.
func Retry(cfg RetryConfig) Middleware {
	cfg = cfg.withDefaults()

	return func(next http.RoundTripper) http.RoundTripper {
		return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			var resp *http.Response
			var err error

			for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
				if attempt > 0 {
					// Reset body for retry if possible.
					if req.GetBody != nil {
						req.Body, err = req.GetBody()
						if err != nil {
							return nil, fmt.Errorf("failed to reset request body for retry: %w", err)
						}
					}
				}

				resp, err = next.RoundTrip(req)

				if err != nil {
					if !isRetryableError(err) {
						return nil, err
					}
					if attempt < cfg.MaxAttempts-1 {
						delay := backoff(attempt, cfg.BaseDelay, cfg.MaxBackoff)
						if cfg.Verbose {
							fmt.Fprintf(os.Stderr, "> Retry %d/%d: %v (waiting %s)\n",
								attempt+1, cfg.MaxAttempts-1, err, delay.Round(time.Millisecond))
						}
						if err := sleep(req.Context(), delay); err != nil {
							return nil, err
						}
					}
					continue
				}

				if !isRetryableStatus(resp.StatusCode) {
					return resp, nil
				}

				if attempt < cfg.MaxAttempts-1 {
					delay := retryDelay(resp, attempt, cfg)
					if cfg.Verbose {
						fmt.Fprintf(os.Stderr, "> Retry %d/%d: %d %s (waiting %s)\n",
							attempt+1, cfg.MaxAttempts-1, resp.StatusCode, http.StatusText(resp.StatusCode),
							delay.Round(time.Millisecond))
					}
					// Drain and close body before retry.
					_, _ = io.Copy(io.Discard, resp.Body)
					_ = resp.Body.Close()

					if err := sleep(req.Context(), delay); err != nil {
						return nil, err
					}
				}
			}

			return resp, err
		})
	}
}

func isRetryableStatus(code int) bool {
	switch code {
	case 429, 500, 502, 503, 504:
		return true
	}
	return false
}

func isRetryableError(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	return true
}

// retryDelay returns the delay before the next retry attempt. Uses Retry-After
// header if present on 429, otherwise falls back to exponential backoff.
func retryDelay(resp *http.Response, attempt int, cfg RetryConfig) time.Duration {
	if resp.StatusCode == http.StatusTooManyRequests {
		if d := parseRetryAfter(resp.Header.Get("Retry-After")); d > 0 {
			if d > cfg.MaxBackoff {
				d = cfg.MaxBackoff
			}
			return d
		}
	}
	return backoff(attempt, cfg.BaseDelay, cfg.MaxBackoff)
}

// backoff calculates delay with full jitter: random in [0, min(base*2^attempt, max)].
func backoff(attempt int, base, max time.Duration) time.Duration {
	delay := time.Duration(float64(base) * math.Pow(2, float64(attempt)))
	if delay > max {
		delay = max
	}
	return time.Duration(rand.Int64N(int64(delay) + 1)) //nolint:gosec // jitter doesn't need crypto randomness
}

// parseRetryAfter parses the Retry-After header value as seconds (integer)
// or HTTP-date (RFC1123). Returns 0 if unparseable.
func parseRetryAfter(val string) time.Duration {
	if val == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(val); err == nil {
		return time.Duration(seconds) * time.Second
	}
	if t, err := time.Parse(time.RFC1123, val); err == nil {
		d := time.Until(t)
		if d > 0 {
			return d
		}
	}
	return 0
}

// sleep waits for the given duration or until the context is cancelled.
func sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
