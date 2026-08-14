package orchestrationhelper

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

const (
	NodeConnectorManagedBrokerTenantSchema       = "dorkpipe.managed-broker-preview.tenant-identity/v1"
	NodeConnectorManagedBrokerQuotaSchema        = "dorkpipe.managed-broker-preview.quota-snapshot/v1"
	NodeConnectorManagedBrokerAuditSchema        = "dorkpipe.managed-broker-preview.audit-evidence/v1"
	NodeConnectorManagedBrokerRetentionSchema    = "dorkpipe.managed-broker-preview.retention-policy/v1"
	NodeConnectorManagedBrokerAvailabilitySchema = "dorkpipe.managed-broker-preview.availability-evidence/v1"
	NodeConnectorManagedBrokerIngressSchema      = "dorkpipe.managed-broker-preview.shared-ingress-binding/v1"
	NodeConnectorManagedBrokerEvidenceSchema     = "dorkpipe.managed-broker-preview.evidence/v1"

	NodeConnectorManagedBrokerIngressMode = "shared_broker_ingress_tenant_node_multiplexing"

	nodeConnectorManagedBrokerStateSchema       = "dorkpipe.managed-broker-preview.state/v1"
	nodeConnectorManagedBrokerProvenance        = "fixture_only_untrusted_preview"
	nodeConnectorManagedBrokerAuthoritySource   = "broker_accepted_request_and_active_lease_only"
	nodeConnectorManagedBrokerMaxEvidenceBytes  = 64 << 10
	nodeConnectorManagedBrokerMaxRecords        = 64
	nodeConnectorManagedBrokerMaxNodes          = 10_000
	nodeConnectorManagedBrokerMaxConcurrentWork = 100_000
	nodeConnectorManagedBrokerMaxArtifactBytes  = int64(1 << 40)
	nodeConnectorManagedBrokerMaxRetentionDays  = 3_650
)

var (
	nodeConnectorManagedBrokerStateName   = regexp.MustCompile(`^managed-broker-preview-state-([0-9]{12})\.json$`)
	nodeConnectorManagedBrokerWriteAtomic = writeJSONFileAtomic
)

// NodeConnectorManagedBrokerPreviewAuthority is deliberately all-negative.
// Managed-service evidence can describe a boundary but cannot authorize work.
type NodeConnectorManagedBrokerPreviewAuthority struct {
	LeaseGranted          bool `json:"lease_granted"`
	ExecutionAuthorized   bool `json:"execution_authorized"`
	ValidationAuthorized  bool `json:"validation_authorized"`
	MutationAuthorized    bool `json:"mutation_authorized"`
	GitAuthorized         bool `json:"git_authorized"`
	ApplyAuthorized       bool `json:"apply_authorized"`
	CheckpointAuthorized  bool `json:"checkpoint_authorized"`
	CommitAuthorized      bool `json:"commit_authorized"`
	PushAuthorized        bool `json:"push_authorized"`
	PublicationAuthorized bool `json:"publication_authorized"`
	CompletionProven      bool `json:"completion_proven"`
}

type NodeConnectorManagedBrokerPreviewExpectedBinding struct {
	TenantID             string `json:"tenant_id"`
	NodeID               string `json:"node_id"`
	MachineID            string `json:"machine_id"`
	CapabilitySnapshotID string `json:"capability_snapshot_id"`
	ConnectionID         string `json:"connection_id"`
	ProviderID           string `json:"provider_id"`
	OperationID          string `json:"operation_id"`
	RequestFingerprint   string `json:"request_fingerprint"`
	LeaseID              string `json:"lease_id"`
	ReceiptID            string `json:"receipt_id"`
}

type NodeConnectorManagedBrokerTenantIdentity struct {
	Schema              string `json:"schema"`
	TenantID            string `json:"tenant_id"`
	CreatedAt           string `json:"created_at"`
	IdentityFingerprint string `json:"identity_fingerprint"`
}

type NodeConnectorManagedBrokerQuotaSnapshot struct {
	Schema                 string                                     `json:"schema"`
	SnapshotID             string                                     `json:"snapshot_id"`
	TenantID               string                                     `json:"tenant_id"`
	MaxNodes               int                                        `json:"max_nodes"`
	MaxConcurrentWorkItems int                                        `json:"max_concurrent_work_items"`
	MaxArtifactBytes       int64                                      `json:"max_artifact_bytes"`
	ObservedAt             string                                     `json:"observed_at"`
	Enforced               bool                                       `json:"enforced"`
	Authority              NodeConnectorManagedBrokerPreviewAuthority `json:"authority"`
	EvidenceFingerprint    string                                     `json:"evidence_fingerprint"`
}

