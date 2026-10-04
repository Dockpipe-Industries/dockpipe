package remote

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

// HTTPError deliberately excludes the server body: it may reflect credentials
// or contain provider HTML. Status still gives the operator actionable context.
type HTTPError struct{ StatusCode int }

func (err *HTTPError) Error() string {
	return fmt.Sprintf("remote operation rejected (HTTP %d)", err.StatusCode)
}

func retryable(err error) bool {
	var status *HTTPError
	if errors.As(err, &status) {
		return status.StatusCode == http.StatusTooManyRequests || status.StatusCode == http.StatusRequestTimeout || status.StatusCode >= 500 && status.StatusCode <= 599
	}
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	var dns *net.DNSError
	if errors.As(err, &dns) {
		return dns.IsTimeout || dns.IsTemporary
	}
	var operation *net.OpError
	if errors.As(err, &operation) {
		return true
	}
	var network net.Error
	return errors.As(err, &network) && network.Timeout()
}

func (c *Client) callWithRetry(ctx context.Context, path string, input, output any) error {
	return retryCall(ctx, func() error { return c.Call(ctx, path, input, output) }, pause)
}

func retryCall(ctx context.Context, call func() error, wait func(context.Context, time.Duration) bool) error {
	const attempts = 5
	for attempt := 0; attempt < attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := call()
		if err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !retryable(err) {
			return err
		}
		if attempt == attempts-1 {
			return fmt.Errorf("remote retry budget exhausted after %d attempts: %w", attempts, err)
		}
		if !wait(ctx, time.Second*time.Duration(1<<attempt)) {
			return ctx.Err()
		}
	}
	return nil
}
