package orchestrationhelper

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestNodeConnectorManagedBrokerPreviewDeterministicBoundaryAndNoAuthority(t *testing.T) {
	expected, evidence := nodeConnectorManagedBrokerPreviewFixture(t)
	rawFirst, err := EncodeNodeConnectorManagedBrokerPreviewEvidence(evidence)
	if err != nil {
		t.Fatal(err)
	}
	rawSecond, err := EncodeNodeConnectorManagedBrokerPreviewEvidence(evidence)
	if err != nil {
		t.Fatal(err)
	}
	if string(rawFirst) != string(rawSecond) {
		t.Fatal("managed broker preview fixture encoding is not deterministic")
	}
	if strings.Contains(strings.ToLower(string(rawFirst)), "secret") || strings.Contains(strings.ToLower(string(rawFirst)), "token") || strings.Contains(strings.ToLower(string(rawFirst)), "credential") {
		t.Fatal("managed broker preview JSON contains forbidden material")
	}

	root := t.TempDir()
	preview, err := OpenNodeConnectorManagedBrokerPreview(root, expected)
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := preview.Accept(rawFirst)
	if err != nil {
		t.Fatal(err)
	}
	if accepted.EvidenceFingerprint == "" || preview.Generation() != 2 || preview.AcceptedCount() != 1 {
		t.Fatal("managed broker preview did not publish one accepted fixture")
	}
	if accepted.Ingress.Mode != NodeConnectorManagedBrokerIngressMode {
		t.Fatal("managed broker preview did not preserve shared tenant/node ingress")
	}
	if accepted.ExecutionAuthority.OperationID != expected.OperationID || accepted.ExecutionAuthority.RequestFingerprint != expected.RequestFingerprint || accepted.ExecutionAuthority.LeaseID != expected.LeaseID {
		t.Fatal("managed broker preview changed the sole request and lease authority binding")
	}
	if accepted.Authority != (NodeConnectorManagedBrokerPreviewAuthority{}) || accepted.Quota.Authority != (NodeConnectorManagedBrokerPreviewAuthority{}) || accepted.Audit.Authority != (NodeConnectorManagedBrokerPreviewAuthority{}) || accepted.Retention.Authority != (NodeConnectorManagedBrokerPreviewAuthority{}) || accepted.Availability.Authority != (NodeConnectorManagedBrokerPreviewAuthority{}) {
		t.Fatal("managed broker evidence gained authority")
	}
	if accepted.Audit.Authority.CompletionProven || accepted.Availability.Authority.CompletionProven {
		t.Fatal("audit or availability evidence proved completion")
	}

	validatorCalls, executorCalls := 0, 0
	connector, err := NewNodeValidationConnector(NodeExecutionWorkflowReference{Kind: "dockpipe.workflow", Package: "dorkpipe", Name: "fixture"}, strings.Repeat("a", 40), func(NodeValidationInvocation) (NodeValidationEvidence, error) {
		validatorCalls++
		return NodeValidationEvidence{}, errors.New("must not run")
	})
	if err != nil || connector == nil {
		t.Fatal("validation connector fixture could not be constructed")
	}
	executor := NodeExecutionFakeExecutor(func(NodeExecutionRequest, NodeExecutionTaskLease) { executorCalls++ })
	if executor == nil {
		t.Fatal("executor fixture could not be constructed")
	}
	if validatorCalls != 0 || executorCalls != 0 {
		t.Fatal("managed broker preview invoked an executor or validation connector")
	}

	nodeConnectorManagedBrokerAssertJSONFields(t, NodeExecutionMachineIdentity{}, []string{"schema", "machine_id", "enrolled_at"})
	nodeConnectorManagedBrokerAssertJSONFields(t, NodeExecutionCapabilitySnapshot{}, []string{"schema", "snapshot_id", "machine_id", "observed", "approved", "observed_at"})
	nodeConnectorManagedBrokerAssertJSONFields(t, NodeExecutionTaskLease{}, []string{"schema", "lease_id", "machine_id", "capability_snapshot_id", "operation_id", "attempt", "issued_at", "expires_at", "cancellation_id"})
	nodeConnectorManagedBrokerAssertJSONFields(t, NodeExecutionReceipt{}, []string{"schema", "receipt_id", "operation_id", "machine_id", "capability_snapshot_id", "lease_id", "attempt", "request_fingerprint", "local_run_id", "final_cursor", "result", "artifacts", "cancellation_id", "cancellation_acknowledged", "cleanup", "completed_at", "receipt_fingerprint"})
	if NodeExecutionMachineIdentitySchema != "dorkpipe.node-execution.machine-identity/v1" || NodeExecutionCapabilitySnapshotSchema != "dorkpipe.node-execution.capability-snapshot/v1" || NodeExecutionLeaseSchema != "dorkpipe.node-execution.task-lease/v1" || NodeExecutionReceiptSchema != "dorkpipe.node-execution.execution-receipt/v1" {
		t.Fatal("managed broker preview changed an existing machine, capability, lease, or receipt schema")
	}
}