type NodeConnectorManagedBrokerAuditEvidence struct {
	Schema               string                                     `json:"schema"`
	EvidenceID           string                                     `json:"evidence_id"`
	TenantID             string                                     `json:"tenant_id"`
	NodeID               string                                     `json:"node_id"`
	MachineID            string                                     `json:"machine_id"`
	CapabilitySnapshotID string                                     `json:"capability_snapshot_id"`
	ConnectionID         string                                     `json:"connection_id"`
	ProviderID           string                                     `json:"provider_id"`
	OperationID          string                                     `json:"operation_id"`
	LeaseID              string                                     `json:"lease_id"`
	ReceiptID            string                                     `json:"receipt_id"`
	Kind                 string                                     `json:"kind"`
	ObservedAt           string                                     `json:"observed_at"`
	Authority            NodeConnectorManagedBrokerPreviewAuthority `json:"authority"`
	EvidenceFingerprint  string                                     `json:"evidence_fingerprint"`
}

type NodeConnectorManagedBrokerRetentionPolicy struct {
	Schema            string                                     `json:"schema"`
	PolicyID          string                                     `json:"policy_id"`
	TenantID          string                                     `json:"tenant_id"`
	AuditDays         int                                        `json:"audit_days"`
	ArtifactDays      int                                        `json:"artifact_days"`
	EffectiveAt       string                                     `json:"effective_at"`
	Enforced          bool                                       `json:"enforced"`
	Authority         NodeConnectorManagedBrokerPreviewAuthority `json:"authority"`
	PolicyFingerprint string                                     `json:"policy_fingerprint"`
}

type NodeConnectorManagedBrokerAvailabilityEvidence struct {
	Schema              string                                     `json:"schema"`
	EvidenceID          string                                     `json:"evidence_id"`
	TenantID            string                                     `json:"tenant_id"`
	NodeID              string                                     `json:"node_id"`
	MachineID           string                                     `json:"machine_id"`
	ConnectionID        string                                     `json:"connection_id"`
	ProviderID          string                                     `json:"provider_id"`
	Status              string                                     `json:"status"`
	ObservedAt          string                                     `json:"observed_at"`
	Authority           NodeConnectorManagedBrokerPreviewAuthority `json:"authority"`
	EvidenceFingerprint string                                     `json:"evidence_fingerprint"`
}

type NodeConnectorManagedBrokerSharedIngressBinding struct {
	Schema             string `json:"schema"`
	BindingID          string `json:"binding_id"`
	TenantID           string `json:"tenant_id"`
	NodeID             string `json:"node_id"`
	MachineID          string `json:"machine_id"`
	ConnectionID       string `json:"connection_id"`
	ProviderID         string `json:"provider_id"`
	Mode               string `json:"mode"`
	BindingFingerprint string `json:"binding_fingerprint"`
}

type NodeConnectorManagedBrokerExecutionAuthority struct {
	Source             string `json:"source"`
	OperationID        string `json:"operation_id"`
	RequestFingerprint string `json:"request_fingerprint"`
	LeaseID            string `json:"lease_id"`
}

// NodeConnectorManagedBrokerPreviewEvidence is an opaque fixture declaration.
// Validation proves only its internal bindings; it performs no managed service,
// quota, retention, availability, execution, validation, or lifecycle action.
type NodeConnectorManagedBrokerPreviewEvidence struct {
	Schema              string                                         `json:"schema"`
	PreviewID           string                                         `json:"preview_id"`
	ReplayIdentity      string                                         `json:"replay_identity"`
	Provenance          string                                         `json:"provenance"`
	Tenant              NodeConnectorManagedBrokerTenantIdentity       `json:"tenant"`
	Quota               NodeConnectorManagedBrokerQuotaSnapshot        `json:"quota"`
	Audit               NodeConnectorManagedBrokerAuditEvidence        `json:"audit"`
	Retention           NodeConnectorManagedBrokerRetentionPolicy      `json:"retention"`
	Availability        NodeConnectorManagedBrokerAvailabilityEvidence `json:"availability"`
	Ingress             NodeConnectorManagedBrokerSharedIngressBinding `json:"ingress"`
	ExecutionAuthority  NodeConnectorManagedBrokerExecutionAuthority   `json:"execution_authority"`
	Authority           NodeConnectorManagedBrokerPreviewAuthority     `json:"authority"`
	EvidenceFingerprint string                                         `json:"evidence_fingerprint"`
}

