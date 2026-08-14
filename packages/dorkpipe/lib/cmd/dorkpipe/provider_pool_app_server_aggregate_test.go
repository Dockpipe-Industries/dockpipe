package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dorkpipe.orchestrator/statepaths"
)

func providerPoolAppServerAggregateFingerprint(character string) string {
	return "sha256:" + strings.Repeat(character, 64)
}

func providerPoolAppServerAggregateFixture() providerPoolAppServerAggregate {
	preState := providerPoolAppServerAggregateFingerprint("1")
	observation := providerPoolAppServerAggregateFingerprint("2")
	return providerPoolAppServerAggregate{
		Schema:                   providerPoolAppServerAggregateSchema,
		Version:                  providerPoolAppServerAggregateVersion,
		Revision:                 1,
		PipeonSessionID:          "pipeon-session-1",
		Adapter:                  providerPoolCodexAppServerAdapter,
		ProviderSessionID:        "provider-session-1",
		RecoveryEvidenceRef:      "recovery-evidence-1",
		Model:                    "gpt-5.6-terra",
		ReasoningEffort:          "high",
		LifecycleState:           providerPoolAppServerAggregateLifecycleState,
		OutcomeUnknown:           true,
		ReconciledToVerifiedIdle: true,
		TerminalOutcome:          false,
		LastCompletedTurn:        7,
		UnknownPendingTurn:       8,
		TurnHighWaterMark:        8,
		PreStateFingerprint:      preState,
		RecoveryObservation: providerPoolAppServerRecoveryBinding{
			Fingerprint:         observation,
			PreStateFingerprint: preState,
		},
		Reconciliation: providerPoolAppServerReconciliationBinding{
			Fingerprint:                    providerPoolAppServerAggregateFingerprint("3"),
			PreStateFingerprint:            preState,
			RecoveryObservationFingerprint: observation,
		},
		UnresolvedClaimConsumed:     true,
		RecoveryObservationConsumed: true,
		PermanentReplayForbidden:    true,
		UserDecision: providerPoolAppServerAggregateUserDecision{
			State: providerPoolAppServerDecisionRequired,
		},
	}
}

