// Package retry provides exponential backoff with jitter and error classification
// for LLM API calls.
package retry

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"net"
	"strings"
	"time"
)

// Config holds retry parameters.
type Config struct {
	MaxAttempts int           // max attempts per operation (default: 3)
	BaseDelay   time.Duration // initial delay between retries (default: 1s)
	MaxDelay    time.Duration // cap on delay growth (default: 30s)
}

// IsRetryable returns true if the error is transient and worth retrying.
// Covers: network errors, timeouts, HTTP 429/5xx, context deadlines.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Network errors — DNS, connection refused, reset, EOF
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	// Context deadline / cancellation (wraps net.Error, but check explicitly)
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}

	// Check for HTTP status codes embedded in error messages (our LLM client format)
	msg := err.Error()
	if strings.Contains(msg, "status 429") || // rate limit
		strings.Contains(msg, "status 500") || // internal server error
		strings.Contains(msg, "status 502") || // bad gateway
		strings.Contains(msg, "status 503") || // service unavailable
		strings.Contains(msg, "status 504") { // gateway timeout
		return true
	}

	// io.EOF, io.ErrUnexpectedEOF from broken connections
	if strings.Contains(msg, "EOF") {
		return true
	}

	// Timeout detection in error message (net/http wraps timeouts)
	if strings.Contains(msg, "timeout") || strings.Contains(msg, "Timeout") {
		return true
	}

	// TLS / connection errors
	if strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "TLS") {
		return true
	}

	return false
}

// IsFatal returns true for errors that should immediately abort all retries:
// HTTP 400 (bad request), 401/403 (auth), 404 (not found).
func IsFatal(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "status 400") ||
		strings.Contains(msg, "status 401") ||
		strings.Contains(msg, "status 403") ||
		strings.Contains(msg, "status 404")
}

// Do executes fn with exponential backoff and full jitter.
// Returns the last error if all attempts fail, or nil on success.
//
// Backoff formula: min(MaxDelay, BaseDelay * 2^attempt) with full jitter [0, delay).
// Full jitter (not "decorrelated" or "equal") spreads retries evenly across the window,
// avoiding thundering herd while keeping worst-case latency bounded.
func Do(cfg Config, fn func() error) error {
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 3
	}
	if cfg.BaseDelay <= 0 {
		cfg.BaseDelay = 1 * time.Second
	}
	if cfg.MaxDelay <= 0 {
		cfg.MaxDelay = 30 * time.Second
	}

	var lastErr error
	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// Don't retry non-retryable errors
		if !IsRetryable(err) {
			return fmt.Errorf("non-retryable error (attempt %d/%d): %w", attempt+1, cfg.MaxAttempts, err)
		}

		// Don't wait after the last attempt
		if attempt == cfg.MaxAttempts-1 {
			break
		}

		// Exponential backoff with full jitter
		delay := time.Duration(float64(cfg.BaseDelay) * math.Pow(2, float64(attempt)))
		if delay > cfg.MaxDelay {
			delay = cfg.MaxDelay
		}
		// Full jitter: random in [0, delay)
		jittered := time.Duration(rand.Int64N(int64(delay)))
		time.Sleep(jittered)
	}

	return fmt.Errorf("all %d attempts failed: %w", cfg.MaxAttempts, lastErr)
}