func TestNodeConnectorManagedBrokerPreviewRejectsSubstitutionMalformedAndUnknownIdentity(t *testing.T) {
	expected, evidence := nodeConnectorManagedBrokerPreviewFixture(t)
	cases := []struct {
		name   string
		mutate func(*NodeConnectorManagedBrokerPreviewEvidence)
	}{
		{name: "cross tenant quota", mutate: func(v *NodeConnectorManagedBrokerPreviewEvidence) { v.Quota.TenantID = "tenant-other-001" }},
		{name: "cross tenant ingress", mutate: func(v *NodeConnectorManagedBrokerPreviewEvidence) { v.Ingress.TenantID = "tenant-other-001" }},
		{name: "unknown machine", mutate: func(v *NodeConnectorManagedBrokerPreviewEvidence) { v.Availability.MachineID = "machine-unknown-001" }},
		{name: "connection substitution", mutate: func(v *NodeConnectorManagedBrokerPreviewEvidence) { v.Audit.ConnectionID = "connection-unknown-001" }},
		{name: "provider substitution", mutate: func(v *NodeConnectorManagedBrokerPreviewEvidence) { v.Ingress.ProviderID = "provider-unknown-001" }},
		{name: "identity collision", mutate: func(v *NodeConnectorManagedBrokerPreviewEvidence) { v.ReplayIdentity = v.PreviewID }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			changed := evidence
			tc.mutate(&changed)
			raw, err := EncodeNodeConnectorManagedBrokerPreviewEvidence(changed)
			if err != nil {
				return
			}
			preview, err := OpenNodeConnectorManagedBrokerPreview(t.TempDir(), expected)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := preview.Accept(raw); err == nil {
				t.Fatal("substituted managed broker evidence was accepted")
			}
			if preview.AcceptedCount() != 0 || preview.Generation() != 1 {
				t.Fatal("rejection published partial managed broker state")
			}
		})
	}

	preview, err := OpenNodeConnectorManagedBrokerPreview(t.TempDir(), expected)
	if err != nil {
		t.Fatal(err)
	}
	valid, err := EncodeNodeConnectorManagedBrokerPreviewEvidence(evidence)
	if err != nil {
		t.Fatal(err)
	}
	malformed := append(append([]byte{}, valid[:len(valid)-1]...), []byte(`,"unknown_identity":"node-unknown-001"}`)...)
	if _, err := preview.Accept(malformed); err == nil {
		t.Fatal("unknown managed broker field was accepted")
	}
	if _, err := preview.Accept([]byte("{not-json")); err == nil {
		t.Fatal("malformed managed broker JSON was accepted")
	}
	secretRaw := []byte(strings.Replace(string(valid), evidence.PreviewID, "preview-token-material-001", 1))
	if _, err := preview.Accept(secretRaw); err == nil || strings.Contains(err.Error(), "material-001") {
		t.Fatal("secret-like material was accepted or leaked in an error")
	}
	if preview.AcceptedCount() != 0 {
		t.Fatal("malformed evidence published managed broker state")
	}
}