func TestProviderPoolAppServerAggregateValidInitialCanonicalRoundTrip(t *testing.T) {
	aggregate := providerPoolAppServerAggregateFixture()
	raw, err := encodeProviderPoolAppServerAggregateCanonical(aggregate, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"schema":"dorkpipe.provider-pool.app-server-lifecycle-aggregate","version":1,"revision":1,"pipeon_session_id":"pipeon-session-1","adapter":"codex_app_server","provider_session_id":"provider-session-1","recovery_evidence_ref":"recovery-evidence-1","model":"gpt-5.6-terra","reasoning_effort":"high","lifecycle_state":"reconciled_outcome_unknown","outcome_unknown":true,"reconciled_to_verified_idle":true,"terminal_outcome":false,"last_completed_turn":7,"unknown_pending_turn":8,"turn_high_water_mark":8,"pre_state_fingerprint":"sha256:1111111111111111111111111111111111111111111111111111111111111111","recovery_observation":{"fingerprint":"sha256:2222222222222222222222222222222222222222222222222222222222222222","pre_state_fingerprint":"sha256:1111111111111111111111111111111111111111111111111111111111111111"},"reconciliation":{"fingerprint":"sha256:3333333333333333333333333333333333333333333333333333333333333333","pre_state_fingerprint":"sha256:1111111111111111111111111111111111111111111111111111111111111111","recovery_observation_fingerprint":"sha256:2222222222222222222222222222222222222222222222222222222222222222"},"unresolved_claim_consumed":true,"recovery_observation_consumed":true,"permanent_replay_forbidden":true,"user_decision":{"state":"required","bound_revision":0,"bound_reconciliation_fingerprint":"","decision_fingerprint":"","consumed":false,"consumed_revision":0,"consumed_turn":0}}` + "\n"
	if string(raw) != want {
		t.Fatalf("canonical aggregate mismatch\n got: %s\nwant: %s", raw, want)
	}
	decoded, err := decodeProviderPoolAppServerAggregateCanonical(raw, aggregate.PipeonSessionID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if decoded != aggregate {
		t.Fatalf("round trip changed aggregate: got %+v want %+v", decoded, aggregate)
	}
	again, err := encodeProviderPoolAppServerAggregateCanonical(decoded, 0)
	if err != nil || !bytes.Equal(again, raw) {
		t.Fatalf("canonical encoding is not deterministic: error=%v\nfirst=%q\nsecond=%q", err, raw, again)
	}
}

func TestProviderPoolAppServerAggregateRejectsUnknownMissingAndTrailingData(t *testing.T) {
	aggregate := providerPoolAppServerAggregateFixture()
	raw, err := encodeProviderPoolAppServerAggregateCanonical(aggregate, 0)
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string][]byte{
		"unknown field":  append(append([]byte{}, raw[:len(raw)-2]...), []byte(",\"extension\":true}\n")...),
		"missing field":  bytes.Replace(raw, []byte(`"terminal_outcome":false,`), nil, 1),
		"trailing value": append(append([]byte{}, raw...), []byte(`{}`)...),
		"second newline": append(append([]byte{}, raw...), '\n'),
		"no newline":     bytes.TrimSuffix(raw, []byte("\n")),
	}
	for name, candidate := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeProviderPoolAppServerAggregateCanonical(candidate, aggregate.PipeonSessionID, 0); err == nil {
				t.Fatalf("non-canonical aggregate was accepted: %q", candidate)
			}
		})
	}
}

func TestProviderPoolAppServerAggregateRejectsIdentityAndSchemaSubstitution(t *testing.T) {
	invalidUTF8 := string([]byte{0xff})
	cases := []struct {
		name   string
		mutate func(*providerPoolAppServerAggregate)
	}{
		{name: "schema", mutate: func(a *providerPoolAppServerAggregate) { a.Schema = "unknown" }},
		{name: "version", mutate: func(a *providerPoolAppServerAggregate) { a.Version = 2 }},
		{name: "adapter", mutate: func(a *providerPoolAppServerAggregate) { a.Adapter = providerPoolCodexExecAdapter }},
		{name: "empty session", mutate: func(a *providerPoolAppServerAggregate) { a.PipeonSessionID = "" }},
		{name: "session leading space", mutate: func(a *providerPoolAppServerAggregate) { a.PipeonSessionID = " session" }},
		{name: "session embedded space", mutate: func(a *providerPoolAppServerAggregate) { a.PipeonSessionID = "session id" }},
		{name: "session control", mutate: func(a *providerPoolAppServerAggregate) { a.PipeonSessionID = "session\n" }},
		{name: "session invalid utf8", mutate: func(a *providerPoolAppServerAggregate) { a.PipeonSessionID = invalidUTF8 }},
		{name: "session oversized", mutate: func(a *providerPoolAppServerAggregate) { a.PipeonSessionID = strings.Repeat("s", 257) }},
		{name: "provider session", mutate: func(a *providerPoolAppServerAggregate) { a.ProviderSessionID = " provider" }},
		{name: "recovery evidence", mutate: func(a *providerPoolAppServerAggregate) { a.RecoveryEvidenceRef = "" }},
		{name: "model", mutate: func(a *providerPoolAppServerAggregate) { a.Model = "gpt model" }},
		{name: "reasoning", mutate: func(a *providerPoolAppServerAggregate) { a.ReasoningEffort = strings.Repeat("h", 257) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			aggregate := providerPoolAppServerAggregateFixture()
			tc.mutate(&aggregate)
			if _, err := encodeProviderPoolAppServerAggregateCanonical(aggregate, 0); err == nil {
				t.Fatal("substituted identity or schema was accepted")
			}
		})
	}
}

func TestProviderPoolAppServerAggregateRejectsRevisionAndTurnInvariants(t *testing.T) {
	cases := []struct {
		name     string
		previous uint64
		mutate   func(*providerPoolAppServerAggregate)
	}{
		{name: "zero revision", mutate: func(a *providerPoolAppServerAggregate) { a.Revision = 0 }},
		{name: "regressing revision", previous: 1},
		{name: "zero completed", mutate: func(a *providerPoolAppServerAggregate) {
			a.LastCompletedTurn = 0
			a.UnknownPendingTurn = 1
			a.TurnHighWaterMark = 1
		}},
		{name: "completed overflow", mutate: func(a *providerPoolAppServerAggregate) {
			a.LastCompletedTurn = ^uint64(0)
			a.UnknownPendingTurn = 0
			a.TurnHighWaterMark = 0
		}},
		{name: "nonconsecutive pending", mutate: func(a *providerPoolAppServerAggregate) { a.UnknownPendingTurn = 9; a.TurnHighWaterMark = 9 }},
		{name: "high water below pending", mutate: func(a *providerPoolAppServerAggregate) { a.TurnHighWaterMark = 7 }},
		{name: "unclaimed high water", mutate: func(a *providerPoolAppServerAggregate) { a.TurnHighWaterMark = 9 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			aggregate := providerPoolAppServerAggregateFixture()
			if tc.mutate != nil {
				tc.mutate(&aggregate)
			}
			if _, err := encodeProviderPoolAppServerAggregateCanonical(aggregate, tc.previous); err == nil {
				t.Fatal("invalid revision or turn relationship was accepted")
			}
		})
	}
}

func TestProviderPoolAppServerAggregateRequiresUnknownIdleConsumedNoReplayState(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*providerPoolAppServerAggregate)
	}{
		{name: "unknown lifecycle", mutate: func(a *providerPoolAppServerAggregate) { a.LifecycleState = "completed" }},
		{name: "outcome classified", mutate: func(a *providerPoolAppServerAggregate) { a.OutcomeUnknown = false }},
		{name: "idle not reconciled", mutate: func(a *providerPoolAppServerAggregate) { a.ReconciledToVerifiedIdle = false }},
		{name: "terminal implication", mutate: func(a *providerPoolAppServerAggregate) { a.TerminalOutcome = true }},
		{name: "claim unconsumed", mutate: func(a *providerPoolAppServerAggregate) { a.UnresolvedClaimConsumed = false }},
		{name: "observation unconsumed", mutate: func(a *providerPoolAppServerAggregate) { a.RecoveryObservationConsumed = false }},
		{name: "replay permitted", mutate: func(a *providerPoolAppServerAggregate) { a.PermanentReplayForbidden = false }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			aggregate := providerPoolAppServerAggregateFixture()
			tc.mutate(&aggregate)
			if _, err := encodeProviderPoolAppServerAggregateCanonical(aggregate, 0); err == nil {
				t.Fatal("weakened reconciled state was accepted")
			}
		})
	}
}

func TestProviderPoolAppServerAggregateRequiresExactFingerprintSyntaxAndBindings(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*providerPoolAppServerAggregate)
	}{
		{name: "algorithm missing", mutate: func(a *providerPoolAppServerAggregate) { a.PreStateFingerprint = strings.Repeat("1", 64) }},
		{name: "uppercase digest", mutate: func(a *providerPoolAppServerAggregate) {
			a.RecoveryObservation.Fingerprint = providerPoolAppServerAggregateFingerprint("A")
		}},
		{name: "short digest", mutate: func(a *providerPoolAppServerAggregate) { a.Reconciliation.Fingerprint = "sha256:1234" }},
		{name: "observation prestate mismatch", mutate: func(a *providerPoolAppServerAggregate) {
			a.RecoveryObservation.PreStateFingerprint = providerPoolAppServerAggregateFingerprint("4")
		}},
		{name: "reconciliation prestate mismatch", mutate: func(a *providerPoolAppServerAggregate) {
			a.Reconciliation.PreStateFingerprint = providerPoolAppServerAggregateFingerprint("4")
		}},
		{name: "observation mismatch", mutate: func(a *providerPoolAppServerAggregate) {
			a.Reconciliation.RecoveryObservationFingerprint = providerPoolAppServerAggregateFingerprint("4")
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			aggregate := providerPoolAppServerAggregateFixture()
			tc.mutate(&aggregate)
			if _, err := encodeProviderPoolAppServerAggregateCanonical(aggregate, 0); err == nil {
				t.Fatal("invalid fingerprint syntax or binding was accepted")
			}
		})
	}
}

func TestProviderPoolAppServerAggregateValidDecisionShapes(t *testing.T) {
	required := providerPoolAppServerAggregateFixture()
	accepted := providerPoolAppServerAggregateFixture()
	accepted.Revision = 2
	accepted.UserDecision = providerPoolAppServerAggregateUserDecision{
		State:                          providerPoolAppServerDecisionAccepted,
		BoundRevision:                  1,
		BoundReconciliationFingerprint: accepted.Reconciliation.Fingerprint,
		DecisionFingerprint:            providerPoolAppServerAggregateFingerprint("4"),
	}
	consumed := accepted
	consumed.Revision = 3
	consumed.TurnHighWaterMark = 9
	consumed.UserDecision.State = providerPoolAppServerDecisionConsumed
	consumed.UserDecision.Consumed = true
	consumed.UserDecision.ConsumedRevision = 3
	consumed.UserDecision.ConsumedTurn = 9
	for name, aggregate := range map[string]providerPoolAppServerAggregate{"required": required, "accepted": accepted, "consumed": consumed} {
		t.Run(name, func(t *testing.T) {
			if _, err := encodeProviderPoolAppServerAggregateCanonical(aggregate, 0); err != nil {
				t.Fatalf("valid decision shape rejected: %v", err)
			}
		})
	}
}

func TestProviderPoolAppServerAggregateRejectsInvalidDecisionBindings(t *testing.T) {
	accepted := providerPoolAppServerAggregateFixture()
	accepted.Revision = 2
	accepted.UserDecision = providerPoolAppServerAggregateUserDecision{State: providerPoolAppServerDecisionAccepted, BoundRevision: 1, BoundReconciliationFingerprint: accepted.Reconciliation.Fingerprint, DecisionFingerprint: providerPoolAppServerAggregateFingerprint("4")}
	consumed := accepted
	consumed.Revision = 3
	consumed.TurnHighWaterMark = 9
	consumed.UserDecision.State = providerPoolAppServerDecisionConsumed
	consumed.UserDecision.Consumed = true
	consumed.UserDecision.ConsumedRevision = 3
	consumed.UserDecision.ConsumedTurn = 9
	cases := []struct {
		name      string
		aggregate providerPoolAppServerAggregate
		mutate    func(*providerPoolAppServerAggregate)
	}{
		{name: "unknown state", aggregate: providerPoolAppServerAggregateFixture(), mutate: func(a *providerPoolAppServerAggregate) { a.UserDecision.State = "unknown" }},
		{name: "required carries revision", aggregate: providerPoolAppServerAggregateFixture(), mutate: func(a *providerPoolAppServerAggregate) { a.UserDecision.BoundRevision = 1 }},
		{name: "accepted stale revision", aggregate: accepted, mutate: func(a *providerPoolAppServerAggregate) { a.UserDecision.BoundRevision = 0 }},
		{name: "accepted mismatched fingerprint", aggregate: accepted, mutate: func(a *providerPoolAppServerAggregate) {
			a.UserDecision.BoundReconciliationFingerprint = providerPoolAppServerAggregateFingerprint("5")
		}},
		{name: "accepted already consumed", aggregate: accepted, mutate: func(a *providerPoolAppServerAggregate) { a.UserDecision.Consumed = true }},
		{name: "consumed flag missing", aggregate: consumed, mutate: func(a *providerPoolAppServerAggregate) { a.UserDecision.Consumed = false }},
		{name: "consumed revision stale", aggregate: consumed, mutate: func(a *providerPoolAppServerAggregate) { a.UserDecision.ConsumedRevision = 2 }},
		{name: "consumed turn unknown", aggregate: consumed, mutate: func(a *providerPoolAppServerAggregate) { a.UserDecision.ConsumedTurn = 8; a.TurnHighWaterMark = 8 }},
		{name: "consumed high water mismatch", aggregate: consumed, mutate: func(a *providerPoolAppServerAggregate) { a.TurnHighWaterMark = 10 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			aggregate := tc.aggregate
			tc.mutate(&aggregate)
			if _, err := encodeProviderPoolAppServerAggregateCanonical(aggregate, 0); err == nil {
				t.Fatal("invalid decision shape was accepted")
			}
		})
	}
}

func TestProviderPoolAppServerAggregateRejectsMalformedOversizedAndWrongSessionBytes(t *testing.T) {
	aggregate := providerPoolAppServerAggregateFixture()
	raw, err := encodeProviderPoolAppServerAggregateCanonical(aggregate, 0)
	if err != nil {
		t.Fatal(err)
	}
	for name, candidate := range map[string][]byte{
		"empty":     nil,
		"malformed": []byte("{\n"),
		"oversized": bytes.Repeat([]byte("x"), providerPoolAppServerAggregateMaxBytes+1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeProviderPoolAppServerAggregateCanonical(candidate, aggregate.PipeonSessionID, 0); err == nil {
				t.Fatal("invalid aggregate bytes were accepted")
			}
		})
	}
	if _, err := decodeProviderPoolAppServerAggregateCanonical(raw, "other-session", 0); err == nil {
		t.Fatal("aggregate bytes were accepted for a substituted session")
	}
}

func TestProviderPoolAppServerAggregateBoundedLoaderAndExactPathBinding(t *testing.T) {
	writeFixture := func(t *testing.T, root, sessionID string, raw []byte) string {
		t.Helper()
		path, err := statepaths.ProviderPoolAppServerAggregatePath(root, sessionID)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}

	t.Run("valid regular file", func(t *testing.T) {
		root := t.TempDir()
		aggregate := providerPoolAppServerAggregateFixture()
		raw, err := encodeProviderPoolAppServerAggregateCanonical(aggregate, 0)
		if err != nil {
			t.Fatal(err)
		}
		writeFixture(t, root, aggregate.PipeonSessionID, raw)
		loaded, err := loadProviderPoolAppServerAggregate(root, aggregate.PipeonSessionID, 0)
		if err != nil || loaded != aggregate {
			t.Fatalf("bounded load failed: aggregate=%+v error=%v", loaded, err)
		}
	})

	t.Run("bytes at substituted session path", func(t *testing.T) {
		root := t.TempDir()
		aggregate := providerPoolAppServerAggregateFixture()
		raw, err := encodeProviderPoolAppServerAggregateCanonical(aggregate, 0)
		if err != nil {
			t.Fatal(err)
		}
		writeFixture(t, root, "other-session", raw)
		if _, err := loadProviderPoolAppServerAggregate(root, "other-session", 0); err == nil {
			t.Fatal("aggregate was accepted from another session's deterministic path")
		}
	})

	t.Run("oversized regular file", func(t *testing.T) {
		root := t.TempDir()
		aggregate := providerPoolAppServerAggregateFixture()
		writeFixture(t, root, aggregate.PipeonSessionID, bytes.Repeat([]byte("x"), providerPoolAppServerAggregateMaxBytes+1))
		if _, err := loadProviderPoolAppServerAggregate(root, aggregate.PipeonSessionID, 0); err == nil {
			t.Fatal("oversized aggregate file was accepted")
		}
	})

	t.Run("non regular directory", func(t *testing.T) {
		root := t.TempDir()
		aggregate := providerPoolAppServerAggregateFixture()
		path, err := statepaths.ProviderPoolAppServerAggregatePath(root, aggregate.PipeonSessionID)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := loadProviderPoolAppServerAggregate(root, aggregate.PipeonSessionID, 0); err == nil {
			t.Fatal("non-regular aggregate path was accepted")
		}
	})
}