type nodeConnectorManagedBrokerPreviewState struct {
	Schema                   string                                           `json:"schema"`
	Generation               int64                                            `json:"generation"`
	PreviousStateFingerprint string                                           `json:"previous_state_fingerprint,omitempty"`
	Expected                 NodeConnectorManagedBrokerPreviewExpectedBinding `json:"expected"`
	Accepted                 []NodeConnectorManagedBrokerPreviewEvidence      `json:"accepted"`
	StateFingerprint         string                                           `json:"state_fingerprint"`
}

type NodeConnectorManagedBrokerPreview struct {
	root  string
	state nodeConnectorManagedBrokerPreviewState
	mu    sync.Mutex
}

func FinalizeNodeConnectorManagedBrokerPreviewEvidence(value NodeConnectorManagedBrokerPreviewEvidence) (NodeConnectorManagedBrokerPreviewEvidence, error) {
	value.Schema = NodeConnectorManagedBrokerEvidenceSchema
	value.Provenance = nodeConnectorManagedBrokerProvenance
	value.Authority = NodeConnectorManagedBrokerPreviewAuthority{}

	value.Tenant.Schema = NodeConnectorManagedBrokerTenantSchema
	value.Tenant.IdentityFingerprint = ""
	value.Tenant.IdentityFingerprint, _ = nodeExecutionFingerprintValue(value.Tenant)

	value.Quota.Schema = NodeConnectorManagedBrokerQuotaSchema
	value.Quota.Enforced = false
	value.Quota.Authority = NodeConnectorManagedBrokerPreviewAuthority{}
	value.Quota.EvidenceFingerprint = ""
	value.Quota.EvidenceFingerprint, _ = nodeExecutionFingerprintValue(value.Quota)

	value.Audit.Schema = NodeConnectorManagedBrokerAuditSchema
	value.Audit.Kind = "activity_observed"
	value.Audit.Authority = NodeConnectorManagedBrokerPreviewAuthority{}
	value.Audit.EvidenceFingerprint = ""
	value.Audit.EvidenceFingerprint, _ = nodeExecutionFingerprintValue(value.Audit)

	value.Retention.Schema = NodeConnectorManagedBrokerRetentionSchema
	value.Retention.Enforced = false
	value.Retention.Authority = NodeConnectorManagedBrokerPreviewAuthority{}
	value.Retention.PolicyFingerprint = ""
	value.Retention.PolicyFingerprint, _ = nodeExecutionFingerprintValue(value.Retention)

	value.Availability.Schema = NodeConnectorManagedBrokerAvailabilitySchema
	value.Availability.Authority = NodeConnectorManagedBrokerPreviewAuthority{}
	value.Availability.EvidenceFingerprint = ""
	value.Availability.EvidenceFingerprint, _ = nodeExecutionFingerprintValue(value.Availability)

	value.Ingress.Schema = NodeConnectorManagedBrokerIngressSchema
	value.Ingress.Mode = NodeConnectorManagedBrokerIngressMode
	value.Ingress.BindingFingerprint = ""
	value.Ingress.BindingFingerprint, _ = nodeExecutionFingerprintValue(value.Ingress)

	value.ExecutionAuthority.Source = nodeConnectorManagedBrokerAuthoritySource
	value.EvidenceFingerprint = ""
	fingerprint, err := nodeExecutionFingerprintValue(value)
	if err != nil {
		return NodeConnectorManagedBrokerPreviewEvidence{}, err
	}
	value.EvidenceFingerprint = fingerprint
	if err := validateNodeConnectorManagedBrokerPreviewEvidence(value); err != nil {
		return NodeConnectorManagedBrokerPreviewEvidence{}, err
	}
	return value, nil
}

func EncodeNodeConnectorManagedBrokerPreviewEvidence(value NodeConnectorManagedBrokerPreviewEvidence) ([]byte, error) {
	finalized, err := FinalizeNodeConnectorManagedBrokerPreviewEvidence(value)
	if err != nil {
		return nil, err
	}
	return json.Marshal(finalized)
}

