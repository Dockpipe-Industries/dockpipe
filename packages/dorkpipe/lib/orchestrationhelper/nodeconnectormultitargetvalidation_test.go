package orchestrationhelper

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestNodeConnectorMultiTargetValidationDeterministicCanonicalAggregate(t *testing.T) {
	expected, input := nodeConnectorMultiTargetValidationFixture(t)
	firstRoot := t.TempDir()
	first := mustOpenNodeConnectorMultiTargetValidation(t, firstRoot, expected)
	firstAggregate := mustAcceptNodeConnectorMultiTargetValidation(t, first, input)
	if firstAggregate.Status != "passed" || len(firstAggregate.FailedTargetIDs) != 0 || firstAggregate.RepairDispatched || firstAggregate.Authority != (NodeConnectorMultiTargetValidationAuthority{}) {
		t.Fatal("three passing receipts did not produce a passed, authority-free aggregate")
	}
	if firstAggregate.Provenance != nodeConnectorMultiTargetValidationProvenance || len(firstAggregate.Targets) != 3 || len(firstAggregate.ReceiptBindings) != 3 {
		t.Fatal("aggregate omitted its bounded declaration, receipt bindings, or provenance")
	}
	for index := 1; index < len(firstAggregate.Targets); index++ {
		if firstAggregate.Targets[index-1].TargetID >= firstAggregate.Targets[index].TargetID || firstAggregate.ReceiptBindings[index-1].TargetID >= firstAggregate.ReceiptBindings[index].TargetID {
			t.Fatal("aggregate output is not sorted by target ID")
		}
	}
	if nodeConnectorMultiTargetValidationContainsForbiddenSurface(firstAggregate) {
		t.Fatal("aggregate serialized provider, availability, managed-service, or connection evidence")
	}
	rawAggregate, err := os.ReadFile(filepath.Join(firstRoot, nodeConnectorMultiTargetValidationArtifactName))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"hostname", "endpoint", "command", "stdout", "stderr", "environment", "filesystem", "credential", "token", "secret"} {
		if strings.Contains(strings.ToLower(string(rawAggregate)), forbidden) {
			t.Fatalf("aggregate contains forbidden serialized surface %q", forbidden)
		}
	}

	reversed := input
	reversed.Targets = append([]NodeConnectorMultiTargetValidationTargetEvidence{}, input.Targets...)
	for left, right := 0, len(reversed.Targets)-1; left < right; left, right = left+1, right-1 {
		reversed.Targets[left], reversed.Targets[right] = reversed.Targets[right], reversed.Targets[left]
	}
	secondRoot := t.TempDir()
	second := mustOpenNodeConnectorMultiTargetValidation(t, secondRoot, expected)
	secondAggregate := mustAcceptNodeConnectorMultiTargetValidation(t, second, reversed)
	secondRaw, err := os.ReadFile(filepath.Join(secondRoot, nodeConnectorMultiTargetValidationArtifactName))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(firstAggregate, secondAggregate) || !bytes.Equal(rawAggregate, secondRaw) || firstAggregate.AggregateFingerprint != secondAggregate.AggregateFingerprint {
		t.Fatal("permuted target input changed aggregate bytes or fingerprint")
	}

	nodeConnectorMultiTargetValidationAssertJSONFields(t, NodeExecutionMachineIdentity{}, []string{"schema", "machine_id", "enrolled_at"})
	nodeConnectorMultiTargetValidationAssertJSONFields(t, NodeExecutionCapabilitySnapshot{}, []string{"schema", "snapshot_id", "machine_id", "observed", "approved", "observed_at"})
	nodeConnectorMultiTargetValidationAssertJSONFields(t, NodeExecutionTaskLease{}, []string{"schema", "lease_id", "machine_id", "capability_snapshot_id", "operation_id", "attempt", "issued_at", "expires_at", "cancellation_id"})
	nodeConnectorMultiTargetValidationAssertJSONFields(t, NodeExecutionReceipt{}, []string{"schema", "receipt_id", "operation_id", "machine_id", "capability_snapshot_id", "lease_id", "attempt", "request_fingerprint", "local_run_id", "final_cursor", "result", "artifacts", "cancellation_id", "cancellation_acknowledged", "cleanup", "completed_at", "receipt_fingerprint"})
}