func TestNodeConnectorManagedBrokerPreviewReplayRestartAndConflict(t *testing.T) {
	expected, evidence := nodeConnectorManagedBrokerPreviewFixture(t)
	raw, err := EncodeNodeConnectorManagedBrokerPreviewEvidence(evidence)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	preview, err := OpenNodeConnectorManagedBrokerPreview(root, expected)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := preview.Accept(raw); err != nil {
		t.Fatal(err)
	}
	generation := preview.Generation()
	if _, err := preview.Accept(raw); err != nil {
		t.Fatal(err)
	}
	if preview.Generation() != generation || preview.AcceptedCount() != 1 {
		t.Fatal("identical replay created new authority or state")
	}

	reopened, err := OpenNodeConnectorManagedBrokerPreview(root, expected)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.Accept(raw); err != nil {
		t.Fatal(err)
	}
	if reopened.Generation() != generation || reopened.AcceptedCount() != 1 {
		t.Fatal("restart replay was not deterministic and idempotent")
	}

	conflict := evidence
	conflict.Availability.Status = "degraded"
	conflictRaw, err := EncodeNodeConnectorManagedBrokerPreviewEvidence(conflict)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.Accept(conflictRaw); err == nil {
		t.Fatal("conflicting managed broker replay was accepted")
	}
	if reopened.Generation() != generation || reopened.AcceptedCount() != 1 {
		t.Fatal("conflicting replay changed durable state")
	}
}

func TestNodeConnectorManagedBrokerPreviewRejectsUnboundedQuotaAndRetention(t *testing.T) {
	expected, evidence := nodeConnectorManagedBrokerPreviewFixture(t)
	cases := []struct {
		name   string
		mutate func(*NodeConnectorManagedBrokerPreviewEvidence)
	}{
		{name: "zero nodes", mutate: func(v *NodeConnectorManagedBrokerPreviewEvidence) { v.Quota.MaxNodes = 0 }},
		{name: "unbounded nodes", mutate: func(v *NodeConnectorManagedBrokerPreviewEvidence) {
			v.Quota.MaxNodes = nodeConnectorManagedBrokerMaxNodes + 1
		}},
		{name: "unbounded concurrency", mutate: func(v *NodeConnectorManagedBrokerPreviewEvidence) {
			v.Quota.MaxConcurrentWorkItems = nodeConnectorManagedBrokerMaxConcurrentWork + 1
		}},
		{name: "unbounded bytes", mutate: func(v *NodeConnectorManagedBrokerPreviewEvidence) {
			v.Quota.MaxArtifactBytes = nodeConnectorManagedBrokerMaxArtifactBytes + 1
		}},
		{name: "zero audit retention", mutate: func(v *NodeConnectorManagedBrokerPreviewEvidence) { v.Retention.AuditDays = 0 }},
		{name: "unbounded artifact retention", mutate: func(v *NodeConnectorManagedBrokerPreviewEvidence) {
			v.Retention.ArtifactDays = nodeConnectorManagedBrokerMaxRetentionDays + 1
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			changed := evidence
			tc.mutate(&changed)
			if _, err := EncodeNodeConnectorManagedBrokerPreviewEvidence(changed); err == nil {
				t.Fatal("invalid or unbounded declaration was finalized")
			}
			preview, err := OpenNodeConnectorManagedBrokerPreview(t.TempDir(), expected)
			if err != nil {
				t.Fatal(err)
			}
			if preview.AcceptedCount() != 0 {
				t.Fatal("invalid declaration published state")
			}
		})
	}
}