func OpenNodeConnectorManagedBrokerPreview(root string, expected NodeConnectorManagedBrokerPreviewExpectedBinding) (*NodeConnectorManagedBrokerPreview, error) {
	if err := validateNodeConnectorManagedBrokerExpectedBinding(expected); err != nil {
		return nil, err
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("managed broker preview root must be an existing regular directory")
	}
	states, err := loadNodeConnectorManagedBrokerPreviewStates(root)
	if err != nil {
		return nil, err
	}
	preview := &NodeConnectorManagedBrokerPreview{root: root}
	if len(states) == 0 {
		preview.state = nodeConnectorManagedBrokerPreviewState{
			Schema: nodeConnectorManagedBrokerStateSchema, Generation: 1, Expected: expected,
			Accepted: []NodeConnectorManagedBrokerPreviewEvidence{},
		}
		if err := finalizeNodeConnectorManagedBrokerState(&preview.state); err != nil {
			return nil, err
		}
		if err := nodeConnectorManagedBrokerWriteAtomic(filepath.Join(root, nodeConnectorManagedBrokerStateFileName(1)), preview.state); err != nil {
			return nil, errors.New("managed broker preview state could not be published")
		}
		return preview, nil
	}
	preview.state = states[len(states)-1]
	if !nodeExecutionEqual(preview.state.Expected, expected) {
		return nil, errors.New("managed broker preview expected identity binding conflicts with durable state")
	}
	return preview, nil
}

func (preview *NodeConnectorManagedBrokerPreview) Accept(raw []byte) (NodeConnectorManagedBrokerPreviewEvidence, error) {
	preview.mu.Lock()
	defer preview.mu.Unlock()
	if len(raw) == 0 || len(raw) > nodeConnectorManagedBrokerMaxEvidenceBytes || containsNodeExecutionSecret(string(raw)) {
		return NodeConnectorManagedBrokerPreviewEvidence{}, errors.New("managed broker preview evidence is malformed or contains forbidden material")
	}
	var evidence NodeConnectorManagedBrokerPreviewEvidence
	if err := decodeNodeExecutionCanonical(raw, &evidence); err != nil {
		return NodeConnectorManagedBrokerPreviewEvidence{}, errors.New("managed broker preview evidence is not strict canonical JSON")
	}
	if err := validateNodeConnectorManagedBrokerPreviewEvidence(evidence); err != nil {
		return NodeConnectorManagedBrokerPreviewEvidence{}, err
	}
	if err := validateNodeConnectorManagedBrokerPreviewBinding(evidence, preview.state.Expected); err != nil {
		return NodeConnectorManagedBrokerPreviewEvidence{}, err
	}
	for _, accepted := range preview.state.Accepted {
		sameID := accepted.PreviewID == evidence.PreviewID
		sameReplay := accepted.ReplayIdentity == evidence.ReplayIdentity
		if sameID && sameReplay && nodeExecutionEqual(accepted, evidence) {
			return accepted, nil
		}
		if sameID || sameReplay {
			return NodeConnectorManagedBrokerPreviewEvidence{}, errors.New("managed broker preview conflicting replay is rejected")
		}
	}
	if len(preview.state.Accepted) >= nodeConnectorManagedBrokerMaxRecords {
		return NodeConnectorManagedBrokerPreviewEvidence{}, errors.New("managed broker preview evidence limit reached")
	}
	next := cloneNodeConnectorManagedBrokerState(preview.state)
	next.Accepted = append(next.Accepted, evidence)
	sort.Slice(next.Accepted, func(i, j int) bool { return next.Accepted[i].PreviewID < next.Accepted[j].PreviewID })
	if err := preview.persist(next); err != nil {
		return NodeConnectorManagedBrokerPreviewEvidence{}, err
	}
	return evidence, nil
}

func (preview *NodeConnectorManagedBrokerPreview) Generation() int64 {
	preview.mu.Lock()
	defer preview.mu.Unlock()
	return preview.state.Generation
}

func (preview *NodeConnectorManagedBrokerPreview) AcceptedCount() int {
	preview.mu.Lock()
	defer preview.mu.Unlock()
	return len(preview.state.Accepted)
}

func (preview *NodeConnectorManagedBrokerPreview) persist(next nodeConnectorManagedBrokerPreviewState) error {
	next.Generation = preview.state.Generation + 1
	next.PreviousStateFingerprint = preview.state.StateFingerprint
	next.StateFingerprint = ""
	if err := finalizeNodeConnectorManagedBrokerState(&next); err != nil {
		return err
	}
	path := filepath.Join(preview.root, nodeConnectorManagedBrokerStateFileName(next.Generation))
	if _, err := os.Lstat(path); err == nil {
		return errors.New("next managed broker preview state already exists")
	} else if !os.IsNotExist(err) {
		return errors.New("managed broker preview state path could not be inspected")
	}
	if err := nodeConnectorManagedBrokerWriteAtomic(path, next); err != nil {
		return errors.New("managed broker preview state could not be published")
	}
	preview.state = next
	return nil
}