func TestNodeConnectorMultiTargetValidationRequiresAllPassingReceiptsAndDispatchesNoRepair(t *testing.T) {
	expected, input := nodeConnectorMultiTargetValidationFixture(t)
	input.Targets[1].Receipt.Result = "degraded"
	input.Targets[1].Receipt = mustFinalizeNodeConnectorMultiTargetValidationReceipt(t, input.Targets[1].Receipt)
	expected.Targets[1].ReceiptFingerprint = input.Targets[1].Receipt.ReceiptFingerprint
	aggregate := mustAcceptNodeConnectorMultiTargetValidation(t, mustOpenNodeConnectorMultiTargetValidation(t, t.TempDir(), expected), input)
	if aggregate.Status != "failed" || !reflect.DeepEqual(aggregate.FailedTargetIDs, []string{input.Targets[1].TargetID}) {
		t.Fatal("degraded receipt did not derive one exact failed target")
	}
	if aggregate.ReceiptBindings[1].TerminalResult != "degraded" || aggregate.ReceiptBindings[1].Outcome != "failed" || aggregate.RepairDispatched || aggregate.Authority != (NodeConnectorMultiTargetValidationAuthority{}) {
		t.Fatal("failed target changed outcome derivation or gained repair/lifecycle authority")
	}

	validatorCalls, executorCalls, schedulerCalls, transportCalls, repairCalls := 0, 0, 0, 0, 0
	_ = []func(){func() { validatorCalls++ }, func() { executorCalls++ }, func() { schedulerCalls++ }, func() { transportCalls++ }, func() { repairCalls++ }}
	if validatorCalls+executorCalls+schedulerCalls+transportCalls+repairCalls != 0 {
		t.Fatal("aggregation invoked an executor, validator, scheduler, transport, or repair surface")
	}
}

func TestNodeConnectorMultiTargetValidationRejectsMissingDuplicateExtraUnknownAndWrongProfiles(t *testing.T) {
	expected, input := nodeConnectorMultiTargetValidationFixture(t)
	cases := []struct {
		name   string
		mutate func(*NodeConnectorMultiTargetValidationInput)
	}{
		{name: "missing", mutate: func(value *NodeConnectorMultiTargetValidationInput) { value.Targets = value.Targets[:2] }},
		{name: "duplicate", mutate: func(value *NodeConnectorMultiTargetValidationInput) { value.Targets[2] = value.Targets[1] }},
		{name: "extra", mutate: func(value *NodeConnectorMultiTargetValidationInput) {
			value.Targets = append(value.Targets, value.Targets[0])
		}},
		{name: "unknown", mutate: func(value *NodeConnectorMultiTargetValidationInput) { value.Targets[1].TargetID = "target-unknown-001" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			changed := cloneNodeConnectorMultiTargetValidationInput(input)
			tc.mutate(&changed)
			validation := mustOpenNodeConnectorMultiTargetValidation(t, t.TempDir(), expected)
			if _, err := validation.Accept(mustMarshalNodeConnectorMultiTargetValidation(t, changed)); err == nil {
				t.Fatal("invalid target set was accepted")
			}
			if _, ok := validation.Aggregate(); ok {
				t.Fatal("invalid target set published aggregate state")
			}
		})
	}

	wrongExpected := expected
	wrongExpected.Targets = append([]NodeConnectorMultiTargetValidationExpectedTarget{}, expected.Targets...)
	wrongExpected.Targets[0].Profile = NodeConnectorMultiTargetValidationProfile{HostOS: "linux", Runtime: "host", GuestOS: "windows"}
	if _, err := OpenNodeConnectorMultiTargetValidation(t.TempDir(), wrongExpected); err == nil {
		t.Fatal("wrong or ambiguous expected target profile was accepted")
	}
	wrongEvidence := cloneNodeConnectorMultiTargetValidationInput(input)
	wrongEvidence.Targets[0].Capability.Observed.HostOS = "windows"
	wrongEvidence.Targets[0].Capability, _ = NewNodeExecutionCapabilitySnapshot(
		wrongEvidence.Targets[0].Machine.MachineID, wrongEvidence.Targets[0].Capability.Observed,
		wrongEvidence.Targets[0].Capability.Approved, mustParseNodeConnectorMultiTargetValidationTime(t, wrongEvidence.Targets[0].Capability.ObservedAt),
	)
	if _, err := mustOpenNodeConnectorMultiTargetValidation(t, t.TempDir(), expected).Accept(mustMarshalNodeConnectorMultiTargetValidation(t, wrongEvidence)); err == nil {
		t.Fatal("wrong host/runtime/guest profile was accepted")
	}
}

