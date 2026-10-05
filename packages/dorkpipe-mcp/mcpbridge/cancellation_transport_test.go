package mcpbridge

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"dorkpipe.orchestrator/providersession"
)

func TestCancellationControllerPinsScopeAcceptsClosedReasonsAndDeliversOnce(t *testing.T) {
	for _, reason := range []string{
		providersession.CancellationReasonUserRequested,
		providersession.CancellationReasonSafetyStop,
		providersession.CancellationReasonDeadline,
	} {
		t.Run(reason, func(t *testing.T) {
			controller := newTransientCancellationController()
			scope := cancellationFixtureScope(reason)
			intent := providersession.CancellationIntent{Session: scope.Session, Correlation: scope.Correlation, Reason: reason}
			delivered := make(chan providersession.CancellationIntent, 1)
			go func() {
				delivery, err := controller.awaitIntent(context.Background(), scope)
				if err == nil {
					delivered <- cloneCancellationIntent(delivery.intent)
					delivery.complete(nil)
				}
			}()
			waitForCancellationControllerScope(t, controller, scope)
			first, err := controller.pendingScope()
			if err != nil {
				t.Fatal(err)
			}
			first.Correlation.ConnectionID = "mutated"
			if again, err := controller.pendingScope(); err != nil || !reflect.DeepEqual(again, scope) {
				t.Fatalf("defensive scope=%+v err=%v", again, err)
			}
			if err := controller.submit(context.Background(), intent); err != nil {
				t.Fatal(err)
			}
			if got := <-delivered; !reflect.DeepEqual(got, intent) {
				t.Fatalf("intent=%+v want %+v", got, intent)
			}
			if _, err := controller.pendingScope(); err == nil {
				t.Fatal("submitted scope remained visible")
			}
			if err := controller.submit(context.Background(), intent); err == nil {
				t.Fatal("replayed cancellation intent was accepted")
			}
		})
	}
}

func TestCancellationControllerRejectsMalformedSubstitutedAndCrossScopeIntents(t *testing.T) {
	scope := cancellationFixtureScope("rejection")
	base := providersession.CancellationIntent{Session: scope.Session, Correlation: scope.Correlation, Reason: providersession.CancellationReasonUserRequested}
	cases := []struct {
		name   string
		mutate func(*providersession.CancellationIntent)
	}{
		{name: "unknown_reason", mutate: func(intent *providersession.CancellationIntent) { intent.Reason = "other" }},
		{name: "cross_process", mutate: func(intent *providersession.CancellationIntent) {
			intent.Correlation.ProcessIncarnationID = "other-process"
		}},
		{name: "cross_connection", mutate: func(intent *providersession.CancellationIntent) { intent.Correlation.ConnectionID = "other-connection" }},
		{name: "cross_session", mutate: func(intent *providersession.CancellationIntent) {
			intent.Session.SessionID, intent.Correlation.SessionID = "other-session", "other-session"
		}},
		{name: "cross_turn", mutate: func(intent *providersession.CancellationIntent) { intent.Correlation.InteractionID = "other-turn" }},
		{name: "activity_scope", mutate: func(intent *providersession.CancellationIntent) { intent.Correlation.ActivityID = "forbidden" }},
		{name: "request_scope", mutate: func(intent *providersession.CancellationIntent) { intent.Correlation.RequestID = "forbidden" }},
		{name: "decision_scope", mutate: func(intent *providersession.CancellationIntent) { intent.Correlation.DecisionID = "forbidden" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			controller := newTransientCancellationController()
			go func() { _, _ = controller.awaitIntent(context.Background(), scope) }()
			waitForCancellationControllerScope(t, controller, scope)
			intent := cloneCancellationIntent(base)
			tc.mutate(&intent)
			err := controller.submit(context.Background(), intent)
			if err == nil || strings.Contains(err.Error(), "forbidden") || strings.Contains(err.Error(), "other-") {
				t.Fatalf("unsafe rejection error=%v", err)
			}
			waitForCancellationControllerScope(t, controller, scope)
			controller.close(errors.New("fixture complete"))
		})
	}
}

