package mcpbridge

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"dorkpipe.orchestrator/providersession"
)

func TestServeStdioTransportsExactPendingUserInputWithoutSettlingChat(t *testing.T) {
	enableMCPExecTier(t)
	server := NewServer("test")
	prompt := userInputFixturePrompt("stdio", providersession.UserInputPromptSelectMany)
	response := providersession.UserInputResponse{Correlation: prompt.Correlation, PromptRef: prompt.PromptRef, SelectedOptionRefs: []string{"option-a", "option-c"}}
	delivered := make(chan providersession.UserInputResponse, 1)
	providerResolved := make(chan struct{})
	server.providerPoolChatInteractiveStreamRunner = func(ctx context.Context, _ []string, _ *transientApprovalController, inputs *transientUserInputController, _ *transientCancellationController) (string, string, int, error) {
		delivery, err := inputs.awaitResponse(ctx, prompt)
		if err != nil {
			return "", "", -1, err
		}
		delivery.complete(nil)
		delivered <- cloneUserInputResponse(delivery.response)
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
	waitForUserInputPrompt(t, server, prompt)

	if _, err := server.activeApprovalRequest(); err == nil {
		t.Fatal("approval operation observed user-input state")
	}
	writeStdioFixture(t, inWriter, userInputReadFixture("1"))
	first := readStdioFixture(t, reader)
	got := userInputPromptFromToolResponse(t, first)
	if !reflect.DeepEqual(got, prompt) {
		t.Fatalf("prompt=%+v, want %+v", got, prompt)
	}
	got.Kind = providersession.UserInputPromptText
	got.Options[0].OptionRef = "mutated"
	got.Options = append(got.Options, providersession.UserInputOption{OptionRef: "injected", Label: "Injected"})
	writeStdioFixture(t, inWriter, userInputReadFixture("2"))
	if again := userInputPromptFromToolResponse(t, readStdioFixture(t, reader)); !reflect.DeepEqual(again, prompt) {
		t.Fatalf("request read was not defensive: %+v", again)
	}

	substituted := cloneUserInputResponse(response)
	substituted.Correlation.ActivityID = "other-item"
	writeStdioFixture(t, inWriter, userInputResponseFixture("3", substituted))
	assertToolCallError(t, readStdioFixture(t, reader), "3", "match the exact")
	waitForUserInputPrompt(t, server, prompt)

	writeStdioFixture(t, inWriter, userInputResponseFixture("4", response))
	assertResponseID(t, readStdioFixture(t, reader), "4")
	select {
	case got := <-delivered:
		if !reflect.DeepEqual(got, response) {
			t.Fatalf("delivered=%+v, want %+v", got, response)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for exact user-input delivery")
	}

	writeStdioFixture(t, inWriter, userInputReadFixture("5"))
	assertToolCallError(t, readStdioFixture(t, reader), "5", "no exact")
	writeStdioFixture(t, inWriter, userInputResponseFixture("6", response))
	assertToolCallError(t, readStdioFixture(t, reader), "6", "no exact")
	writeStdioFixture(t, inWriter, `{"jsonrpc":"2.0","id":7,"method":"ping"}`)
	assertResponseID(t, readStdioFixture(t, reader), "7")

	close(providerResolved)
	assertResponseID(t, readStdioFixture(t, reader), `"chat"`)
	if err := inWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := waitServeResult(t, serveDone); err != nil {
		t.Fatal(err)
	}
	_ = outWriter.Close()
	_ = outReader.Close()
}

func TestUserInputControllerPinsCopiesRejectsSecondPendingAndDeliversOnce(t *testing.T) {
	controller := newTransientUserInputController()
	prompt := userInputFixturePrompt("copies", providersession.UserInputPromptSelectMany)
	original := cloneUserInputPrompt(prompt)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	deliveryDone := make(chan providersession.UserInputResponse, 1)
	waitDone := make(chan error, 1)
	go func() {
		delivery, err := controller.awaitResponse(ctx, prompt)
		if err == nil {
			deliveryDone <- cloneUserInputResponse(delivery.response)
			delivery.complete(nil)
		}
		waitDone <- err
	}()
	waitForControllerUserInputPrompt(t, controller, original)
	prompt.Kind = providersession.UserInputPromptText
	prompt.Options[0].OptionRef = "source-mutated"
	prompt.Options = append(prompt.Options, providersession.UserInputOption{OptionRef: "injected", Label: "Injected"})
	pinned, err := controller.pendingPrompt()
	if err != nil || !reflect.DeepEqual(pinned, original) {
		t.Fatalf("pinned=%+v err=%v", pinned, err)
	}
	pinned.Options[0].OptionRef = "caller-mutated"
	if again, err := controller.pendingPrompt(); err != nil || !reflect.DeepEqual(again, original) {
		t.Fatalf("defensive prompt=%+v err=%v", again, err)
	}
	if _, err := controller.awaitResponse(ctx, userInputFixturePrompt("second", providersession.UserInputPromptSelectOne)); err == nil || !strings.Contains(err.Error(), "already pending") {
		t.Fatalf("second pending error=%v", err)
	}

	response := providersession.UserInputResponse{Correlation: original.Correlation, PromptRef: original.PromptRef, SelectedOptionRefs: []string{"option-a", "option-c"}}
	if err := controller.submit(ctx, response); err != nil {
		t.Fatal(err)
	}
	if err := <-waitDone; err != nil {
		t.Fatal(err)
	}
	if got := <-deliveryDone; !reflect.DeepEqual(got, response) {
		t.Fatalf("delivered=%+v, want %+v", got, response)
	}
	if err := controller.submit(ctx, response); err == nil || !strings.Contains(err.Error(), "no exact") {
		t.Fatalf("replay error=%v", err)
	}
}

func TestUserInputControllerRejectsMalformedStaleSubstitutedAndKindMismatchedResponses(t *testing.T) {
	selectPrompt := userInputFixturePrompt("invalid", providersession.UserInputPromptSelectMany)
	textPrompt := userInputFixturePrompt("invalid-text", providersession.UserInputPromptText)
	baseSelect := providersession.UserInputResponse{Correlation: selectPrompt.Correlation, PromptRef: selectPrompt.PromptRef, SelectedOptionRefs: []string{"option-a"}}
	baseText := providersession.UserInputResponse{Correlation: textPrompt.Correlation, PromptRef: textPrompt.PromptRef, Text: "safe"}
	cases := []struct {
		name     string
		prompt   providersession.UserInputPrompt
		response providersession.UserInputResponse
	}{
		{name: "unknown_option", prompt: selectPrompt, response: func() providersession.UserInputResponse {
			r := cloneUserInputResponse(baseSelect)
			r.SelectedOptionRefs[0] = "unknown"
			return r
		}()},
		{name: "duplicate_option", prompt: selectPrompt, response: providersession.UserInputResponse{Correlation: selectPrompt.Correlation, PromptRef: selectPrompt.PromptRef, SelectedOptionRefs: []string{"option-a", "option-a"}}},
		{name: "over_limit", prompt: func() providersession.UserInputPrompt {
			p := cloneUserInputPrompt(selectPrompt)
			p.MaxSelections = 1
			return p
		}(), response: providersession.UserInputResponse{Correlation: selectPrompt.Correlation, PromptRef: selectPrompt.PromptRef, SelectedOptionRefs: []string{"option-a", "option-c"}}},
		{name: "missing_answer", prompt: selectPrompt, response: providersession.UserInputResponse{Correlation: selectPrompt.Correlation, PromptRef: selectPrompt.PromptRef}},
		{name: "kind_mismatch", prompt: textPrompt, response: providersession.UserInputResponse{Correlation: textPrompt.Correlation, PromptRef: textPrompt.PromptRef, SelectedOptionRefs: []string{"option-a"}}},
		{name: "empty_text", prompt: textPrompt, response: func() providersession.UserInputResponse { r := baseText; r.Text = "   "; return r }()},
		{name: "oversized_text", prompt: textPrompt, response: func() providersession.UserInputResponse {
			r := baseText
			r.Text = strings.Repeat("x", textPrompt.MaxTextBytes+1)
			return r
		}()},
		{name: "nul_text", prompt: textPrompt, response: func() providersession.UserInputResponse { r := baseText; r.Text = "bad\x00text"; return r }()},
		{name: "control_text", prompt: textPrompt, response: func() providersession.UserInputResponse { r := baseText; r.Text = "bad\ntext"; return r }()},
		{name: "prompt_substitution", prompt: selectPrompt, response: func() providersession.UserInputResponse {
			r := cloneUserInputResponse(baseSelect)
			r.PromptRef = "other-prompt"
			return r
		}()},
		{name: "cross_process", prompt: selectPrompt, response: mutateUserInputCorrelation(baseSelect, func(c *providersession.Correlation) { c.ProcessIncarnationID = "other-process" })},
		{name: "cross_connection", prompt: selectPrompt, response: mutateUserInputCorrelation(baseSelect, func(c *providersession.Correlation) { c.ConnectionID = "other-connection" })},
		{name: "cross_session", prompt: selectPrompt, response: mutateUserInputCorrelation(baseSelect, func(c *providersession.Correlation) { c.SessionID = "other-session" })},
		{name: "cross_turn", prompt: selectPrompt, response: mutateUserInputCorrelation(baseSelect, func(c *providersession.Correlation) { c.InteractionID = "other-turn" })},
		{name: "cross_item", prompt: selectPrompt, response: mutateUserInputCorrelation(baseSelect, func(c *providersession.Correlation) { c.ActivityID = "other-item" })},
		{name: "cross_request", prompt: selectPrompt, response: mutateUserInputCorrelation(baseSelect, func(c *providersession.Correlation) { c.RequestID = "other-request" })},
		{name: "cross_decision", prompt: selectPrompt, response: mutateUserInputCorrelation(baseSelect, func(c *providersession.Correlation) { c.DecisionID = "other-decision" })},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			controller := newTransientUserInputController()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			go func() { _, _ = controller.awaitResponse(ctx, tc.prompt) }()
			waitForControllerUserInputPrompt(t, controller, tc.prompt)
			if err := controller.submit(ctx, tc.response); err == nil {
				t.Fatal("invalid response was accepted")
			}
			waitForControllerUserInputPrompt(t, controller, tc.prompt)
			controller.close(errors.New("fixture done"))
		})
	}
}