func TestNodeConnectorMultiTargetValidationRejectsCrossTargetIdentitySubstitution(t *testing.T) {
	expected, input := nodeConnectorMultiTargetValidationFixture(t)
	cases := []struct {
		name   string
		mutate func(*NodeConnectorMultiTargetValidationInput)
	}{
		{name: "machine", mutate: func(value *NodeConnectorMultiTargetValidationInput) {
			value.Targets[0].Machine = value.Targets[1].Machine
		}},
		{name: "capability", mutate: func(value *NodeConnectorMultiTargetValidationInput) {
			value.Targets[0].Capability = value.Targets[1].Capability
		}},
		{name: "operation request", mutate: func(value *NodeConnectorMultiTargetValidationInput) {
			value.Targets[0].Request = value.Targets[1].Request
		}},
		{name: "lease", mutate: func(value *NodeConnectorMultiTargetValidationInput) { value.Targets[0].Lease = value.Targets[1].Lease }},
		{name: "attempt", mutate: func(value *NodeConnectorMultiTargetValidationInput) { value.Targets[0].Lease.Attempt++ }},
		{name: "receipt", mutate: func(value *NodeConnectorMultiTargetValidationInput) {
			value.Targets[0].Receipt = value.Targets[1].Receipt
		}},
		{name: "local run", mutate: func(value *NodeConnectorMultiTargetValidationInput) {
			value.Targets[0].Receipt.LocalRunID = value.Targets[1].Receipt.LocalRunID
			value.Targets[0].Receipt = mustFinalizeNodeConnectorMultiTargetValidationReceipt(t, value.Targets[0].Receipt)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			changed := cloneNodeConnectorMultiTargetValidationInput(input)
			tc.mutate(&changed)
			validation := mustOpenNodeConnectorMultiTargetValidation(t, t.TempDir(), expected)
			if _, err := validation.Accept(mustMarshalNodeConnectorMultiTargetValidation(t, changed)); err == nil {
				t.Fatal("cross-target identity substitution was accepted")
			}
			if _, ok := validation.Aggregate(); ok {
				t.Fatal("cross-target substitution published aggregate state")
			}
		})
	}
}