func TestServeStdioTransportsExactCancellationWithoutSettlingChat(t *testing.T) {
	enableMCPExecTier(t)
	server := NewServer("cancellation-stdio")
	scope := cancellationFixtureScope("stdio")
	intent := providersession.CancellationIntent{Session: scope.Session, Correlation: scope.Correlation, Reason: providersession.CancellationReasonUserRequested}
	delivered := make(chan providersession.CancellationIntent, 1)
	providerResolved := make(chan struct{})
	server.providerPoolChatInteractiveStreamRunner = func(ctx context.Context, _ []string, _ *transientApprovalController, _ *transientUserInputController, cancellations *transientCancellationController) (string, string, int, error) {
		delivery, err := cancellations.awaitIntent(ctx, scope)
		if err != nil {
			return "", "", -1, err
		}
		delivered <- cloneCancellationIntent(delivery.intent)
		delivery.complete(nil)
		select {
		case <-providerResolved:
			return `{"state":"cancelled"}`, "", 0, nil
		case <-ctx.Done():
			return "", "", -1, ctx.Err()
		}
	}

	inReader, inWriter := io.Pipe()
	outReader, outWriter := io.Pipe()
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.ServeStdio(inReader, outWriter, io.Discard) }()
	reader := bufio.NewReader(outReader)
	writeStdioFixture(t, inWriter, providerPoolChatFixture(`"cancellation-chat"`))
	waitForActiveCancellationScope(t, server, scope)

	writeStdioFixture(t, inWriter, cancellationRequestFixture("1"))
	first := cancellationScopeFromToolResponse(t, readStdioFixture(t, reader))
	first.Correlation.ConnectionID = "mutated"
	writeStdioFixture(t, inWriter, cancellationRequestFixture("2"))
	if again := cancellationScopeFromToolResponse(t, readStdioFixture(t, reader)); !reflect.DeepEqual(again, scope) {
		t.Fatalf("scope read was not defensive: %+v", again)
	}

	substituted := cloneCancellationIntent(intent)
	substituted.Correlation.ConnectionID = "other-connection"
	writeStdioFixture(t, inWriter, cancellationDeliverFixture("3", substituted))
	assertToolCallError(t, readStdioFixture(t, reader), "3", "exact pending scope")
	waitForActiveCancellationScope(t, server, scope)

	writeStdioFixture(t, inWriter, cancellationDeliverFixture("4", intent))
	deliveryResponse := readStdioFixture(t, reader)
	assertResponseID(t, deliveryResponse, "4")
	if !strings.Contains(string(deliveryResponse), `\"delivered\":true`) || strings.Contains(string(deliveryResponse), "cancelled") || strings.Contains(string(deliveryResponse), "cancellation_requested") {
		t.Fatalf("delivery response made a terminal claim: %s", deliveryResponse)
	}
	if got := <-delivered; !reflect.DeepEqual(got, intent) {
		t.Fatalf("delivered=%+v want %+v", got, intent)
	}
	select {
	case <-time.After(20 * time.Millisecond):
	case <-providerResolved:
		t.Fatal("provider fixture resolved unexpectedly")
	}
	writeStdioFixture(t, inWriter, cancellationRequestFixture("5"))
	assertToolCallError(t, readStdioFixture(t, reader), "5", "no exact")
	close(providerResolved)
	assertResponseID(t, readStdioFixture(t, reader), `"cancellation-chat"`)
	if err := inWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := waitServeResult(t, serveDone); err != nil {
		t.Fatal(err)
	}
	_ = outWriter.Close()
	_ = outReader.Close()
	if err := server.submitActiveCancellationIntent(context.Background(), intent); err == nil || !strings.Contains(err.Error(), "no provider-pool chat") {
		t.Fatalf("post-chat error=%v", err)
	}
}

func TestRelayInteractiveFramesKeepsCancellationConcurrentAndWritesSerializedFrames(t *testing.T) {
	approvals := newTransientApprovalController()
	inputs := newTransientUserInputController()
	cancellations := newTransientCancellationController()
	scope := cancellationFixtureScope("relay")
	intent := providersession.CancellationIntent{Session: scope.Session, Correlation: scope.Correlation, Reason: providersession.CancellationReasonDeadline}
	request := approvalFixtureRequest("cancellation-relay")
	decision := providersession.ApprovalDecision{Correlation: request.Correlation, Decision: providersession.DecisionApprove}
	scopeJSON, _ := json.Marshal(privateCancellationScopeFrame{Type: "cancellation_scope", Scope: scope})
	approvalJSON, _ := json.Marshal(privateApprovalRequestFrame{Type: "approval_request", Request: request})
	fromChild, childStderr := io.Pipe()
	toChild := &capturedPrivateWriter{frames: make(chan []byte, 2)}
	relayDone := make(chan error, 1)
	go func() {
		_, err := relayInteractiveFrames(context.Background(), fromChild, toChild, approvals, inputs, cancellations)
		relayDone <- err
	}()
	if _, err := fmt.Fprintf(childStderr, "%s%s\n%s%s\n", privateCancellationFramePrefix, scopeJSON, privateApprovalFramePrefix, approvalJSON); err != nil {
		t.Fatal(err)
	}
	waitForCancellationControllerScope(t, cancellations, scope)
	waitForControllerRequest(t, approvals, request)

	cancellationSubmit := make(chan error, 1)
	go func() { cancellationSubmit <- cancellations.submit(context.Background(), intent) }()
	first := <-toChild.frames
	if !bytes.HasPrefix(first, []byte(privateCancellationFramePrefix)) || bytes.Count(first, []byte("\n")) != 1 {
		t.Fatalf("first child frame=%q", first)
	}
	if err := <-cancellationSubmit; err != nil {
		t.Fatal(err)
	}
	if err := approvals.submit(context.Background(), decision); err != nil {
		t.Fatal(err)
	}
	second := <-toChild.frames
	if !bytes.HasPrefix(second, []byte(privateApprovalFramePrefix)) || bytes.Count(second, []byte("\n")) != 1 {
		t.Fatalf("second child frame=%q", second)
	}
	if _, err := inputs.pendingPrompt(); err == nil {
		t.Fatal("user-input controller observed another controller's frame")
	}
	if err := childStderr.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-relayDone; err != nil {
		t.Fatal(err)
	}
}