func TestUserInputControllerRejectsCrossServerAndPostShutdownResponses(t *testing.T) {
	enableMCPExecTier(t)
	server := NewServer("one")
	other := NewServer("other")
	active, ok := server.beginActiveProviderPoolChat(context.Background())
	if !ok {
		t.Fatal("failed to begin fixture chat")
	}
	prompt := userInputFixturePrompt("server", providersession.UserInputPromptText)
	go func() {
		_, _ = active.inputs.awaitResponse(active.ctx, prompt)
		server.finishActiveProviderPoolChat(active)
	}()
	waitForUserInputPrompt(t, server, prompt)
	response := providersession.UserInputResponse{Correlation: prompt.Correlation, PromptRef: prompt.PromptRef, Text: "safe"}
	if err := other.submitActiveUserInputResponse(context.Background(), response); err == nil || !strings.Contains(err.Error(), "no provider-pool chat") {
		t.Fatalf("cross-server error=%v", err)
	}
	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	invalidUTF8 := bytes.Replace(encoded, []byte("safe"), []byte{0xff}, 1)
	if _, _, err := server.dispatchTool(context.Background(), "dorkpipe.provider_pool_user_input_respond", invalidUTF8); err == nil || !strings.Contains(err.Error(), "UTF-8") {
		t.Fatalf("invalid UTF-8 error=%v", err)
	}
	waitForUserInputPrompt(t, server, prompt)
	server.cancelAndWaitForActiveProviderPoolChat()
	if err := server.submitActiveUserInputResponse(context.Background(), response); err == nil || !strings.Contains(err.Error(), "no provider-pool chat") {
		t.Fatalf("post-shutdown error=%v", err)
	}
}