func validateNodeConnectorManagedBrokerExpectedBinding(value NodeConnectorManagedBrokerPreviewExpectedBinding) error {
	typed := []struct{ kind, value string }{
		{"tenant", value.TenantID}, {"node", value.NodeID}, {"machine", value.MachineID},
		{"connection", value.ConnectionID}, {"provider", value.ProviderID}, {"operation", value.OperationID},
		{"lease", value.LeaseID}, {"receipt", value.ReceiptID},
	}
	for _, identity := range typed {
		if err := validateNodeExecutionTypedID(identity.kind, identity.value); err != nil {
			return errors.New("managed broker preview expected identity is invalid")
		}
	}
	if !nodeExecutionFingerprint.MatchString(value.CapabilitySnapshotID) || !nodeExecutionFingerprint.MatchString(value.RequestFingerprint) {
		return errors.New("managed broker preview expected fingerprint identity is invalid")
	}
	identities := []string{value.TenantID, value.NodeID, value.MachineID, value.CapabilitySnapshotID, value.ConnectionID, value.ProviderID, value.OperationID, value.RequestFingerprint, value.LeaseID, value.ReceiptID}
	if !nodeConnectorManagedBrokerDistinct(identities) {
		return errors.New("managed broker preview identities must remain strictly separate")
	}
	return nil
}

func validateNodeConnectorManagedBrokerPreviewEvidence(value NodeConnectorManagedBrokerPreviewEvidence) error {
	if value.Schema != NodeConnectorManagedBrokerEvidenceSchema || value.Provenance != nodeConnectorManagedBrokerProvenance {
		return errors.New("managed broker preview contract or provenance is invalid")
	}
	for _, identity := range []struct{ kind, value string }{{"preview", value.PreviewID}, {"replay", value.ReplayIdentity}} {
		if err := validateNodeExecutionTypedID(identity.kind, identity.value); err != nil {
			return errors.New("managed broker preview identity is invalid")
		}
	}
	if value.Authority != (NodeConnectorManagedBrokerPreviewAuthority{}) {
		return errors.New("managed broker preview evidence cannot grant authority or prove completion")
	}
	if err := validateNodeConnectorManagedBrokerTenant(value.Tenant); err != nil {
		return err
	}
	if err := validateNodeConnectorManagedBrokerQuota(value.Quota); err != nil {
		return err
	}
	if err := validateNodeConnectorManagedBrokerAudit(value.Audit); err != nil {
		return err
	}
	if err := validateNodeConnectorManagedBrokerRetention(value.Retention); err != nil {
		return err
	}
	if err := validateNodeConnectorManagedBrokerAvailability(value.Availability); err != nil {
		return err
	}
	if err := validateNodeConnectorManagedBrokerIngress(value.Ingress); err != nil {
		return err
	}
	if value.ExecutionAuthority.Source != nodeConnectorManagedBrokerAuthoritySource {
		return errors.New("managed broker preview execution authority source is invalid")
	}
	if err := validateNodeExecutionTypedID("operation", value.ExecutionAuthority.OperationID); err != nil ||
		!nodeExecutionFingerprint.MatchString(value.ExecutionAuthority.RequestFingerprint) ||
		validateNodeExecutionTypedID("lease", value.ExecutionAuthority.LeaseID) != nil {
		return errors.New("managed broker preview broker authority binding is invalid")
	}
	expectedFingerprint := value.EvidenceFingerprint
	value.EvidenceFingerprint = ""
	actualFingerprint, err := nodeExecutionFingerprintValue(value)
	if err != nil || expectedFingerprint != actualFingerprint {
		return errors.New("managed broker preview evidence fingerprint is invalid")
	}
	identities := []string{
		value.PreviewID, value.ReplayIdentity, value.Tenant.TenantID, value.Quota.SnapshotID,
		value.Audit.EvidenceID, value.Retention.PolicyID, value.Availability.EvidenceID, value.Ingress.BindingID,
		value.Audit.NodeID, value.Audit.MachineID, value.Audit.CapabilitySnapshotID, value.Audit.ConnectionID,
		value.Audit.ProviderID, value.Audit.OperationID, value.Audit.LeaseID, value.Audit.ReceiptID,
		value.ExecutionAuthority.RequestFingerprint,
	}
	if !nodeConnectorManagedBrokerDistinct(identities) {
		return errors.New("managed broker preview identities must remain strictly separate")
	}
	return nil
}