func TestNodeConnectorMultiTargetValidationRejectsReceiptEventArtifactCleanupAndResultTampering(t *testing.T) {
	expected, input := nodeConnectorMultiTargetValidationFixture(t)
	cases := []struct {
		name   string
		mutate func(*NodeConnectorMultiTargetValidationInput)
	}{
		{name: "receipt fingerprint", mutate: func(value *NodeConnectorMultiTargetValidationInput) {
			value.Targets[0].Receipt.ReceiptFingerprint = "sha256:" + strings.Repeat("f", 64)
		}},
		{name: "event fingerprint", mutate: func(value *NodeConnectorMultiTargetValidationInput) {
			value.Targets[0].Events[0].EnvelopeFingerprint = "sha256:" + strings.Repeat("f", 64)
		}},
		{name: "refinalized event bytes", mutate: func(value *NodeConnectorMultiTargetValidationInput) {
			var event map[string]any
			if err := json.Unmarshal(value.Targets[0].Events[0].Event, &event); err != nil {
				t.Fatal(err)
			}
			event["status"] = "changed"
			value.Targets[0].Events[0].Event = mustMarshalNodeConnectorMultiTargetValidation(t, event)
			finalized, err := FinalizeNodeExecutionEvent(value.Targets[0].Events[0])
			if err != nil {
				t.Fatal(err)
			}
			value.Targets[0].Events[0] = finalized
		}},
		{name: "artifact binding", mutate: func(value *NodeConnectorMultiTargetValidationInput) {
			value.Targets[0].Receipt.Artifacts.ManifestFingerprint = "sha256:" + strings.Repeat("f", 64)
		}},
		{name: "cleanup binding", mutate: func(value *NodeConnectorMultiTargetValidationInput) {
			value.Targets[0].Receipt.Cleanup.Status = "failed"
		}},
		{name: "terminal result", mutate: func(value *NodeConnectorMultiTargetValidationInput) { value.Targets[0].Receipt.Result = "failed" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			changed := cloneNodeConnectorMultiTargetValidationInput(input)
			tc.mutate(&changed)
			validation := mustOpenNodeConnectorMultiTargetValidation(t, t.TempDir(), expected)
			if _, err := validation.Accept(mustMarshalNodeConnectorMultiTargetValidation(t, changed)); err == nil {
				t.Fatal("tampered immutable receipt chain was accepted")
			}
			if _, ok := validation.Aggregate(); ok {
				t.Fatal("tampered immutable receipt chain published state")
			}
		})
	}
}

func TestNodeConnectorMultiTargetValidationExcludesManagedAndPresenceClaimsAndRejectsMalformedInput(t *testing.T) {
	expected, input := nodeConnectorMultiTargetValidationFixture(t)
	valid := mustMarshalNodeConnectorMultiTargetValidation(t, input)
	root := t.TempDir()
	validation := mustOpenNodeConnectorMultiTargetValidation(t, root, expected)

	claims := []string{"availability", "connection", "provider", "ingress", "quota", "audit", "retention", "managed_service"}
	for _, claim := range claims {
		withClaim := append(append([]byte{}, valid[:len(valid)-1]...), []byte(`,"`+claim+`":{"status":"available","authoritative":true}}`)...)
		if _, err := validation.Accept(withClaim); err == nil {
			t.Fatalf("non-authoritative %s claim entered the aggregation input", claim)
		}
	}
	if _, err := validation.Accept([]byte("{not-json")); err == nil {
		t.Fatal("malformed input was accepted")
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, valid, "", "  "); err != nil {
		t.Fatal(err)
	}
	if _, err := validation.Accept(pretty.Bytes()); err == nil {
		t.Fatal("noncanonical input JSON was accepted")
	}
	oversized := make([]byte, nodeConnectorMultiTargetValidationMaxInputBytes+1)
	if _, err := validation.Accept(oversized); err == nil {
		t.Fatal("oversized input was accepted")
	}
	if _, ok := validation.Aggregate(); ok {
		t.Fatal("rejected contextual or malformed claims published state")
	}
}

