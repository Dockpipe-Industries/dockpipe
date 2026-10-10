package appserversupervisor

import (
	"context"
	"strings"
	"testing"

	"dorkpipe.orchestrator/providersession"
)

func TestCAS14BaselineOneShotPromptReturnsOnlyCompletedTransientText(t *testing.T) {
	s, child, scanner, initialPolicy := initializedUnselectedLifecycle(t)
	selected := make(chan struct {
		policy providersession.EffectivePolicySnapshot
		err    error
	}, 1)
	go func() {
		policy, err := s.SelectBaselinePolicy(context.Background(), PinnedModel, PinnedReasoningEffort)
		selected <- struct {
			policy providersession.EffectivePolicySnapshot
			err    error
		}{policy: policy, err: err}
	}()
	_ = lifecycleRequest(t, scanner, "model/list", 2)
	_, _ = child.stdoutW.Write([]byte(response(2, modelCatalogFixture)))
	effective := <-selected
	if effective.err != nil {
		t.Fatal(effective.err)
	}
	if effective.policy.EffectiveModelRef != PinnedModel || effective.policy.EffectiveReasoningRef != PinnedReasoningEffort || effective.policy.Approval.EffectiveRef != humanReviewPolicyRef || effective.policy.Sandbox.EffectiveRef != workspaceWritePolicyRef {
		t.Fatalf("baseline effective policy = %+v", effective.policy)
	}
	if len(effective.policy.Capabilities) != 1 || effective.policy.Capabilities[0].CapabilityRef != requestAttestationCapabilityRef || effective.policy.Capabilities[0].UserEnabled {
		t.Fatalf("baseline capabilities = %+v", effective.policy.Capabilities)
	}

	policy, err := BaselineLifecyclePolicy(initialPolicy.Workspace, PinnedModel, PinnedReasoningEffort)
	if err != nil {
		t.Fatal(err)
	}
	threadDone := make(chan struct {
		ref LifecycleReference
		err error
	}, 1)
	go func() {
		ref, err := s.StartThread(context.Background(), policy)
		threadDone <- struct {
			ref LifecycleReference
			err error
		}{ref: ref, err: err}
	}()
	request := lifecycleRequest(t, scanner, "thread/start", 3)
	assertSelectedPolicy(t, request, policy)
	_, _ = child.stdoutW.Write([]byte(response(3, `{"thread":{"id":"thread-1"}}`)))
	thread := <-threadDone
	if thread.err != nil {
		t.Fatal(thread.err)
	}

	prompt := "Give one short fixture answer.\nDo not use tools."
	turnDone := make(chan struct {
		ref LifecycleReference
		err error
	}, 1)
	go func() {
		ref, err := s.StartPromptTurn(context.Background(), thread.ref, policy, prompt)
		turnDone <- struct {
			ref LifecycleReference
			err error
		}{ref: ref, err: err}
	}()
	request = lifecycleRequest(t, scanner, "turn/start", 4)
	params := requestParams(t, request)
	input, ok := params["input"].([]any)
	if !ok || len(input) != 1 || input[0].(map[string]any)["text"] != prompt {
		t.Fatalf("turn input = %#v", params["input"])
	}
	_, _ = child.stdoutW.Write([]byte(response(4, `{"thread":{"id":"thread-1"},"turn":{"id":"turn-1","status":"inProgress"}}`)))
	turn := <-turnDone
	if turn.err != nil {
		t.Fatal(turn.err)
	}

	sendNotification(t, child, "turn/started", `{"threadId":"thread-1","turn":{"id":"turn-1","status":"inProgress"}}`)
	sendNotification(t, child, "item/started", `{"threadId":"thread-1","turnId":"turn-1","item":{"id":"item-1","type":"agentMessage","status":"inProgress"}}`)
	sendNotification(t, child, "item/completed", `{"threadId":"thread-1","turnId":"turn-1","item":{"id":"item-1","type":"agentMessage","status":"completed","text":"fixture answer"}}`)
	sendNotification(t, child, "turn/completed", `{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed"}}`)

	for {
		event := nextEvent(t, s)
		if strings.Contains(event.Summary, "fixture answer") || strings.Contains(event.Summary, prompt) {
			t.Fatalf("transient content leaked into neutral event: %+v", event)
		}
		if event.Summary == "turn_completed" {
			break
		}
	}
	if text, ok := s.CompletedTurnText(); !ok || text != "fixture answer" {
		t.Fatalf("completed text = %q, %t", text, ok)
	}
	s.disconnect(DisconnectShutdown)
	if text, ok := s.CompletedTurnText(); ok || text != "" {
		t.Fatalf("disconnected text = %q, %t", text, ok)
	}
}