func validateNodeConnectorManagedBrokerTenant(value NodeConnectorManagedBrokerTenantIdentity) error {
	if value.Schema != NodeConnectorManagedBrokerTenantSchema || validateNodeExecutionTypedID("tenant", value.TenantID) != nil {
		return errors.New("managed broker tenant identity is invalid")
	}
	if _, err := parseNodeExecutionTime(value.CreatedAt); err != nil {
		return errors.New("managed broker tenant identity time is invalid")
	}
	expected := value.IdentityFingerprint
	value.IdentityFingerprint = ""
	actual, _ := nodeExecutionFingerprintValue(value)
	if expected != actual {
		return errors.New("managed broker tenant identity fingerprint is invalid")
	}
	return nil
}

func validateNodeConnectorManagedBrokerQuota(value NodeConnectorManagedBrokerQuotaSnapshot) error {
	if value.Schema != NodeConnectorManagedBrokerQuotaSchema || validateNodeExecutionTypedID("quota", value.SnapshotID) != nil || validateNodeExecutionTypedID("tenant", value.TenantID) != nil {
		return errors.New("managed broker quota identity is invalid")
	}
	if value.MaxNodes < 1 || value.MaxNodes > nodeConnectorManagedBrokerMaxNodes || value.MaxConcurrentWorkItems < 1 || value.MaxConcurrentWorkItems > nodeConnectorManagedBrokerMaxConcurrentWork || value.MaxArtifactBytes < 1 || value.MaxArtifactBytes > nodeConnectorManagedBrokerMaxArtifactBytes {
		return errors.New("managed broker quota declaration is invalid or unbounded")
	}
	if _, err := parseNodeExecutionTime(value.ObservedAt); err != nil {
		return errors.New("managed broker quota time is invalid")
	}
	if value.Enforced || value.Authority != (NodeConnectorManagedBrokerPreviewAuthority{}) {
		return errors.New("managed broker quota evidence cannot enforce or authorize work")
	}
	expected := value.EvidenceFingerprint
	value.EvidenceFingerprint = ""
	actual, _ := nodeExecutionFingerprintValue(value)
	if expected != actual {
		return errors.New("managed broker quota fingerprint is invalid")
	}
	return nil
}

func validateNodeConnectorManagedBrokerAudit(value NodeConnectorManagedBrokerAuditEvidence) error {
	if value.Schema != NodeConnectorManagedBrokerAuditSchema || value.Kind != "activity_observed" || value.Authority != (NodeConnectorManagedBrokerPreviewAuthority{}) {
		return errors.New("managed broker audit evidence cannot authorize work or prove completion")
	}
	for _, identity := range []struct{ kind, value string }{{"audit", value.EvidenceID}, {"tenant", value.TenantID}, {"node", value.NodeID}, {"machine", value.MachineID}, {"connection", value.ConnectionID}, {"provider", value.ProviderID}, {"operation", value.OperationID}, {"lease", value.LeaseID}, {"receipt", value.ReceiptID}} {
		if validateNodeExecutionTypedID(identity.kind, identity.value) != nil {
			return errors.New("managed broker audit identity is invalid")
		}
	}
	if !nodeExecutionFingerprint.MatchString(value.CapabilitySnapshotID) {
		return errors.New("managed broker audit capability identity is invalid")
	}
	if _, err := parseNodeExecutionTime(value.ObservedAt); err != nil {
		return errors.New("managed broker audit time is invalid")
	}
	expected := value.EvidenceFingerprint
	value.EvidenceFingerprint = ""
	actual, _ := nodeExecutionFingerprintValue(value)
	if expected != actual {
		return errors.New("managed broker audit fingerprint is invalid")
	}
	return nil
}