func TestNodeConnectorMultiTargetValidationReplayRestartConflictAndTamperFailClosed(t *testing.T) {
	expected, input := nodeConnectorMultiTargetValidationFixture(t)
	raw := mustMarshalNodeConnectorMultiTargetValidation(t, input)
	root := t.TempDir()
	validation := mustOpenNodeConnectorMultiTargetValidation(t, root, expected)
	accepted, err := validation.Accept(raw)
	if err != nil {
		t.Fatal(err)
	}

	originalWriter := nodeConnectorMultiTargetValidationWriteAtomic
	writes := 0
	nodeConnectorMultiTargetValidationWriteAtomic = func(path string, value any) error {
		writes++
		return originalWriter(path, value)
	}
	t.Cleanup(func() { nodeConnectorMultiTargetValidationWriteAtomic = originalWriter })
	if replay, err := validation.Accept(raw); err != nil || !reflect.DeepEqual(replay, accepted) || writes != 0 {
		t.Fatal("exact replay rewrote or changed the aggregate")
	}
	restarted := mustOpenNodeConnectorMultiTargetValidation(t, root, expected)
	if replay, err := restarted.Accept(raw); err != nil || !reflect.DeepEqual(replay, accepted) || writes != 0 {
		t.Fatal("restart replay rewrote or changed the aggregate")
	}

	conflict := cloneNodeConnectorMultiTargetValidationInput(input)
	conflict.Targets[0].Receipt.Result = "failed"
	conflict.Targets[0].Receipt = mustFinalizeNodeConnectorMultiTargetValidationReceipt(t, conflict.Targets[0].Receipt)
	if _, err := restarted.Accept(mustMarshalNodeConnectorMultiTargetValidation(t, conflict)); err == nil || writes != 0 {
		t.Fatal("changed receipt bytes did not fail closed as a conflicting replay")
	}
	changedExpected := expected
	changedExpected.Targets = append([]NodeConnectorMultiTargetValidationExpectedTarget{}, expected.Targets...)
	changedExpected.Targets[0].LocalRunID = "local-run-changed-001"
	if _, err := OpenNodeConnectorMultiTargetValidation(root, changedExpected); err == nil {
		t.Fatal("changed expected target binding was accepted after restart")
	}

	artifactPath := filepath.Join(root, nodeConnectorMultiTargetValidationArtifactName)
	artifactRaw, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatal(err)
	}
	tampered := bytes.Replace(artifactRaw, []byte(`"status": "passed"`), []byte(`"status": "failed"`), 1)
	if err := os.WriteFile(artifactPath, tampered, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenNodeConnectorMultiTargetValidation(root, expected); err == nil {
		t.Fatal("tampered existing aggregate was accepted")
	}
}

func TestNodeConnectorMultiTargetValidationAtomicFailurePublishesNothing(t *testing.T) {
	expected, input := nodeConnectorMultiTargetValidationFixture(t)
	root := t.TempDir()
	validation := mustOpenNodeConnectorMultiTargetValidation(t, root, expected)
	originalWriter := nodeConnectorMultiTargetValidationWriteAtomic
	nodeConnectorMultiTargetValidationWriteAtomic = func(string, any) error { return errors.New("injected atomic write failure") }
	t.Cleanup(func() { nodeConnectorMultiTargetValidationWriteAtomic = originalWriter })
	if _, err := validation.Accept(mustMarshalNodeConnectorMultiTargetValidation(t, input)); err == nil {
		t.Fatal("injected atomic write failure was accepted")
	}
	if _, ok := validation.Aggregate(); ok {
		t.Fatal("atomic write failure published in-memory aggregate state")
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
		t.Fatalf("atomic write failure left a partial artifact: %#v, %v", entries, err)
	}
	nodeConnectorMultiTargetValidationWriteAtomic = originalWriter
	restarted := mustOpenNodeConnectorMultiTargetValidation(t, root, expected)
	if _, ok := restarted.Aggregate(); ok {
		t.Fatal("restart observed partial aggregate state")
	}
}