func TestCancelledUserInputResponseCancelsAndJoinsOwnedChat(t *testing.T) {
	enableMCPExecTier(t)
	server := NewServer("cancelled-response")
	active, ok := server.beginActiveProviderPoolChat(context.Background())
	if !ok {
		t.Fatal("failed to begin fixture chat")
	}
	prompt := userInputFixturePrompt("cancelled-response", providersession.UserInputPromptText)
	go func() {
		_, _ = active.inputs.awaitResponse(active.ctx, prompt)
		server.finishActiveProviderPoolChat(active)
	}()
	waitForUserInputPrompt(t, server, prompt)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	response := providersession.UserInputResponse{Correlation: prompt.Correlation, PromptRef: prompt.PromptRef, Text: "safe"}
	if err := server.submitActiveUserInputResponse(ctx, response); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled response error=%v", err)
	}
	if _, err := server.activeUserInputPrompt(); err == nil {
		t.Fatal("cancelled response left an active prompt")
	}
}

func TestHTTPResponseWriteFailureCancelsAndJoinsOwnedUserInputChat(t *testing.T) {
	enableMCPExecTier(t)
	server := NewServer("http-write-failure")
	active, ok := server.beginActiveProviderPoolChat(context.Background())
	if !ok {
		t.Fatal("failed to begin fixture chat")
	}
	prompt := userInputFixturePrompt("http-write-failure", providersession.UserInputPromptText)
	go func() {
		delivery, err := active.inputs.awaitResponse(active.ctx, prompt)
		if err == nil {
			delivery.complete(nil)
			<-active.ctx.Done()
		}
		server.finishActiveProviderPoolChat(active)
	}()
	waitForUserInputPrompt(t, server, prompt)
	response := providersession.UserInputResponse{Correlation: prompt.Correlation, PromptRef: prompt.PromptRef, Text: "safe"}
	request := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(userInputResponseFixture("1", response)))
	request.Header.Set("Content-Type", "application/json")
	server.jsonRPCHandler(io.Discard).ServeHTTP(fixtureHTTPErrorWriter{header: make(http.Header)}, request)
	if _, err := server.activeUserInputPrompt(); err == nil {
		t.Fatal("HTTP response write failure left an active prompt")
	}
}

