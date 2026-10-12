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
	"testing"
	"time"

	"dorkpipe.orchestrator/providersession"
)

func TestServeStdioTransportsExactPendingApprovalWithoutUnblockingChat(t *testing.T) {
	enableMCPExecTier(t)
	server := NewServer("test")
	request := approvalFixtureRequest("request-one")
	decisionDelivered := make(chan struct{})
	providerResolved := make(chan struct{})
	server.providerPoolChatStreamRunner = func(ctx context.Context, _ []string, approvals *transientApprovalController) (string, string, int, error) {
		delivery, err := approvals.awaitDecision(ctx, request)
		if err != nil {
			return "", "", -1, err
		}
		delivery.complete(nil)
		close(decisionDelivered)
		if delivery.decision.Decision == providersession.DecisionDeny {
			return `{"state":"failed","summary":"approval_denied"}`, "", 0, nil
		}
		select {
		case <-providerResolved:
			return `{"state":"ready"}`, "", 0, nil
		case <-ctx.Done():
			return "", "", -1, ctx.Err()
		}
	}

	inReader, inWriter := io.Pipe()
	outReader, outWriter := io.Pipe()
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.ServeStdio(inReader, outWriter, io.Discard) }()
	reader := bufio.NewReader(outReader)

	writeStdioFixture(t, inWriter, providerPoolChatFixture(`"chat"`))
	waitForApprovalRequest(t, server, request)
	writeStdioFixture(t, inWriter, `{"jsonrpc":"2.0","id":1,"method":"ping"}`)
	writeStdioFixture(t, inWriter, approvalReadFixture("2"))
	assertResponseID(t, readStdioFixture(t, reader), "1")
	firstRead := readStdioFixture(t, reader)
	assertResponseID(t, firstRead, "2")
	if got := approvalRequestFromToolResponse(t, firstRead); !reflect.DeepEqual(got, request) {
		t.Fatalf("pending request=%+v, want %+v", got, request)
	}

	writeStdioFixture(t, inWriter, approvalReadFixture("3"))
	secondRead := readStdioFixture(t, reader)
	if got := approvalRequestFromToolResponse(t, secondRead); !reflect.DeepEqual(got, request) {
		t.Fatalf("second read=%+v, want %+v", got, request)
	}

	substituted := providersession.ApprovalDecision{Correlation: request.Correlation, Decision: providersession.DecisionApprove}
	substituted.Correlation.RequestID = "other-request"
	writeStdioFixture(t, inWriter, approvalDecisionFixture("4", substituted))
	assertToolCallError(t, readStdioFixture(t, reader), "4", "does not match")
	waitForApprovalRequest(t, server, request)

	exact := providersession.ApprovalDecision{Correlation: request.Correlation, Decision: providersession.DecisionApprove}
	writeStdioFixture(t, inWriter, approvalDecisionFixture("5", exact))
	decideResponse := readStdioFixture(t, reader)
	assertResponseID(t, decideResponse, "5")
	waitFixtureSignal(t, decisionDelivered, "approval delivery")

	writeStdioFixture(t, inWriter, approvalReadFixture("6"))
	assertToolCallError(t, readStdioFixture(t, reader), "6", "no exact")
	writeStdioFixture(t, inWriter, approvalDecisionFixture("7", exact))
	assertToolCallError(t, readStdioFixture(t, reader), "7", "no exact")
	writeStdioFixture(t, inWriter, `{"jsonrpc":"2.0","id":8,"method":"ping"}`)
	assertResponseID(t, readStdioFixture(t, reader), "8")

	close(providerResolved)
	chat := readStdioFixture(t, reader)
	assertResponseID(t, chat, `"chat"`)
	if err := inWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := waitServeResult(t, serveDone); err != nil {
		t.Fatal(err)
	}
	_ = outWriter.Close()
	_ = outReader.Close()
}