func TestNodeConnectorManagedBrokerPreviewAtomicFailurePublishesNoPartialRestartState(t *testing.T) {
	expected, evidence := nodeConnectorManagedBrokerPreviewFixture(t)
	raw, err := EncodeNodeConnectorManagedBrokerPreviewEvidence(evidence)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	preview, err := OpenNodeConnectorManagedBrokerPreview(root, expected)
	if err != nil {
		t.Fatal(err)
	}
	original := nodeConnectorManagedBrokerWriteAtomic
	nodeConnectorManagedBrokerWriteAtomic = func(string, any) error { return errors.New("injected write failure") }
	t.Cleanup(func() { nodeConnectorManagedBrokerWriteAtomic = original })
	if _, err := preview.Accept(raw); err == nil {
		t.Fatal("atomic write failure was accepted")
	}
	if preview.Generation() != 1 || preview.AcceptedCount() != 0 {
		t.Fatal("atomic failure advanced in-memory state")
	}
	nodeConnectorManagedBrokerWriteAtomic = original
	reopened, err := OpenNodeConnectorManagedBrokerPreview(root, expected)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.Generation() != 1 || reopened.AcceptedCount() != 0 {
		t.Fatal("restart observed partial managed broker state")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != nodeConnectorManagedBrokerStateFileName(1) {
		t.Fatalf("atomic failure left partial artifacts: %#v", entries)
	}
}

func nodeConnectorManagedBrokerPreviewFixture(t *testing.T) (NodeConnectorManagedBrokerPreviewExpectedBinding, NodeConnectorManagedBrokerPreviewEvidence) {
	t.Helper()
	expected := NodeConnectorManagedBrokerPreviewExpectedBinding{
		TenantID: "tenant-managed-preview-001", NodeID: "node-managed-preview-001", MachineID: "machine-managed-preview-001",
		CapabilitySnapshotID: "sha256:" + strings.Repeat("1", 64), ConnectionID: "connection-managed-preview-001", ProviderID: "provider-managed-preview-001",
		OperationID: "operation-managed-preview-001", RequestFingerprint: "sha256:" + strings.Repeat("2", 64), LeaseID: "lease-managed-preview-001", ReceiptID: "receipt-managed-preview-001",
	}
	evidence := NodeConnectorManagedBrokerPreviewEvidence{
		PreviewID: "preview-managed-broker-001", ReplayIdentity: "replay-managed-broker-001",
		Tenant:             NodeConnectorManagedBrokerTenantIdentity{TenantID: expected.TenantID, CreatedAt: "2026-07-26T00:00:00Z"},
		Quota:              NodeConnectorManagedBrokerQuotaSnapshot{SnapshotID: "quota-managed-preview-001", TenantID: expected.TenantID, MaxNodes: 25, MaxConcurrentWorkItems: 100, MaxArtifactBytes: 1 << 30, ObservedAt: "2026-07-26T00:01:00Z"},
		Audit:              NodeConnectorManagedBrokerAuditEvidence{EvidenceID: "audit-managed-preview-001", TenantID: expected.TenantID, NodeID: expected.NodeID, MachineID: expected.MachineID, CapabilitySnapshotID: expected.CapabilitySnapshotID, ConnectionID: expected.ConnectionID, ProviderID: expected.ProviderID, OperationID: expected.OperationID, LeaseID: expected.LeaseID, ReceiptID: expected.ReceiptID, ObservedAt: "2026-07-26T00:02:00Z"},
		Retention:          NodeConnectorManagedBrokerRetentionPolicy{PolicyID: "retention-managed-preview-001", TenantID: expected.TenantID, AuditDays: 365, ArtifactDays: 30, EffectiveAt: "2026-07-26T00:03:00Z"},
		Availability:       NodeConnectorManagedBrokerAvailabilityEvidence{EvidenceID: "availability-managed-preview-001", TenantID: expected.TenantID, NodeID: expected.NodeID, MachineID: expected.MachineID, ConnectionID: expected.ConnectionID, ProviderID: expected.ProviderID, Status: "available", ObservedAt: "2026-07-26T00:04:00Z"},
		Ingress:            NodeConnectorManagedBrokerSharedIngressBinding{BindingID: "ingress-managed-preview-001", TenantID: expected.TenantID, NodeID: expected.NodeID, MachineID: expected.MachineID, ConnectionID: expected.ConnectionID, ProviderID: expected.ProviderID},
		ExecutionAuthority: NodeConnectorManagedBrokerExecutionAuthority{OperationID: expected.OperationID, RequestFingerprint: expected.RequestFingerprint, LeaseID: expected.LeaseID},
	}
	return expected, evidence
}

func nodeConnectorManagedBrokerAssertJSONFields(t *testing.T, value any, expected []string) {
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

func TestNodeConnectorManagedBrokerPreviewStateNamesRemainContained(t *testing.T) {
	root := t.TempDir()
	expected, _ := nodeConnectorManagedBrokerPreviewFixture(t)
	if _, err := OpenNodeConnectorManagedBrokerPreview(root, expected); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, nodeConnectorManagedBrokerStateFileName(1))); err != nil {
		t.Fatal(err)
	}
}
