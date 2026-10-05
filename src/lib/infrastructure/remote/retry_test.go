package remote

import (
	"context"
	contract "dockpipe/src/lib/domain/remote"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestRetryPolicyStopsPermanentErrorsAndBoundsTransientErrors(t *testing.T) {
	for _, status := range []int{401, 403, 409, 429, 503} {
		calls := 0
		rejection := &HTTPError{StatusCode: status}
		var delays []time.Duration
		err := retryCall(context.Background(), func() error {
			calls++
			return rejection
		}, func(_ context.Context, delay time.Duration) bool {
			delays = append(delays, delay)
			return true
		})
		want := 1
		if status == http.StatusTooManyRequests || status == http.StatusServiceUnavailable {
			want = 5
		}
		if calls != want || len(delays) != want-1 || !errors.Is(err, rejection) {
			t.Fatalf("HTTP %d: calls=%d delays=%v error=%v", status, calls, delays, err)
		}
	}
}

func TestRetryHonorsCancellationAndRecovery(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	err := retryCall(ctx, func() error {
		calls++
		return &HTTPError{StatusCode: 503}
	}, func(ctx context.Context, _ time.Duration) bool {
		cancel()
		return false
	})
	if !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatalf("cancel = %v, attempts = %d", err, calls)
	}
	calls = 0
	err = retryCall(context.Background(), func() error {
		calls++
		if calls == 1 {
			return &HTTPError{StatusCode: 429}
		}
		return nil
	}, func(context.Context, time.Duration) bool { return true })
	if err != nil || calls != 2 {
		t.Fatalf("recovery = %v, attempts = %d", err, calls)
	}
}

func TestTerminalFailureCauseSurvivesFullWorkflowLog(t *testing.T) {
	log := &boundedLog{}
	if _, err := log.Write([]byte(strings.Repeat("x", contract.MaxLog))); err != nil {
		t.Fatal(err)
	}
	log.appendDiagnostic("broker heartbeat failed")
	if len(log.data) != contract.MaxLog || !log.truncated || !strings.HasSuffix(string(log.data), "broker heartbeat failed\n") {
		t.Fatal("bounded workflow log lost its terminal failure cause")
	}
}
