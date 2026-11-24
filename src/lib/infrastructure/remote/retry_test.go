package remote

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	contract "dockpipe/src/lib/domain/remote"
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

func TestWorkerWaitingForAssignmentsReconnectsAcrossRetryBudgets(t *testing.T) {
	polls := 0
	waits := 0
	err := waitForWorkerConnection(context.Background(), func() error {
		polls++
		if polls <= 3 {
			return fmt.Errorf("request retry budget: %w", &HTTPError{StatusCode: http.StatusServiceUnavailable})
		}
		return nil
	}, func(_ context.Context, delay time.Duration) bool {
		if delay < 10*time.Second || delay > time.Minute {
			t.Fatalf("unexpected reconnect delay %s", delay)
		}
		waits++
		return true
	})
	if err != nil || polls != 4 || waits != 3 {
		t.Fatalf("error=%v polls=%d waits=%d", err, polls, waits)
	}
}

func TestWorkerReconnectStopsOnRejectionOrCancellation(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, http.StatusServiceUnavailable} {
		ctx, cancel := context.WithCancel(context.Background())
		failure := &HTTPError{StatusCode: status}
		var expected error = failure
		if status == http.StatusServiceUnavailable {
			expected = context.Canceled
		}

		polls := 0
		err := waitForWorkerConnection(ctx, func() error {
			polls++
			return failure
		}, func(_ context.Context, _ time.Duration) bool {
			if status == http.StatusForbidden {
				t.Error("worker retried an authorization rejection")
			}
			cancel()
			return false
		})
		cancel()
		if !errors.Is(err, expected) || polls != 1 {
			t.Errorf("HTTP %d: error=%v polls=%d", status, err, polls)
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