func validateNodeConnectorManagedBrokerRetention(value NodeConnectorManagedBrokerRetentionPolicy) error {
	if value.Schema != NodeConnectorManagedBrokerRetentionSchema || validateNodeExecutionTypedID("retention", value.PolicyID) != nil || validateNodeExecutionTypedID("tenant", value.TenantID) != nil {
		return errors.New("managed broker retention identity is invalid")
	}
	if value.AuditDays < 1 || value.AuditDays > nodeConnectorManagedBrokerMaxRetentionDays || value.ArtifactDays < 1 || value.ArtifactDays > nodeConnectorManagedBrokerMaxRetentionDays {
		return errors.New("managed broker retention declaration is invalid or unbounded")
	}
	if _, err := parseNodeExecutionTime(value.EffectiveAt); err != nil {
		return errors.New("managed broker retention time is invalid")
	}
	if value.Enforced || value.Authority != (NodeConnectorManagedBrokerPreviewAuthority{}) {
		return errors.New("managed broker retention evidence cannot enforce or authorize work")
	}
	expected := value.PolicyFingerprint
	value.PolicyFingerprint = ""
	actual, _ := nodeExecutionFingerprintValue(value)
	if expected != actual {
		return errors.New("managed broker retention fingerprint is invalid")
	}
	return nil
}

func validateNodeConnectorManagedBrokerAvailability(value NodeConnectorManagedBrokerAvailabilityEvidence) error {
	if value.Schema != NodeConnectorManagedBrokerAvailabilitySchema || (value.Status != "available" && value.Status != "degraded" && value.Status != "unavailable") || value.Authority != (NodeConnectorManagedBrokerPreviewAuthority{}) {
		return errors.New("managed broker availability evidence cannot authorize work or prove completion")
	}
	for _, identity := range []struct{ kind, value string }{{"availability", value.EvidenceID}, {"tenant", value.TenantID}, {"node", value.NodeID}, {"machine", value.MachineID}, {"connection", value.ConnectionID}, {"provider", value.ProviderID}} {
		if validateNodeExecutionTypedID(identity.kind, identity.value) != nil {
			return errors.New("managed broker availability identity is invalid")
		}
	}
	if _, err := parseNodeExecutionTime(value.ObservedAt); err != nil {
		return errors.New("managed broker availability time is invalid")
	}
	expected := value.EvidenceFingerprint
	value.EvidenceFingerprint = ""
	actual, _ := nodeExecutionFingerprintValue(value)
	if expected != actual {
		return errors.New("managed broker availability fingerprint is invalid")
	}
	return nil
}

func validateNodeConnectorManagedBrokerIngress(value NodeConnectorManagedBrokerSharedIngressBinding) error {
	if value.Schema != NodeConnectorManagedBrokerIngressSchema || value.Mode != NodeConnectorManagedBrokerIngressMode {
		return errors.New("managed broker ingress must use shared tenant and node multiplexing")
	}
	for _, identity := range []struct{ kind, value string }{{"ingress", value.BindingID}, {"tenant", value.TenantID}, {"node", value.NodeID}, {"machine", value.MachineID}, {"connection", value.ConnectionID}, {"provider", value.ProviderID}} {
		if validateNodeExecutionTypedID(identity.kind, identity.value) != nil {
			return errors.New("managed broker ingress identity is invalid")
		}
	}
	expected := value.BindingFingerprint
	value.BindingFingerprint = ""
	actual, _ := nodeExecutionFingerprintValue(value)
	if expected != actual {
		return errors.New("managed broker ingress fingerprint is invalid")
	}
	return nil
}

func validateNodeConnectorManagedBrokerPreviewBinding(value NodeConnectorManagedBrokerPreviewEvidence, expected NodeConnectorManagedBrokerPreviewExpectedBinding) error {
	if value.Tenant.TenantID != expected.TenantID || value.Quota.TenantID != expected.TenantID || value.Audit.TenantID != expected.TenantID || value.Retention.TenantID != expected.TenantID || value.Availability.TenantID != expected.TenantID || value.Ingress.TenantID != expected.TenantID ||
		value.Audit.NodeID != expected.NodeID || value.Availability.NodeID != expected.NodeID || value.Ingress.NodeID != expected.NodeID ||
		value.Audit.MachineID != expected.MachineID || value.Availability.MachineID != expected.MachineID || value.Ingress.MachineID != expected.MachineID ||
		value.Audit.CapabilitySnapshotID != expected.CapabilitySnapshotID ||
		value.Audit.ConnectionID != expected.ConnectionID || value.Availability.ConnectionID != expected.ConnectionID || value.Ingress.ConnectionID != expected.ConnectionID ||
		value.Audit.ProviderID != expected.ProviderID || value.Availability.ProviderID != expected.ProviderID || value.Ingress.ProviderID != expected.ProviderID ||
		value.Audit.OperationID != expected.OperationID || value.ExecutionAuthority.OperationID != expected.OperationID ||
		value.ExecutionAuthority.RequestFingerprint != expected.RequestFingerprint ||
		value.Audit.LeaseID != expected.LeaseID || value.ExecutionAuthority.LeaseID != expected.LeaseID ||
		value.Audit.ReceiptID != expected.ReceiptID {
		return errors.New("managed broker preview contains an unknown, substituted, or cross-tenant identity")
	}
	return nil
}

