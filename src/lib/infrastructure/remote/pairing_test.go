package remote

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	contract "dockpipe/src/lib/domain/remote"
)

func TestOnlinePairingApprovalAndRecovery(t *testing.T) {
	root, broker, admin, server := testBroker(t)
	public, _ := NewClient(server.URL, "")
	defer public.HTTP.CloseIdleConnections()
	ctx := context.Background()
	token, _ := Secret()
	request := contract.PairingRequest{Node: "mac-mini", Token: token}
	var status contract.PairingStatus
	if err := public.Call(ctx, "/v1/pairing-request", request, &status); err == nil {
		t.Fatal("pairing accepted while closed")
	}
	if err := public.Call(ctx, "/v1/pairing-open", nil, nil); err == nil {
		t.Fatal("anonymous caller opened pairing")
	}
	if err := admin.Call(ctx, "/v1/pairing-open", nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := public.Call(ctx, "/v1/pairing-request", request, &status); err != nil {
		t.Fatal(err)
	}
	if err := public.Call(ctx, "/v1/pairing-request", contract.PairingRequest{Node: "other-node", Token: token}, nil); err == nil {
		t.Fatal("credential reused for another pending node")
	}
	code := status.Code
	if status.Status != "pending" || len(code) != 14 {
		t.Fatalf("bad status: %+v", status)
	}
	if err := public.Call(ctx, "/v1/pairing-request", request, &status); err != nil || status.Code != code {
		t.Fatalf("retry changed request: %+v %v", status, err)
	}
	worker, _ := NewClient(server.URL, token)
	defer worker.HTTP.CloseIdleConnections()
	if err := worker.Call(ctx, "/v1/next", nil, nil); err == nil {
		t.Fatal("unapproved worker authenticated")
	}
	if err := public.Call(ctx, "/v1/approve", map[string]string{"code": code}, nil); err == nil {
		t.Fatal("anonymous caller approved")
	}
	var listed json.RawMessage
	if err := admin.Call(ctx, "/v1/pairings", nil, &listed); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(listed), token) || strings.Contains(string(listed), Hash(token)) {
		t.Fatal("credential leaked in listing")
	}
	if err := admin.Call(ctx, "/v1/approve", map[string]string{"code": code}, nil); err != nil {
		t.Fatal(err)
	}
	if err := public.Call(ctx, "/v1/pairing-request", request, &status); err != nil || status.Status != "approved" {
		t.Fatalf("approval not recoverable: %+v %v", status, err)
	}
	submission := contract.Submission{ID: "online-pair-job", Node: request.Node, Profile: "approved-profile"}
	if err := admin.Call(ctx, "/v1/submit", submission, nil); err != nil {
		t.Fatal(err)
	}
	workerSession, _ := Secret()
	var claimed contract.Job
	if err := worker.Call(ctx, "/v1/next", map[string]string{"session": workerSession}, &claimed); err != nil || claimed.ID != submission.ID {
		t.Fatalf("online-paired worker could not claim its job: %+v %v", claimed, err)
	}
	attacker, _ := Secret()
	if err := public.Call(ctx, "/v1/pairing-request", contract.PairingRequest{Node: request.Node, Token: attacker}, nil); err == nil {
		t.Fatal("node replacement allowed")
	}
	recovered, err := NewBroker(root)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(request)
	recovered.state.Pairings = map[string]PairingSession{}
	if _, err := recovered.requestPairing(raw); err != nil {
		t.Fatalf("approved identity lost after session cleanup: %v", err)
	}
	if err := admin.Call(ctx, "/v1/revoke", map[string]string{"id": request.Node}, nil); err != nil {
		t.Fatal(err)
	}
	if err := public.Call(ctx, "/v1/pairing-request", request, &status); err != nil || status.Status != "revoked" {
		t.Fatal("revoked identity was not identified without granting access")
	}
	if !broker.state.Nodes[request.Node].Revoked {
		t.Fatal("revocation missing")
	}
	freshToken, err := Secret()
	if err != nil {
		t.Fatal(err)
	}
	freshRequest := contract.PairingRequest{Node: request.Node, Token: freshToken}
	if err := public.Call(ctx, "/v1/pairing-request", freshRequest, &status); err != nil || status.Status != "pending" {
		t.Fatalf("fresh pairing with revoked name: %+v %v", status, err)
	}
	if !broker.state.Nodes[request.Node].Revoked {
		t.Fatal("pairing request restored access without approval")
	}
	if err := admin.Call(ctx, "/v1/approve", map[string]string{"code": status.Code}, nil); err != nil {
		t.Fatal(err)
	}
	if err := worker.Call(ctx, "/v1/next", nil, nil); err == nil {
		t.Fatal("old credential authenticated after re-pairing")
	}
	if broker.state.Nodes[request.Node].TokenHash != Hash(freshToken) || broker.state.Nodes[request.Node].Revoked {
		t.Fatal("approval did not bind the fresh identity")
	}
}

func TestOnlinePairingExpiryDenialAndLimits(t *testing.T) {
	_, broker, admin, server := testBroker(t)
	public, _ := NewClient(server.URL, "")
	defer public.HTTP.CloseIdleConnections()
	ctx := context.Background()
	if err := admin.Call(ctx, "/v1/pairing-open", nil, nil); err != nil {
		t.Fatal(err)
	}
	token, _ := Secret()
	request := contract.PairingRequest{Node: "test-worker", Token: token}
	var status contract.PairingStatus
	if err := public.Call(ctx, "/v1/pairing-request", request, &status); err != nil {
		t.Fatal(err)
	}
	broker.mutex.Lock()
	session := broker.state.Pairings[status.Code]
	session.ExpiresAt = time.Now().Add(-time.Minute)
	broker.state.Pairings[status.Code] = session
	broker.mutex.Unlock()
	if err := admin.Call(ctx, "/v1/approve", map[string]string{"code": status.Code}, nil); err == nil {
		t.Fatal("expired request approved")
	}
	if err := public.Call(ctx, "/v1/pairing-request", request, &status); err != nil {
		t.Fatal(err)
	}
	if err := admin.Call(ctx, "/v1/deny", map[string]string{"code": status.Code}, nil); err != nil {
		t.Fatal(err)
	}
	if err := public.Call(ctx, "/v1/pairing-request", request, &status); err != nil || status.Status != "denied" {
		t.Fatal("denial not reported")
	}
	for i := 0; i < 10; i++ {
		secret, _ := Secret()
		if err := public.Call(ctx, "/v1/pairing-request", contract.PairingRequest{Node: "other", Token: secret}, nil); err != nil {
			t.Fatal(err)
		}
	}
	secret, _ := Secret()
	if err := public.Call(ctx, "/v1/pairing-request", contract.PairingRequest{Node: "other", Token: secret}, nil); err == nil {
		t.Fatal("request rate unbounded")
	}
	if err := admin.Call(ctx, "/v1/pairing-close", nil, nil); err != nil {
		t.Fatal(err)
	}
	var pending []contract.PairingStatus
	if err := admin.Call(ctx, "/v1/pairings", nil, &pending); err != nil || len(pending) != 0 {
		t.Fatal("close left pending requests")
	}
}