func TestServeStdioDenialIsDeliveredOnceAndEndsTheSameChat(t *testing.T) {
	enableMCPExecTier(t)
	server := NewServer("test")
	request := approvalFixtureRequest("deny-request")
	var delivered providersession.ApprovalDecision
	server.providerPoolChatStreamRunner = func(ctx context.Context, _ []string, approvals *transientApprovalController) (string, string, int, error) {
		delivery, err := approvals.awaitDecision(ctx, request)
		if err != nil {
			return "", "", -1, err
		}
		delivered = delivery.decision
		delivery.complete(nil)
		return `{"state":"failed","summary":"approval_denied"}`, "", 0, nil
	}

	inReader, inWriter := io.Pipe()
	outReader, outWriter := io.Pipe()
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.ServeStdio(inReader, outWriter, io.Discard) }()
	reader := bufio.NewReader(outReader)
	writeStdioFixture(t, inWriter, providerPoolChatFixture("10"))
	waitForApprovalRequest(t, server, request)
	decision := providersession.ApprovalDecision{Correlation: request.Correlation, Decision: providersession.DecisionDeny}
	writeStdioFixture(t, inWriter, approvalDecisionFixture("11", decision))
	assertResponseID(t, readStdioFixture(t, reader), "11")
	assertResponseID(t, readStdioFixture(t, reader), "10")
	if err := inWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := waitServeResult(t, serveDone); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(delivered, decision) {
		t.Fatalf("delivered=%+v, want %+v", delivered, decision)
	}
	_ = outWriter.Close()
	_ = outReader.Close()
}

func TestServeStdioEOFInvalidatesPendingApprovalAndJoinsChat(t *testing.T) {
	enableMCPExecTier(t)
	server := NewServer("test")
	request := approvalFixtureRequest("eof-request")
	joined := make(chan struct{})
	server.providerPoolChatStreamRunner = func(ctx context.Context, _ []string, approvals *transientApprovalController) (string, string, int, error) {
		defer close(joined)
		_, err := approvals.awaitDecision(ctx, request)
		return "", "", -1, err
	}
	inReader, inWriter := io.Pipe()
	var output bytes.Buffer
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.ServeStdio(inReader, &output, io.Discard) }()
	writeStdioFixture(t, inWriter, providerPoolChatFixture("20"))
	waitForApprovalRequest(t, server, request)
	if err := inWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := waitServeResult(t, serveDone); err != nil {
		t.Fatal(err)
	}
	waitFixtureSignal(t, joined, "approval handler join")
	if _, err := server.activeApprovalRequest(); err == nil {
		t.Fatal("pending approval survived EOF")
	}
	if output.Len() != 0 {
		t.Fatalf("late response after EOF: %q", output.String())
	}
}

func TestApprovalControllerPinsCopiesRejectsSecondPendingAndCrossServer(t *testing.T) {
	controller := newTransientApprovalController()
	request := approvalFixtureRequest("copy-request")
	original := append([]string(nil), request.AllowedDecisions...)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	deliveryDone := make(chan error, 1)
	go func() {
		_, err := controller.awaitDecision(ctx, request)
		deliveryDone <- err
	}()
	waitForControllerRequest(t, controller, request)
	request.AllowedDecisions[0] = providersession.DecisionDeny
	request.AllowedDecisions = append(request.AllowedDecisions, "widened")
	pinned, err := controller.pendingRequest()
	if err != nil || !reflect.DeepEqual(pinned.AllowedDecisions, original) {
		t.Fatalf("pinned=%+v err=%v", pinned, err)
	}
	pinned.AllowedDecisions[0] = "mutated"
	again, err := controller.pendingRequest()
	if err != nil || !reflect.DeepEqual(again.AllowedDecisions, original) {
		t.Fatalf("defensive copy=%+v err=%v", again, err)
	}
	if _, err := controller.awaitDecision(ctx, approvalFixtureRequest("second-request")); err == nil || !strings.Contains(err.Error(), "already pending") {
		t.Fatalf("second pending error=%v", err)
	}

	other := NewServer("other")
	decision := providersession.ApprovalDecision{Correlation: again.Correlation, Decision: providersession.DecisionApprove}
	if err := other.submitActiveApprovalDecision(ctx, decision); err == nil || !strings.Contains(err.Error(), "no provider-pool chat") {
		t.Fatalf("cross-server decision error=%v", err)
	}
	controller.close(errors.New("fixture closed"))
	if err := <-deliveryDone; err == nil || !strings.Contains(err.Error(), "fixture closed") {
		t.Fatalf("await error=%v", err)
	}
	if _, err := controller.pendingRequest(); err == nil {
		t.Fatal("pending request survived close")
	}
}

