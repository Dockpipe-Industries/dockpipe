package remote

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"testing/synctest"
	"time"
)

func TestWorkflowHeartbeatRecoversWithinDeadline(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		calls := 0
		client := &Client{
			Endpoint: "http://broker.test",
			HTTP: &http.Client{Transport: machineTestTransport(func(request *http.Request) (*http.Response, error) {
				calls++
				if request.URL.Path != "/v1/pulse" {
					t.Fatalf("unexpected path: %s", request.URL.Path)
				}
				if calls == 1 {
					<-request.Context().Done()
					return nil, request.Context().Err()
				}

				response := recordedMachineResponse{status: http.StatusOK, body: `{"cancel":true}`}
				return response.response(request), nil
			})},
		}

		cancelled, err := pollWorkflowCancellation(context.Background(), client, "job", "session")
		if err != nil || !cancelled || calls != 2 {
			t.Fatalf("cancelled=%t calls=%d error=%v", cancelled, calls, err)
		}
	})
}

func TestWorkflowHeartbeatDeadlineBoundsRetries(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		calls := 0
		client := &Client{
			Endpoint: "http://broker.test",
			HTTP: &http.Client{Transport: machineTestTransport(func(request *http.Request) (*http.Response, error) {
				calls++
				<-request.Context().Done()
				return nil, request.Context().Err()
			})},
		}

		started := time.Now()
		cancelled, err := pollWorkflowCancellation(context.Background(), client, "job", "session")
		if cancelled || !errors.Is(err, context.DeadlineExceeded) || calls != 3 || time.Since(started) != 20*time.Second {
			t.Fatalf("cancelled=%t calls=%d elapsed=%s error=%v", cancelled, calls, time.Since(started), err)
		}
	})
}

func TestWorkflowHeartbeatRecoversFromConnectionClosedBeforeResponse(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		calls := 0
		client := &Client{
			Endpoint: "http://broker.test",
			HTTP: &http.Client{Transport: machineTestTransport(func(request *http.Request) (*http.Response, error) {
				calls++
				if calls == 1 {
					return nil, io.EOF
				}

				response := recordedMachineResponse{status: http.StatusOK, body: `{"cancel":false}`}
				return response.response(request), nil
			})},
		}

		cancelled, err := pollWorkflowCancellation(context.Background(), client, "job", "session")
		if err != nil || cancelled || calls != 2 {
			t.Fatalf("cancelled=%t calls=%d error=%v", cancelled, calls, err)
		}
	})
}

func TestWorkflowHeartbeatStopsOnPermanentFailure(t *testing.T) {
	for _, fixture := range []struct {
		name     string
		response recordedMachineResponse
		detail   string
	}{
		{"authorization rejected", recordedMachineResponse{status: http.StatusForbidden, body: "private"}, "HTTP 403"},
		{"malformed response", recordedMachineResponse{status: http.StatusOK, body: `{"private":true}`}, "invalid or unverified response"},
		{"missing cancellation state", recordedMachineResponse{status: http.StatusOK, body: "{}"}, "invalid or unverified response"},
		{"empty successful response", recordedMachineResponse{status: http.StatusOK, body: ""}, "invalid or unverified response"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			calls := 0
			client := &Client{
				Endpoint: "http://broker.test",
				HTTP: &http.Client{Transport: machineTestTransport(func(request *http.Request) (*http.Response, error) {
					calls++
					return fixture.response.response(request), nil
				})},
			}

			cancelled, err := pollWorkflowCancellation(context.Background(), client, "job", "session")
			if cancelled || err == nil || calls != 1 {
				t.Fatalf("cancelled=%t calls=%d error=%v", cancelled, calls, err)
			}

			detail := workflowHeartbeatFailure(err).Error()
			if !strings.HasSuffix(detail, fixture.detail) || strings.Contains(detail, "private") {
				t.Fatalf("unexpected diagnostic: %s", detail)
			}
		})
	}
}