func TestRelayInteractiveFramesSeparatesOrdinaryStderrAndRequestClasses(t *testing.T) {
	inputs := newTransientUserInputController()
	approvals := newTransientApprovalController()
	prompt := userInputFixturePrompt("relay", providersession.UserInputPromptText)
	response := providersession.UserInputResponse{Correlation: prompt.Correlation, PromptRef: prompt.PromptRef, Text: "safe"}
	encoded, err := json.Marshal(privateUserInputPromptFrame{Type: "user_input_prompt", Prompt: prompt})
	if err != nil {
		t.Fatal(err)
	}
	fromChildReader, fromChildWriter := io.Pipe()
	var toChild bytes.Buffer
	relayDone := make(chan struct {
		stderr string
		err    error
	}, 1)
	go func() {
		stderr, relayErr := relayInteractiveFrames(context.Background(), fromChildReader, &toChild, approvals, inputs, newTransientCancellationController())
		relayDone <- struct {
			stderr string
			err    error
		}{stderr: stderr, err: relayErr}
	}()
	if _, err := fmt.Fprintf(fromChildWriter, "ordinary stderr\n%s%s\n", privateUserInputFramePrefix, encoded); err != nil {
		t.Fatal(err)
	}
	waitForControllerUserInputPrompt(t, inputs, prompt)
	if _, err := approvals.pendingRequest(); err == nil {
		t.Fatal("approval controller observed a user-input frame")
	}
	if err := inputs.submit(context.Background(), response); err != nil {
		t.Fatal(err)
	}
	if err := fromChildWriter.Close(); err != nil {
		t.Fatal(err)
	}
	result := <-relayDone
	if result.err != nil || result.stderr != "ordinary stderr\n" {
		t.Fatalf("stderr=%q err=%v", result.stderr, result.err)
	}
	line := strings.TrimSpace(toChild.String())
	if !strings.HasPrefix(line, privateUserInputFramePrefix) || strings.Contains(line, prompt.Summary) || strings.Contains(line, "ordinary stderr") {
		t.Fatalf("response frame=%q", line)
	}
	var frame privateUserInputResponseFrame
	if err := decodeClosedJSON([]byte(strings.TrimPrefix(line, privateUserInputFramePrefix)), &frame); err != nil || frame.Type != "user_input_response" || !reflect.DeepEqual(frame.Response, response) {
		t.Fatalf("response frame=%+v err=%v", frame, err)
	}
}

