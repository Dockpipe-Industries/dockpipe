package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"

	"dorkpipe.orchestrator/statepaths"
)

const (
	providerPoolAppServerAggregateSchema           = "dorkpipe.provider-pool.app-server-lifecycle-aggregate"
	providerPoolAppServerAggregateVersion          = uint64(1)
	providerPoolAppServerAggregateMaxBytes         = 16 * 1024
	providerPoolAppServerAggregateMaxIdentityBytes = 256
	providerPoolAppServerAggregateLifecycleState   = "reconciled_outcome_unknown"
	providerPoolAppServerDecisionRequired          = "required"
	providerPoolAppServerDecisionAccepted          = "accepted"
	providerPoolAppServerDecisionConsumed          = "consumed"
)

type providerPoolAppServerAggregate struct {
	Schema                      string                                     `json:"schema"`
	Version                     uint64                                     `json:"version"`
	Revision                    uint64                                     `json:"revision"`
	PipeonSessionID             string                                     `json:"pipeon_session_id"`
	Adapter                     string                                     `json:"adapter"`
	ProviderSessionID           string                                     `json:"provider_session_id"`
	RecoveryEvidenceRef         string                                     `json:"recovery_evidence_ref"`
	Model                       string                                     `json:"model"`
	ReasoningEffort             string                                     `json:"reasoning_effort"`
	LifecycleState              string                                     `json:"lifecycle_state"`
	OutcomeUnknown              bool                                       `json:"outcome_unknown"`
	ReconciledToVerifiedIdle    bool                                       `json:"reconciled_to_verified_idle"`
	TerminalOutcome             bool                                       `json:"terminal_outcome"`
	LastCompletedTurn           uint64                                     `json:"last_completed_turn"`
	UnknownPendingTurn          uint64                                     `json:"unknown_pending_turn"`
	TurnHighWaterMark           uint64                                     `json:"turn_high_water_mark"`
	PreStateFingerprint         string                                     `json:"pre_state_fingerprint"`
	RecoveryObservation         providerPoolAppServerRecoveryBinding       `json:"recovery_observation"`
	Reconciliation              providerPoolAppServerReconciliationBinding `json:"reconciliation"`
	UnresolvedClaimConsumed     bool                                       `json:"unresolved_claim_consumed"`
	RecoveryObservationConsumed bool                                       `json:"recovery_observation_consumed"`
	PermanentReplayForbidden    bool                                       `json:"permanent_replay_forbidden"`
	UserDecision                providerPoolAppServerAggregateUserDecision `json:"user_decision"`
}

type providerPoolAppServerRecoveryBinding struct {
	Fingerprint         string `json:"fingerprint"`
	PreStateFingerprint string `json:"pre_state_fingerprint"`
}

type providerPoolAppServerReconciliationBinding struct {
	Fingerprint                    string `json:"fingerprint"`
	PreStateFingerprint            string `json:"pre_state_fingerprint"`
	RecoveryObservationFingerprint string `json:"recovery_observation_fingerprint"`
}

type providerPoolAppServerAggregateUserDecision struct {
	State                          string `json:"state"`
	BoundRevision                  uint64 `json:"bound_revision"`
	BoundReconciliationFingerprint string `json:"bound_reconciliation_fingerprint"`
	DecisionFingerprint            string `json:"decision_fingerprint"`
	Consumed                       bool   `json:"consumed"`
	ConsumedRevision               uint64 `json:"consumed_revision"`
	ConsumedTurn                   uint64 `json:"consumed_turn"`
}