func nodeConnectorMultiTargetValidationFixture(t *testing.T) (NodeConnectorMultiTargetValidationExpected, NodeConnectorMultiTargetValidationInput) {
	t.Helper()
	profiles := []struct {
		targetID string
		profile  NodeConnectorMultiTargetValidationProfile
		guestOS  string
		guestID  string
		digest   string
		revision string
	}{
		{targetID: "target-linux-host-001", profile: NodeConnectorMultiTargetValidationProfile{HostOS: "linux", Runtime: "host", GuestOS: "none"}, digest: "1", revision: "a"},
		{targetID: "target-linux-qemu-windows-001", profile: NodeConnectorMultiTargetValidationProfile{HostOS: "linux", Runtime: "qemu", GuestOS: "windows"}, guestOS: "windows", guestID: "image-windows-ci-001", digest: "2", revision: "b"},
		{targetID: "target-windows-host-001", profile: NodeConnectorMultiTargetValidationProfile{HostOS: "windows", Runtime: "host", GuestOS: "none"}, digest: "3", revision: "c"},
	}
	now := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	expected := NodeConnectorMultiTargetValidationExpected{AggregateID: "aggregate-multi-target-validation-001", Targets: []NodeConnectorMultiTargetValidationExpectedTarget{}}
	input := NodeConnectorMultiTargetValidationInput{Schema: NodeConnectorMultiTargetValidationInputSchema, AggregateID: expected.AggregateID, Targets: []NodeConnectorMultiTargetValidationTargetEvidence{}}
	for index, fixture := range profiles {
		number := index + 1
		machine := NodeExecutionMachineIdentity{Schema: NodeExecutionMachineIdentitySchema, MachineID: "machine-validation-00" + string(rune('1'+index)), EnrolledAt: nodeExecutionTime(now.Add(-4 * time.Hour))}
		capability, err := NewNodeExecutionCapabilitySnapshot(machine.MachineID, NodeExecutionObservedCapabilities{
			HostOS: fixture.profile.HostOS, Runtime: fixture.profile.Runtime, GuestOS: fixture.guestOS, GuestImageID: fixture.guestID, Toolchains: []string{"go1.25"},
		}, NodeExecutionApprovedCapabilities{PolicyClass: "validation-only", AllowedWorkflowKinds: []string{"dockpipe.workflow"}}, now.Add(-3*time.Hour+time.Duration(index)*time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		request, err := FinalizeNodeExecutionRequest(NodeExecutionRequest{
			Schema: NodeExecutionRequestSchema, OperationID: "operation-validation-00" + string(rune('1'+index)), GraphRunID: "graph-multi-target-001",
			RunID: "run-validation-00" + string(rune('1'+index)), TaskID: "task-validation-00" + string(rune('1'+index)), SourceRevision: strings.Repeat(fixture.revision, 40),
			Workflow:             NodeExecutionWorkflowReference{Kind: "dockpipe.workflow", Package: "dorkpipe", Name: "validate.readonly"},
			CapabilitySnapshotID: capability.SnapshotID, Inputs: []NodeExecutionInput{}, Artifacts: []NodeExecutionArtifactReference{}, RequestedAt: nodeExecutionTime(now.Add(-time.Hour)),
		})
		if err != nil {
			t.Fatal(err)
		}
		issuedAt := now.Add(time.Duration(index) * time.Minute)
		lease := newNodeExecutionLease(request, machine.MachineID, issuedAt, issuedAt.Add(30*time.Minute))
		artifact := NodeExecutionArtifactReference{Name: "validation-result.json", MediaType: "application/json", Digest: "sha256:" + strings.Repeat(fixture.digest, 64), Bytes: int64(100 + number)}
		event, err := FinalizeNodeExecutionEvent(NodeExecutionEventEnvelope{
			OperationID: request.OperationID, GraphRunID: request.GraphRunID, RunID: request.RunID, TaskID: request.TaskID,
			MachineID: machine.MachineID, CapabilitySnapshotID: capability.SnapshotID, LeaseID: lease.LeaseID, Attempt: lease.Attempt,
			Sequence: 1, RecordedAt: nodeExecutionTime(issuedAt.Add(time.Minute)), OutputReferences: []NodeExecutionArtifactReference{artifact},
			Event: mustMarshalNodeConnectorMultiTargetValidation(t, map[string]any{"schema": "dockpipe.operation_event.v1", "type": "operation_result", "ts": nodeExecutionTime(issuedAt.Add(time.Minute)), "unit": "validation.readonly", "status": "done"}),
		})
		if err != nil {
			t.Fatal(err)
		}
		manifest, err := NewNodeExecutionArtifactManifest([]NodeExecutionArtifactReference{artifact})
		if err != nil {
			t.Fatal(err)
		}
		receipt := mustFinalizeNodeConnectorMultiTargetValidationReceipt(t, NodeExecutionReceipt{
			ReceiptID: nodeExecutionReceiptID(request.OperationID), OperationID: request.OperationID, MachineID: machine.MachineID,
			CapabilitySnapshotID: capability.SnapshotID, LeaseID: lease.LeaseID, Attempt: lease.Attempt, RequestFingerprint: request.RequestFingerprint,
			LocalRunID: "local-run-validation-00" + string(rune('1'+index)), FinalCursor: nodeExecutionCursor(1), Result: "succeeded",
			Artifacts: manifest, Cleanup: NodeExecutionCleanupOutcome{Status: "not_required"}, CompletedAt: nodeExecutionTime(issuedAt.Add(2 * time.Minute)),
		})
		machineFingerprint, err := nodeExecutionFingerprintValue(machine)
		if err != nil {
			t.Fatal(err)
		}
		leaseFingerprint, err := nodeExecutionFingerprintValue(lease)
		if err != nil {
			t.Fatal(err)
		}
		eventsFingerprint, err := nodeExecutionFingerprintValue([]NodeExecutionEventEnvelope{event})
		if err != nil {
			t.Fatal(err)
		}
		expected.Targets = append(expected.Targets, NodeConnectorMultiTargetValidationExpectedTarget{
			TargetID: fixture.targetID, Profile: fixture.profile, MachineID: machine.MachineID, MachineFingerprint: machineFingerprint, CapabilitySnapshotID: capability.SnapshotID,
			OperationID: request.OperationID, RequestFingerprint: request.RequestFingerprint, LeaseID: lease.LeaseID, LeaseFingerprint: leaseFingerprint, Attempt: lease.Attempt,
			EventsFingerprint: eventsFingerprint, ReceiptID: receipt.ReceiptID, ReceiptFingerprint: receipt.ReceiptFingerprint, LocalRunID: receipt.LocalRunID,
		})
		input.Targets = append(input.Targets, NodeConnectorMultiTargetValidationTargetEvidence{
			TargetID: fixture.targetID, Machine: machine, Capability: capability, Request: request, Lease: lease,
			Events: []NodeExecutionEventEnvelope{event}, Receipt: receipt,
		})
	}
	return expected, input
}

func mustOpenNodeConnectorMultiTargetValidation(t *testing.T, root string, expected NodeConnectorMultiTargetValidationExpected) *NodeConnectorMultiTargetValidation {
	t.Helper()
	value, err := OpenNodeConnectorMultiTargetValidation(root, expected)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func mustAcceptNodeConnectorMultiTargetValidation(t *testing.T, validation *NodeConnectorMultiTargetValidation, input NodeConnectorMultiTargetValidationInput) NodeConnectorMultiTargetValidationAggregate {
	t.Helper()
	value, err := validation.Accept(mustMarshalNodeConnectorMultiTargetValidation(t, input))
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func mustFinalizeNodeConnectorMultiTargetValidationReceipt(t *testing.T, value NodeExecutionReceipt) NodeExecutionReceipt {
	t.Helper()
	finalized, err := FinalizeNodeExecutionReceipt(value)
	if err != nil {
		t.Fatal(err)
	}
	return finalized
}

func mustMarshalNodeConnectorMultiTargetValidation(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func cloneNodeConnectorMultiTargetValidationInput(value NodeConnectorMultiTargetValidationInput) NodeConnectorMultiTargetValidationInput {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorMultiTargetValidationInput
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func mustParseNodeConnectorMultiTargetValidationTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := parseNodeExecutionTime(value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func nodeConnectorMultiTargetValidationAssertJSONFields(t *testing.T, value any, expected []string) {
	t.Helper()
	typeOf := reflect.TypeOf(value)
	actual := make([]string, typeOf.NumField())
	for index := 0; index < typeOf.NumField(); index++ {
		actual[index] = strings.Split(typeOf.Field(index).Tag.Get("json"), ",")[0]
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("existing node-execution contract changed: got %#v want %#v", actual, expected)
	}
}