func loadNodeConnectorManagedBrokerPreviewStates(root string) ([]nodeConnectorManagedBrokerPreviewState, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	names := []string{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasPrefix(entry.Name(), "managed-broker-preview-state-") {
			if !nodeConnectorManagedBrokerStateName.MatchString(entry.Name()) {
				return nil, errors.New("managed broker preview state artifact name is malformed")
			}
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	states := make([]nodeConnectorManagedBrokerPreviewState, 0, len(names))
	previous := ""
	for index, name := range names {
		raw, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			return nil, errors.New("managed broker preview state cannot be read")
		}
		var state nodeConnectorManagedBrokerPreviewState
		if err := decodeNodeExecutionStrict(raw, &state); err != nil {
			return nil, errors.New("managed broker preview state is malformed")
		}
		if state.Generation != int64(index+1) || name != nodeConnectorManagedBrokerStateFileName(state.Generation) || state.PreviousStateFingerprint != previous {
			return nil, errors.New("managed broker preview state chain is not contiguous")
		}
		if err := validateNodeConnectorManagedBrokerState(state); err != nil {
			return nil, err
		}
		previous = state.StateFingerprint
		states = append(states, state)
	}
	return states, nil
}

func finalizeNodeConnectorManagedBrokerState(state *nodeConnectorManagedBrokerPreviewState) error {
	state.StateFingerprint = ""
	fingerprint, err := nodeExecutionFingerprintValue(*state)
	if err != nil {
		return err
	}
	state.StateFingerprint = fingerprint
	return validateNodeConnectorManagedBrokerState(*state)
}

func validateNodeConnectorManagedBrokerState(state nodeConnectorManagedBrokerPreviewState) error {
	if state.Schema != nodeConnectorManagedBrokerStateSchema || state.Generation < 1 || len(state.Accepted) > nodeConnectorManagedBrokerMaxRecords {
		return errors.New("managed broker preview state contract is invalid")
	}
	if (state.Generation == 1 && state.PreviousStateFingerprint != "") || (state.Generation > 1 && !nodeExecutionFingerprint.MatchString(state.PreviousStateFingerprint)) {
		return errors.New("managed broker preview state chain binding is invalid")
	}
	if err := validateNodeConnectorManagedBrokerExpectedBinding(state.Expected); err != nil {
		return err
	}
	last := ""
	previewIDs, replayIDs := map[string]bool{}, map[string]bool{}
	for _, evidence := range state.Accepted {
		if evidence.PreviewID <= last || previewIDs[evidence.PreviewID] || replayIDs[evidence.ReplayIdentity] {
			return errors.New("managed broker preview accepted evidence is duplicated or unsorted")
		}
		if err := validateNodeConnectorManagedBrokerPreviewEvidence(evidence); err != nil {
			return err
		}
		if err := validateNodeConnectorManagedBrokerPreviewBinding(evidence, state.Expected); err != nil {
			return err
		}
		last = evidence.PreviewID
		previewIDs[evidence.PreviewID], replayIDs[evidence.ReplayIdentity] = true, true
	}
	expected := state.StateFingerprint
	state.StateFingerprint = ""
	actual, _ := nodeExecutionFingerprintValue(state)
	if expected != actual {
		return errors.New("managed broker preview state fingerprint is invalid")
	}
	return nil
}

func nodeConnectorManagedBrokerStateFileName(generation int64) string {
	return fmt.Sprintf("managed-broker-preview-state-%012d.json", generation)
}

func cloneNodeConnectorManagedBrokerState(value nodeConnectorManagedBrokerPreviewState) nodeConnectorManagedBrokerPreviewState {
	raw, _ := json.Marshal(value)
	var cloned nodeConnectorManagedBrokerPreviewState
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func nodeConnectorManagedBrokerDistinct(values []string) bool {
	seen := map[string]bool{}
	for _, value := range values {
		if value == "" || seen[value] {
			return false
		}
		seen[value] = true
	}
	return true
}