func TestCancellationOperationsAreClosedExecTierAndSameServerOwned(t *testing.T) {
	enableMCPExecTier(t)
	seen := map[string]json.RawMessage{}
	for _, tool := range mcpToolCatalog() {
		if strings.Contains(tool.Name, "cancellation") {
			seen[tool.Name] = tool.InputSchema
			if tier, ok := minTierForTool(tool.Name); !ok || tier != TierExec {
				t.Fatalf("cancellation tool %q tier=%v present=%v", tool.Name, tier, ok)
			}
		}
	}
	if len(seen) != 2 || seen["dorkpipe.provider_pool_cancellation_request"] == nil || seen["dorkpipe.provider_pool_cancellation_deliver"] == nil {
		t.Fatalf("cancellation tools=%v", seen)
	}
	if string(seen["dorkpipe.provider_pool_cancellation_request"]) != `{"type":"object","properties":{},"additionalProperties":false}` {
		t.Fatalf("request schema=%s", seen["dorkpipe.provider_pool_cancellation_request"])
	}
	var deliverSchema struct {
		Properties           map[string]json.RawMessage `json:"properties"`
		Required             []string                   `json:"required"`
		AdditionalProperties bool                       `json:"additionalProperties"`
	}
	if err := json.Unmarshal(seen["dorkpipe.provider_pool_cancellation_deliver"], &deliverSchema); err != nil {
		t.Fatal(err)
	}
	if deliverSchema.AdditionalProperties || !reflect.DeepEqual(deliverSchema.Required, []string{"session", "correlation", "reason"}) {
		t.Fatalf("deliver schema=%+v", deliverSchema)
	}
	for _, raw := range []json.RawMessage{json.RawMessage(`null`), json.RawMessage(`{"unexpected":true}`)} {
		if _, _, err := NewServer("closed-request").dispatchTool(context.Background(), "dorkpipe.provider_pool_cancellation_request", raw); err == nil || !strings.Contains(err.Error(), "invalid cancellation-request") {
			t.Fatalf("closed request raw=%s err=%v", raw, err)
		}
	}

	server := NewServer("one")
	other := NewServer("other")
	active, ok := server.beginActiveProviderPoolChat(context.Background())
	if !ok {
		t.Fatal("failed to begin fixture chat")
	}
	scope := cancellationFixtureScope("same-server")
	go func() {
		_, _ = active.cancellations.awaitIntent(active.ctx, scope)
		server.finishActiveProviderPoolChat(active)
	}()
	waitForActiveCancellationScope(t, server, scope)
	intent := providersession.CancellationIntent{Session: scope.Session, Correlation: scope.Correlation, Reason: providersession.CancellationReasonSafetyStop}
	if err := other.submitActiveCancellationIntent(context.Background(), intent); err == nil || !strings.Contains(err.Error(), "no provider-pool chat") {
		t.Fatalf("cross-server error=%v", err)
	}
	server.cancelAndWaitForActiveProviderPoolChat()
	if err := server.submitActiveCancellationIntent(context.Background(), intent); err == nil || !strings.Contains(err.Error(), "no provider-pool chat") {
		t.Fatalf("post-shutdown error=%v", err)
	}
}