func TestRelayInteractiveFramesPreservesApprovalTransportAndIsolation(t *testing.T) {
	approvals := newTransientApprovalController()
	inputs := newTransientUserInputController()
	request := approvalFixtureRequest("combined-relay")
	decision := providersession.ApprovalDecision{Correlation: request.Correlation, Decision: providersession.DecisionApprove}
	encoded, err := json.Marshal(privateApprovalRequestFrame{Type: "approval_request", Request: request})
	if err != nil {
		t.Fatal(err)
	}
	fromChildReader, fromChildWriter := io.Pipe()
	var toChild bytes.Buffer
	relayDone := make(chan error, 1)
	go func() {
		_, relayErr := relayInteractiveFrames(context.Background(), fromChildReader, &toChild, approvals, inputs, newTransientCancellationController())
		relayDone <- relayErr
	}()
	if _, err := fmt.Fprintf(fromChildWriter, "%s%s\n", privateApprovalFramePrefix, encoded); err != nil {
		t.Fatal(err)
	}
	waitForControllerRequest(t, approvals, request)
	if _, err := inputs.pendingPrompt(); err == nil {
		t.Fatal("user-input controller observed an approval frame")
	}
	if err := approvals.submit(context.Background(), decision); err != nil {
		t.Fatal(err)
	}
	if err := fromChildWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-relayDone; err != nil {
		t.Fatal(err)
	}
	line := strings.TrimSpace(toChild.String())
	if !strings.HasPrefix(line, privateApprovalFramePrefix) || strings.Contains(line, privateUserInputFramePrefix) {
		t.Fatalf("approval decision frame=%q", line)
	}
	var frame privateApprovalDecisionFrame
	if err := decodeClosedJSON([]byte(strings.TrimPrefix(line, privateApprovalFramePrefix)), &frame); err != nil || frame.Type != "approval_decision" || !reflect.DeepEqual(frame.Decision, decision) {
		t.Fatalf("approval decision frame=%+v err=%v", frame, err)
	}
}

func TestRelayInteractiveFramesFailuresInvalidatePendingUserInput(t *testing.T) {
	t.Run("write_failure", func(t *testing.T) {
		inputs := newTransientUserInputController()
		prompt := userInputFixturePrompt("write-failure", providersession.UserInputPromptSelectOne)
		encoded, _ := json.Marshal(privateUserInputPromptFrame{Type: "user_input_prompt", Prompt: prompt})
		relayDone := make(chan error, 1)
		go func() {
			_, err := relayInteractiveFrames(context.Background(), bytes.NewBufferString(privateUserInputFramePrefix+string(encoded)+"\n"), errorWriter{}, newTransientApprovalController(), inputs, newTransientCancellationController())
			relayDone <- err
		}()
		waitForControllerUserInputPrompt(t, inputs, prompt)
		response := providersession.UserInputResponse{Correlation: prompt.Correlation, PromptRef: prompt.PromptRef, SelectedOptionRefs: []string{"option-a"}}
		if err := inputs.submit(context.Background(), response); err == nil || !strings.Contains(err.Error(), "fixture transport failure") {
			t.Fatalf("submit error=%v", err)
		}
		if err := <-relayDone; err == nil || !strings.Contains(err.Error(), "fixture transport failure") {
			t.Fatalf("relay error=%v", err)
		}
	})

	for _, tc := range []struct {
		name  string
		input io.Reader
	}{
		{name: "malformed", input: strings.NewReader(privateUserInputFramePrefix + `{}` + "\n")},
		{name: "unknown_field", input: strings.NewReader(privateUserInputFramePrefix + `{"type":"user_input_prompt","prompt":{},"provider_payload":"forbidden"}` + "\n")},
		{name: "invalid_utf8", input: bytes.NewReader(append(append([]byte(privateUserInputFramePrefix), 0xff), '\n'))},
		{name: "oversized", input: strings.NewReader(privateUserInputFramePrefix + strings.Repeat("x", privateUserInputFrameLimit) + "\n")},
		{name: "read_failure", input: fixtureErrorReader{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := relayInteractiveFrames(context.Background(), tc.input, io.Discard, newTransientApprovalController(), newTransientUserInputController(), newTransientCancellationController()); err == nil {
				t.Fatal("invalid transport input was accepted")
			}
		})
	}
}