func TestApprovalControllerRejectsInvalidStaleAndDisallowedDecisionsBeforeDelivery(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*providersession.ApprovalDecision)
	}{
		{"missing", func(d *providersession.ApprovalDecision) { d.Correlation.DecisionID = "" }},
		{"stale-session", func(d *providersession.ApprovalDecision) { d.Correlation.SessionID = "stale" }},
		{"cross-turn", func(d *providersession.ApprovalDecision) { d.Correlation.InteractionID = "other-turn" }},
		{"cross-request", func(d *providersession.ApprovalDecision) { d.Correlation.RequestID = "other-request" }},
		{"malformed", func(d *providersession.ApprovalDecision) { d.Decision = "allow" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			controller := newTransientApprovalController()
			request := approvalFixtureRequest(tc.name)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			go func() { _, _ = controller.awaitDecision(ctx, request) }()
			waitForControllerRequest(t, controller, request)
			decision := providersession.ApprovalDecision{Correlation: request.Correlation, Decision: providersession.DecisionApprove}
			tc.mutate(&decision)
			if err := controller.submit(ctx, decision); err == nil {
				t.Fatal("invalid decision was accepted")
			}
			waitForControllerRequest(t, controller, request)
			controller.close(errors.New("fixture done"))
		})
	}

	controller := newTransientApprovalController()
	request := approvalFixtureRequest("deny-only")
	request.Reason = providersession.ApprovalReasonPermission
	request.AllowedDecisions = []string{providersession.DecisionDeny}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _, _ = controller.awaitDecision(ctx, request) }()
	waitForControllerRequest(t, controller, request)
	approve := providersession.ApprovalDecision{Correlation: request.Correlation, Decision: providersession.DecisionApprove}
	if err := controller.submit(ctx, approve); err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("disallowed approval error=%v", err)
	}
	controller.close(errors.New("fixture done"))
}

func TestRelayApprovalFramesCarriesOnlyExactNeutralFrames(t *testing.T) {
	controller := newTransientApprovalController()
	request := approvalFixtureRequest("transport-request")
	requestFrame, err := json.Marshal(privateApprovalRequestFrame{Type: "approval_request", Request: request})
	if err != nil {
		t.Fatal(err)
	}
	fromChildReader, fromChildWriter := io.Pipe()
	var toChild bytes.Buffer
	relayDone := make(chan error, 1)
	go func() {
		_, relayErr := relayApprovalFrames(context.Background(), fromChildReader, &toChild, controller)
		relayDone <- relayErr
	}()
	if _, err := fmt.Fprintf(fromChildWriter, "%s%s\n", privateApprovalFramePrefix, requestFrame); err != nil {
		t.Fatal(err)
	}
	waitForControllerRequest(t, controller, request)
	decision := providersession.ApprovalDecision{Correlation: request.Correlation, Decision: providersession.DecisionApprove}
	if err := controller.submit(context.Background(), decision); err != nil {
		t.Fatal(err)
	}
	if err := fromChildWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-relayDone; err != nil {
		t.Fatal(err)
	}
	line := strings.TrimSpace(toChild.String())
	if !strings.HasPrefix(line, privateApprovalFramePrefix) {
		t.Fatalf("decision frame=%q", line)
	}
	var got privateApprovalDecisionFrame
	if err := decodeClosedJSON([]byte(strings.TrimPrefix(line, privateApprovalFramePrefix)), &got); err != nil {
		t.Fatal(err)
	}
	if got.Type != "approval_decision" || !reflect.DeepEqual(got.Decision, decision) {
		t.Fatalf("decision frame=%+v", got)
	}
	if strings.Contains(toChild.String(), request.Reason) || strings.Contains(toChild.String(), "command") || strings.Contains(toChild.String(), "prompt") {
		t.Fatalf("decision transport exposed request/provider content: %q", toChild.String())
	}
}