func TestHTTPCancellationOperationsAddressOnlyOneActiveChatAndRemainNonTerminal(t *testing.T) {
	enableMCPExecTier(t)
	server := NewServer("cancellation-http")
	scope := cancellationFixtureScope("http")
	intent := providersession.CancellationIntent{Session: scope.Session, Correlation: scope.Correlation, Reason: providersession.CancellationReasonSafetyStop}
	providerResolved := make(chan struct{})
	var runnerCalls atomic.Int32
	server.providerPoolChatInteractiveStreamRunner = func(ctx context.Context, _ []string, _ *transientApprovalController, _ *transientUserInputController, cancellations *transientCancellationController) (string, string, int, error) {
		runnerCalls.Add(1)
		delivery, err := cancellations.awaitIntent(ctx, scope)
		if err != nil {
			return "", "", -1, err
		}
		delivery.complete(nil)
		select {
		case <-providerResolved:
			return `{"state":"cancelled"}`, "", 0, nil
		case <-ctx.Done():
			return "", "", -1, ctx.Err()
		}
	}
	srv := newIPv4TestServer(t, server.jsonRPCHandler(io.Discard))
	defer srv.Close()
	type callResult struct {
		response *rpcResponse
		err      error
	}
	chatDone := make(chan callResult, 1)
	go func() {
		response, err := callHTTPRPC(srv.URL, providerPoolChatFixture("20"))
		chatDone <- callResult{response: response, err: err}
	}()
	waitForActiveCancellationScope(t, server, scope)
	read, err := callHTTPRPC(srv.URL, cancellationRequestFixture("21"))
	if err != nil || !reflect.DeepEqual(cancellationScopeFromRPCResponse(t, read), scope) {
		t.Fatalf("read=%+v err=%v", read, err)
	}
	second, err := callHTTPRPC(srv.URL, providerPoolChatFixture("22"))
	if err != nil || second.Error == nil || !strings.Contains(second.Error.Message, "already active") || runnerCalls.Load() != 1 {
		t.Fatalf("second=%+v err=%v calls=%d", second, err, runnerCalls.Load())
	}
	deliver, err := callHTTPRPC(srv.URL, cancellationDeliverFixture("23", intent))
	if err != nil || deliver.Error != nil || !strings.Contains(string(deliver.Result), "delivered") {
		t.Fatalf("deliver=%+v err=%v", deliver, err)
	}
	select {
	case result := <-chatDone:
		t.Fatalf("chat completed on delivery: %+v", result)
	default:
	}
	close(providerResolved)
	select {
	case result := <-chatDone:
		if result.err != nil || result.response == nil || result.response.Error != nil {
			t.Fatalf("chat result=%+v", result)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for HTTP chat completion")
	}
}

type capturedPrivateWriter struct {
	frames chan []byte
}

func (w *capturedPrivateWriter) Write(raw []byte) (int, error) {
	w.frames <- append([]byte(nil), raw...)
	return len(raw), nil
}

func cancellationFixtureScope(suffix string) providerPoolCancellationScope {
	sessionID := "session-cancellation-" + suffix
	return providerPoolCancellationScope{
		Session: providersession.SessionRef{Provider: "codex", SessionID: sessionID},
		Correlation: providersession.Correlation{
			ProcessIncarnationID: "process-cancellation-" + suffix,
			ConnectionID:         "connection-cancellation-" + suffix,
			SessionID:            sessionID,
			InteractionID:        "turn-cancellation-" + suffix,
		},
	}
}

func waitForCancellationControllerScope(t *testing.T, controller *transientCancellationController, want providerPoolCancellationScope) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		got, err := controller.pendingScope()
		if err == nil && reflect.DeepEqual(got, want) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed out waiting for cancellation scope")
}

func waitForActiveCancellationScope(t *testing.T, server *Server, want providerPoolCancellationScope) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	var last providerPoolCancellationScope
	var lastErr error
	for time.Now().Before(deadline) {
		got, err := server.activeCancellationScope()
		last, lastErr = got, err
		if err == nil && reflect.DeepEqual(got, want) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for active cancellation scope: got=%+v want=%+v err=%v", last, want, lastErr)
}

func cancellationRequestFixture(id string) string {
	return fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"method":"tools/call","params":{"name":"dorkpipe.provider_pool_cancellation_request","arguments":{}}}`, id)
}

func cancellationDeliverFixture(id string, intent providersession.CancellationIntent) string {
	encoded, _ := json.Marshal(intent)
	return fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"method":"tools/call","params":{"name":"dorkpipe.provider_pool_cancellation_deliver","arguments":%s}}`, id, encoded)
}

func cancellationScopeFromToolResponse(t *testing.T, body []byte) providerPoolCancellationScope {
	t.Helper()
	var response rpcResponse
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatal(err)
	}
	return cancellationScopeFromRPCResponse(t, &response)
}

func cancellationScopeFromRPCResponse(t *testing.T, response *rpcResponse) providerPoolCancellationScope {
	t.Helper()
	if response == nil || response.Error != nil {
		t.Fatalf("cancellation response=%+v", response)
	}
	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(response.Result, &result); err != nil || len(result.Content) != 1 {
		t.Fatalf("cancellation result=%s err=%v", response.Result, err)
	}
	var scope providerPoolCancellationScope
	if err := json.Unmarshal([]byte(result.Content[0].Text), &scope); err != nil {
		t.Fatal(err)
	}
	return scope
}