func validateProviderPoolAppServerAggregate(aggregate providerPoolAppServerAggregate, previousRevision uint64) error {
	if aggregate.Schema != providerPoolAppServerAggregateSchema || aggregate.Version != providerPoolAppServerAggregateVersion {
		return fmt.Errorf("provider-pool App Server aggregate schema is unsupported")
	}
	if aggregate.Revision == 0 || aggregate.Revision <= previousRevision {
		return fmt.Errorf("provider-pool App Server aggregate revision must advance")
	}
	for label, value := range map[string]string{
		"Pipeon session":    aggregate.PipeonSessionID,
		"provider session":  aggregate.ProviderSessionID,
		"recovery evidence": aggregate.RecoveryEvidenceRef,
		"model":             aggregate.Model,
		"reasoning effort":  aggregate.ReasoningEffort,
	} {
		if !validProviderPoolAppServerAggregateIdentity(value) {
			return fmt.Errorf("provider-pool App Server aggregate %s identity is invalid", label)
		}
	}
	if aggregate.Adapter != providerPoolCodexAppServerAdapter {
		return fmt.Errorf("provider-pool App Server aggregate adapter is unsupported")
	}
	if aggregate.LifecycleState != providerPoolAppServerAggregateLifecycleState || !aggregate.OutcomeUnknown || !aggregate.ReconciledToVerifiedIdle || aggregate.TerminalOutcome {
		return fmt.Errorf("provider-pool App Server aggregate lifecycle state is invalid")
	}
	if aggregate.LastCompletedTurn == 0 || aggregate.LastCompletedTurn == ^uint64(0) || aggregate.UnknownPendingTurn != aggregate.LastCompletedTurn+1 || aggregate.TurnHighWaterMark < aggregate.UnknownPendingTurn {
		return fmt.Errorf("provider-pool App Server aggregate turn boundary is invalid")
	}
	if !validProviderPoolAppServerAggregateFingerprint(aggregate.PreStateFingerprint) ||
		!validProviderPoolAppServerAggregateFingerprint(aggregate.RecoveryObservation.Fingerprint) ||
		!validProviderPoolAppServerAggregateFingerprint(aggregate.Reconciliation.Fingerprint) ||
		aggregate.RecoveryObservation.PreStateFingerprint != aggregate.PreStateFingerprint ||
		aggregate.Reconciliation.PreStateFingerprint != aggregate.PreStateFingerprint ||
		aggregate.Reconciliation.RecoveryObservationFingerprint != aggregate.RecoveryObservation.Fingerprint {
		return fmt.Errorf("provider-pool App Server aggregate fingerprint binding is invalid")
	}
	if !aggregate.UnresolvedClaimConsumed || !aggregate.RecoveryObservationConsumed || !aggregate.PermanentReplayForbidden {
		return fmt.Errorf("provider-pool App Server aggregate consumption and replay state is invalid")
	}
	if err := validateProviderPoolAppServerAggregateDecision(aggregate); err != nil {
		return err
	}
	return nil
}

func validateProviderPoolAppServerAggregateDecision(aggregate providerPoolAppServerAggregate) error {
	decision := aggregate.UserDecision
	switch decision.State {
	case providerPoolAppServerDecisionRequired:
		if decision.BoundRevision != 0 || decision.BoundReconciliationFingerprint != "" || decision.DecisionFingerprint != "" || decision.Consumed || decision.ConsumedRevision != 0 || decision.ConsumedTurn != 0 || aggregate.TurnHighWaterMark != aggregate.UnknownPendingTurn {
			return fmt.Errorf("provider-pool App Server aggregate required decision is invalid")
		}
	case providerPoolAppServerDecisionAccepted:
		if aggregate.Revision < 2 || decision.BoundRevision != aggregate.Revision-1 || decision.BoundReconciliationFingerprint != aggregate.Reconciliation.Fingerprint || !validProviderPoolAppServerAggregateFingerprint(decision.DecisionFingerprint) || decision.Consumed || decision.ConsumedRevision != 0 || decision.ConsumedTurn != 0 || aggregate.TurnHighWaterMark != aggregate.UnknownPendingTurn {
			return fmt.Errorf("provider-pool App Server aggregate accepted decision is invalid")
		}
	case providerPoolAppServerDecisionConsumed:
		if aggregate.Revision < 3 || decision.BoundRevision == 0 || decision.BoundRevision >= aggregate.Revision || decision.BoundReconciliationFingerprint != aggregate.Reconciliation.Fingerprint || !validProviderPoolAppServerAggregateFingerprint(decision.DecisionFingerprint) || !decision.Consumed || decision.ConsumedRevision != aggregate.Revision || decision.ConsumedTurn <= aggregate.UnknownPendingTurn || aggregate.TurnHighWaterMark != decision.ConsumedTurn {
			return fmt.Errorf("provider-pool App Server aggregate consumed decision is invalid")
		}
	default:
		return fmt.Errorf("provider-pool App Server aggregate decision state is unsupported")
	}
	return nil
}

