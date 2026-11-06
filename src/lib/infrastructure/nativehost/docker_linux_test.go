package nativehost

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type dockerTransport func(*http.Request) (*http.Response, error)

func (transport dockerTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func TestDockerActionUsesOnlyValidatedOperation(t *testing.T) {
	for _, action := range []string{"start", "stop"} {
		calls := 0
		client := &http.Client{Transport: dockerTransport(func(request *http.Request) (*http.Response, error) {
			calls++
			if calls == 1 {
				if request.Method != http.MethodGet || request.URL.Path != "/version" {
					t.Fatalf("unexpected version probe: %s", request.URL)
				}
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"ApiVersion":"1.41"}`))}, nil
			}
			if request.Method != http.MethodPost || request.URL.Path != "/v1.41/containers/abcdef123456/"+action || request.Body != nil {
				t.Fatalf("unexpected Docker operation: %+v", request)
			}
			if action == "stop" && request.URL.RawQuery != "t=5" {
				t.Fatal("stop was not bounded")
			}
			return &http.Response{StatusCode: http.StatusNotModified, Body: io.NopCloser(strings.NewReader(""))}, nil
		})}
		result := dockerAction(context.Background(), client, action, "abcdef123456")
		if result.Outcome != "Success" || calls != 2 {
			t.Fatalf("Docker action failed: %+v", result)
		}
	}
}

func TestDockerActionFailsClosedAndBoundsResponse(t *testing.T) {
	for _, body := range []string{`{"ApiVersion":"../evil"}`, strings.Repeat("x", 16385)} {
		client := &http.Client{Transport: dockerTransport(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
		})}
		result := dockerAction(context.Background(), client, "start", "abcdef123456")
		if result.Outcome != "Failure" || result.Reason != "docker_unavailable" {
			t.Fatalf("invalid daemon response accepted: %+v", result)
		}
	}
	for _, id := range []string{"", "abcdef", "container-name", "abcdef123456/../other", "--help", strings.Repeat("a", 65)} {
		if validDockerAction("start", id) {
			t.Fatalf("invalid ID accepted: %q", id)
		}
	}
	if validDockerAction("exec", "abcdef123456") {
		t.Fatal("arbitrary operation accepted")
	}
}

func TestDockerActionCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	client := &http.Client{Transport: dockerTransport(func(request *http.Request) (*http.Response, error) {
		cancel()
		<-request.Context().Done()
		return nil, request.Context().Err()
	})}
	result := dockerAction(ctx, client, "stop", "abcdef123456")
	if result.Outcome != "Cancelled" || result.Reason != "operation_cancelled" {
		t.Fatalf("cancellation lost: %+v", result)
	}
}