func TestServeStdioEOFInvalidatesPendingUserInputAndJoinsChat(t *testing.T) {
	enableMCPExecTier(t)
	server := NewServer("test")
	prompt := userInputFixturePrompt("eof", providersession.UserInputPromptSelectOne)
	joined := make(chan struct{})
	server.providerPoolChatInteractiveStreamRunner = func(ctx context.Context, _ []string, _ *transientApprovalController, inputs *transientUserInputController, _ *transientCancellationController) (string, string, int, error) {
		defer close(joined)
		_, err := inputs.awaitResponse(ctx, prompt)
		return "", "", -1, err
	}
	inReader, inWriter := io.Pipe()
	var output bytes.Buffer
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.ServeStdio(inReader, &output, io.Discard) }()
	writeStdioFixture(t, inWriter, providerPoolChatFixture("20"))
	waitForUserInputPrompt(t, server, prompt)
	if err := inWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := waitServeResult(t, serveDone); err != nil {
		t.Fatal(err)
	}
	waitFixtureSignal(t, joined, "user-input handler join")
	if _, err := server.activeUserInputPrompt(); err == nil {
		t.Fatal("pending prompt survived EOF")
	}
	if output.Len() != 0 {
		t.Fatalf("late response after EOF: %q", output.String())
	}
}

func TestMCPBridgeExposesClosedUserInputOperationsAtExecTier(t *testing.T) {
	want := map[string]bool{
		"dorkpipe.provider_pool_user_input_request": false,
		"dorkpipe.provider_pool_user_input_respond": false,
	}
	for _, tool := range mcpToolCatalog() {
		if _, ok := want[tool.Name]; !ok {
			continue
		}
		if tier, ok := minTierForTool(tool.Name); !ok || tier != TierExec {
			t.Fatalf("tool %q tier=%v present=%v", tool.Name, tier, ok)
		}
		var schema map[string]any
		if err := json.Unmarshal(tool.InputSchema, &schema); err != nil {
			t.Fatal(err)
		}
		if additional, ok := schema["additionalProperties"].(bool); !ok || additional {
			t.Fatalf("tool %q schema is not closed: %s", tool.Name, tool.InputSchema)
		}
		want[tool.Name] = true
	}
	for name, found := range want {
		if !found {
			t.Fatalf("missing user-input tool %q", name)
		}
	}
}

