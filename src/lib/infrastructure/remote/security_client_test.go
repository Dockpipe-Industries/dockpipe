package remote

import (
	"context"
	"errors"
	"net/http"
	"testing"

	contract "dockpipe/src/lib/domain/remote"
)

type countedOperatorTransport struct {
	http.RoundTripper
	requests int
}

func (transport *countedOperatorTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	transport.requests++
	return transport.RoundTripper.RoundTrip(request)
}

func TestLockedOperatorAuthenticatesBeforeSendingExactlyOnce(t *testing.T) {
	_, broker, client, _ := testBroker(t)
	authority := broker.security
	authority.Lock()
	signCalls, authentications := 0, 0
	client.protectLocalOperator(broker.state.Identity.PublicKey, func(ctx context.Context, request SecurityRequest) (SecurityResponse, error) {
		signCalls++
		proof, err := authority.Sign(ctx, request.Audience, request.Path, request.Digest)
		return SecurityResponse{Proof: &proof}, err
	})
	client.ReauthenticateOperator = func(ctx context.Context) error {
		authentications++
		return authority.Unlock(ctx, 1)
	}
	transport := &countedOperatorTransport{RoundTripper: client.HTTP.Transport}
	client.HTTP.Transport = transport
	if err := client.Call(context.Background(), "/v1/nodes", nil, nil); err != nil {
		t.Fatal(err)
	}
	if signCalls != 2 || authentications != 1 || transport.requests != 1 {
		t.Fatalf("sign=%d auth=%d HTTP=%d", signCalls, authentications, transport.requests)
	}
	// An unlocked second action needs neither a prompt nor a network retry.
	if err := client.Call(context.Background(), "/v1/nodes", nil, nil); err != nil {
		t.Fatal(err)
	}
	if authentications != 1 || transport.requests != 2 {
		t.Fatal("unlocked action prompted or replayed")
	}
}

func TestOperatorReauthenticationIsBoundedAndOnlyForTypedLocalLock(t *testing.T) {
	unavailable := errors.New("OS authority unavailable")
	for _, test := range []struct {
		name          string
		signError     error
		authError     error
		expectedSigns int
		expectedAuth  int
	}{
		{"cancelled", ErrOperatorLocked, context.Canceled, 1, 1},
		{"still locked", ErrOperatorLocked, nil, 2, 1},
		{"unavailable", unavailable, nil, 1, 0},
		{"text is not authority", errors.New("remote access is locked"), nil, 1, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := &Client{}
			signs, authentications := 0, 0
			client.protectLocalOperator("broker", func(context.Context, SecurityRequest) (SecurityResponse, error) {
				signs++
				return SecurityResponse{}, test.signError
			})
			client.ReauthenticateOperator = func(context.Context) error {
				authentications++
				return test.authError
			}
			if _, err := client.OperatorProof(context.Background(), "/v1/nodes", []byte("null")); err == nil {
				t.Fatal("failed authentication or signing succeeded")
			}
			if signs != test.expectedSigns || authentications != test.expectedAuth {
				t.Fatalf("sign=%d auth=%d", signs, authentications)
			}
		})
	}
}

func TestNonInteractiveOperatorNeverAuthenticates(t *testing.T) {
	client := &Client{}
	client.protectLocalOperator("broker", func(context.Context, SecurityRequest) (SecurityResponse, error) {
		return SecurityResponse{}, ErrOperatorLocked
	})
	if _, err := client.OperatorProof(context.Background(), "/v1/nodes", []byte("null")); !errors.Is(err, ErrOperatorLocked) {
		t.Fatal("noninteractive lock was not preserved")
	}
}

func TestOperatorNetworkFailureDoesNotReauthenticateOrReplay(t *testing.T) {
	_, broker, client, server := testBroker(t)
	client.protectLocalOperator(broker.state.Identity.PublicKey, func(ctx context.Context, request SecurityRequest) (SecurityResponse, error) {
		proof, err := broker.security.Sign(ctx, request.Audience, request.Path, request.Digest)
		return SecurityResponse{Proof: &proof}, err
	})
	authentications := 0
	client.ReauthenticateOperator = func(context.Context) error {
		authentications++
		return nil
	}
	transport := &countedOperatorTransport{RoundTripper: client.HTTP.Transport}
	client.HTTP.Transport = transport
	server.Close()
	if err := client.Call(context.Background(), "/v1/nodes", nil, nil); err == nil {
		t.Fatal("closed server request succeeded")
	}
	if authentications != 0 || transport.requests != 1 {
		t.Fatalf("network failure prompted or replayed: auth=%d HTTP=%d", authentications, transport.requests)
	}
}

func TestReauthenticationPreservesOriginalSubmissionSnapshot(t *testing.T) {
	_, broker, client, _ := testBroker(t)
	enroll(t, client, "worker")
	authority := broker.security
	authority.Lock()
	pending := &contract.Submission{ID: "original", Node: "worker", Profile: "test"}
	client.protectLocalOperator(broker.state.Identity.PublicKey, func(ctx context.Context, request SecurityRequest) (SecurityResponse, error) {
		proof, err := authority.Sign(ctx, request.Audience, request.Path, request.Digest)
		return SecurityResponse{Proof: &proof}, err
	})
	client.ReauthenticateOperator = func(ctx context.Context) error {
		pending.ID = "changed-during-auth"
		return authority.Unlock(ctx, 1)
	}
	if err := client.Call(context.Background(), "/v1/submit", pending, nil); err != nil {
		t.Fatal(err)
	}
	broker.mutex.Lock()
	defer broker.mutex.Unlock()
	if _, exists := broker.state.Jobs["original"]; !exists {
		t.Fatal("original submission was not preserved across authentication")
	}
	if _, exists := broker.state.Jobs["changed-during-auth"]; exists {
		t.Fatal("authentication changed the pending submission")
	}
}