func TestRelayApprovalFramesWriteFailureRejectsDeliveryAndClearsPending(t *testing.T) {
	controller := newTransientApprovalController()
	request := approvalFixtureRequest("write-failure")
	requestFrame, err := json.Marshal(privateApprovalRequestFrame{Type: "approval_request", Request: request})
	if err != nil {
		t.Fatal(err)
	}
	input := bytes.NewBufferString(privateApprovalFramePrefix + string(requestFrame) + "\n")
	relayDone := make(chan error, 1)
	go func() {
		_, relayErr := relayApprovalFrames(context.Background(), input, errorWriter{}, controller)
		relayDone <- relayErr
	}()
	waitForControllerRequest(t, controller, request)
	decision := providersession.ApprovalDecision{Correlation: request.Correlation, Decision: providersession.DecisionApprove}
	if err := controller.submit(context.Background(), decision); err == nil || !strings.Contains(err.Error(), "fixture transport failure") {
		t.Fatalf("submit error=%v", err)
	}
	if err := <-relayDone; err == nil || !strings.Contains(err.Error(), "fixture transport failure") {
		t.Fatalf("relay error=%v", err)
	}
	if _, err := controller.pendingRequest(); err == nil {
		t.Fatal("pending approval survived decision transport failure")
	}
}

func approvalFixtureRequest(requestID string) providersession.ApprovalRequest {
	return providersession.ApprovalRequest{
		Correlation: providersession.Correlation{
			ProcessIncarnationID: "process-one",
			ConnectionID:         "connection-one",
			SessionID:            "session-one",
			InteractionID:        "turn-one",
			ActivityID:           "item-one",
			RequestID:            requestID,
			DecisionID:           "decision-" + requestID,
		},
		Reason:           providersession.ApprovalReasonCommandExecution,
		AllowedDecisions: []string{providersession.DecisionApprove, providersession.DecisionDeny},
	}
}

func waitForApprovalRequest(t *testing.T, server *Server, want providersession.ApprovalRequest) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	var last providersession.ApprovalRequest
	var lastErr error
	for time.Now().Before(deadline) {
		got, err := server.activeApprovalRequest()
		last, lastErr = got, err
		if err == nil && reflect.DeepEqual(got, want) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for active approval request: got=%+v want=%+v err=%v", last, want, lastErr)
}

func waitForControllerRequest(t *testing.T, controller *transientApprovalController, want providersession.ApprovalRequest) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		got, err := controller.pendingRequest()
		if err == nil && reflect.DeepEqual(got, want) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed out waiting for controller approval request")
}

func approvalReadFixture(id string) string {
	return fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"method":"tools/call","params":{"name":"dorkpipe.provider_pool_approval_request","arguments":{}}}`, id)
}

func approvalDecisionFixture(id string, decision providersession.ApprovalDecision) string {
	encoded, _ := json.Marshal(decision)
	return fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"method":"tools/call","params":{"name":"dorkpipe.provider_pool_approval_decide","arguments":%s}}`, id, encoded)
}

func approvalRequestFromToolResponse(t *testing.T, body []byte) providersession.ApprovalRequest {
	t.Helper()
	var response struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Result.Content) != 1 {
		t.Fatalf("tool response=%s", body)
	}
	var request providersession.ApprovalRequest
	if err := json.Unmarshal([]byte(response.Result.Content[0].Text), &request); err != nil {
		t.Fatal(err)
	}
	return request
}

func assertToolCallError(t *testing.T, body []byte, wantID, contains string) {
	t.Helper()
	assertResponseID(t, body, wantID)
	var response rpcResponse
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatal(err)
	}
	if response.Error == nil || !strings.Contains(response.Error.Message, contains) {
		t.Fatalf("response=%s, want error containing %q", body, contains)
	}
}