func TestHTTPUserInputReadAndResponseShareTheExactActiveChat(t *testing.T) {
	enableMCPExecTier(t)
	server := NewServer("test")
	prompt := userInputFixturePrompt("http", providersession.UserInputPromptText)
	response := providersession.UserInputResponse{Correlation: prompt.Correlation, PromptRef: prompt.PromptRef, Text: "bounded answer"}
	providerResolved := make(chan struct{})
	var runnerCalls atomic.Int32
	server.providerPoolChatInteractiveStreamRunner = func(ctx context.Context, _ []string, _ *transientApprovalController, inputs *transientUserInputController, _ *transientCancellationController) (string, string, int, error) {
		runnerCalls.Add(1)
		delivery, err := inputs.awaitResponse(ctx, prompt)
		if err != nil {
			return "", "", -1, err
		}
		delivery.complete(nil)
		select {
		case <-providerResolved:
			return `{"state":"ready"}`, "", 0, nil
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
		got, err := callHTTPRPC(srv.URL, providerPoolChatFixture("30"))
		chatDone <- callResult{response: got, err: err}
	}()
	waitForUserInputPrompt(t, server, prompt)
	read, err := callHTTPRPC(srv.URL, userInputReadFixture("31"))
	if err != nil || !reflect.DeepEqual(userInputPromptFromRPCResponse(t, read), prompt) {
		t.Fatalf("read=%+v err=%v", read, err)
	}
	second, err := callHTTPRPC(srv.URL, providerPoolChatFixture("32"))
	if err != nil || second.Error == nil || !strings.Contains(second.Error.Message, "already active") || runnerCalls.Load() != 1 {
		t.Fatalf("second chat=%+v err=%v calls=%d", second, err, runnerCalls.Load())
	}
	respond, err := callHTTPRPC(srv.URL, userInputResponseFixture("33", response))
	if err != nil || respond.Error != nil {
		t.Fatalf("respond=%+v err=%v", respond, err)
	}
	select {
	case result := <-chatDone:
		t.Fatalf("chat completed on delivery: response=%+v err=%v", result.response, result.err)
	default:
	}
	close(providerResolved)
	select {
	case result := <-chatDone:
		if result.err != nil || result.response == nil || result.response.Error != nil {
			t.Fatalf("chat response=%+v err=%v", result.response, result.err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for HTTP chat completion")
	}
}

type fixtureErrorReader struct{}

func (fixtureErrorReader) Read([]byte) (int, error) {
	return 0, errors.New("fixture read failure")
}

type fixtureHTTPErrorWriter struct {
	header http.Header
}

func (w fixtureHTTPErrorWriter) Header() http.Header {
	return w.header
}

func (fixtureHTTPErrorWriter) WriteHeader(int) {}

func (fixtureHTTPErrorWriter) Write([]byte) (int, error) {
	return 0, errors.New("fixture HTTP response failure")
}

func userInputFixturePrompt(suffix string, kind providersession.UserInputPromptKind) providersession.UserInputPrompt {
	request := approvalFixtureRequest("input-" + suffix)
	prompt := providersession.UserInputPrompt{
		Correlation: request.Correlation,
		PromptRef:   "prompt-" + suffix,
		Kind:        kind,
		Summary:     "Bounded neutral prompt",
	}
	switch kind {
	case providersession.UserInputPromptSelectOne:
		prompt.Options = []providersession.UserInputOption{{OptionRef: "option-a", Label: "Option A"}, {OptionRef: "option-b", Label: "Option B"}}
		prompt.MaxSelections = 1
	case providersession.UserInputPromptSelectMany:
		prompt.Options = []providersession.UserInputOption{{OptionRef: "option-a", Label: "Option A"}, {OptionRef: "option-b", Label: "Option B"}, {OptionRef: "option-c", Label: "Option C"}}
		prompt.MaxSelections = 2
	case providersession.UserInputPromptText:
		prompt.MaxTextBytes = 32
	}
	return prompt
}

func mutateUserInputCorrelation(response providersession.UserInputResponse, mutate func(*providersession.Correlation)) providersession.UserInputResponse {
	response = cloneUserInputResponse(response)
	mutate(&response.Correlation)
	return response
}

func waitForUserInputPrompt(t *testing.T, server *Server, want providersession.UserInputPrompt) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		got, err := server.activeUserInputPrompt()
		if err == nil && reflect.DeepEqual(got, want) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed out waiting for active user-input prompt")
}

func waitForControllerUserInputPrompt(t *testing.T, controller *transientUserInputController, want providersession.UserInputPrompt) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		got, err := controller.pendingPrompt()
		if err == nil && reflect.DeepEqual(got, want) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed out waiting for controller user-input prompt")
}

func userInputReadFixture(id string) string {
	return fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"method":"tools/call","params":{"name":"dorkpipe.provider_pool_user_input_request","arguments":{}}}`, id)
}

func userInputResponseFixture(id string, response providersession.UserInputResponse) string {
	encoded, _ := json.Marshal(response)
	return fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"method":"tools/call","params":{"name":"dorkpipe.provider_pool_user_input_respond","arguments":%s}}`, id, encoded)
}

func userInputPromptFromToolResponse(t *testing.T, body []byte) providersession.UserInputPrompt {
	t.Helper()
	var response rpcResponse
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatal(err)
	}
	return userInputPromptFromRPCResponse(t, &response)
}

func userInputPromptFromRPCResponse(t *testing.T, response *rpcResponse) providersession.UserInputPrompt {
	t.Helper()
	if response == nil || response.Error != nil {
		t.Fatalf("user-input response=%+v", response)
	}
	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(response.Result, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Content) != 1 {
		t.Fatalf("user-input result=%s", response.Result)
	}
	var prompt providersession.UserInputPrompt
	if err := json.Unmarshal([]byte(result.Content[0].Text), &prompt); err != nil {
		t.Fatal(err)
	}
	return prompt
}