func validProviderPoolAppServerAggregateIdentity(value string) bool {
	if value == "" || len(value) > providerPoolAppServerAggregateMaxIdentityBytes || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if !unicode.IsGraphic(character) || unicode.IsSpace(character) || unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validProviderPoolAppServerAggregateFingerprint(value string) bool {
	if len(value) != len("sha256:")+64 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	for _, character := range value[len("sha256:"):] {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func encodeProviderPoolAppServerAggregateCanonical(aggregate providerPoolAppServerAggregate, previousRevision uint64) ([]byte, error) {
	if err := validateProviderPoolAppServerAggregate(aggregate, previousRevision); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(aggregate)
	if err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}

func decodeProviderPoolAppServerAggregateCanonical(raw []byte, expectedSessionID string, previousRevision uint64) (providerPoolAppServerAggregate, error) {
	if len(raw) == 0 || len(raw) > providerPoolAppServerAggregateMaxBytes {
		return providerPoolAppServerAggregate{}, fmt.Errorf("provider-pool App Server aggregate size is invalid")
	}
	var aggregate providerPoolAppServerAggregate
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&aggregate); err != nil {
		return providerPoolAppServerAggregate{}, fmt.Errorf("provider-pool App Server aggregate is malformed: %w", err)
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return providerPoolAppServerAggregate{}, fmt.Errorf("provider-pool App Server aggregate has trailing data")
	}
	if aggregate.PipeonSessionID != expectedSessionID || !validProviderPoolAppServerAggregateIdentity(expectedSessionID) {
		return providerPoolAppServerAggregate{}, fmt.Errorf("provider-pool App Server aggregate session binding is invalid")
	}
	canonical, err := encodeProviderPoolAppServerAggregateCanonical(aggregate, previousRevision)
	if err != nil {
		return providerPoolAppServerAggregate{}, err
	}
	if !bytes.Equal(raw, canonical) {
		return providerPoolAppServerAggregate{}, fmt.Errorf("provider-pool App Server aggregate is not canonical")
	}
	return aggregate, nil
}

func loadProviderPoolAppServerAggregate(workdir, expectedSessionID string, previousRevision uint64) (providerPoolAppServerAggregate, error) {
	path, err := statepaths.ProviderPoolAppServerAggregatePath(workdir, expectedSessionID)
	if err != nil {
		return providerPoolAppServerAggregate{}, err
	}
	pathInfo, err := os.Lstat(path)
	if err != nil || !pathInfo.Mode().IsRegular() || pathInfo.Size() <= 0 || pathInfo.Size() > providerPoolAppServerAggregateMaxBytes {
		return providerPoolAppServerAggregate{}, fmt.Errorf("provider-pool App Server aggregate must be a bounded regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return providerPoolAppServerAggregate{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > providerPoolAppServerAggregateMaxBytes {
		return providerPoolAppServerAggregate{}, fmt.Errorf("provider-pool App Server aggregate must be a bounded regular file")
	}
	raw, err := io.ReadAll(io.LimitReader(file, providerPoolAppServerAggregateMaxBytes+1))
	if err != nil || len(raw) == 0 || len(raw) > providerPoolAppServerAggregateMaxBytes {
		return providerPoolAppServerAggregate{}, fmt.Errorf("provider-pool App Server aggregate read is invalid")
	}
	return decodeProviderPoolAppServerAggregateCanonical(raw, expectedSessionID, previousRevision)
}
