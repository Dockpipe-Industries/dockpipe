package orchestrationhelper

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const (
	NodeConnectorInventorySnapshotFixtureSchema      = "dorkpipe.node-inventory-snapshot-fixture/v1"
	NodeConnectorInventorySnapshotSchema             = "dorkpipe.node-inventory-snapshot/v1"
	NodeConnectorPlacementInputSnapshotFixtureSchema = "dorkpipe.node-placement-input-snapshot-fixture/v1"
	NodeConnectorPlacementInputSnapshotSchema        = "dorkpipe.node-placement-input-snapshot/v1"

	nodeConnectorInventoryProvenance               = "fixture_only_node_inventory_snapshot"
	nodeConnectorPlacementInputProvenance          = "fixture_only_node_placement_input_snapshot"
	nodeConnectorInventoryArtifactName             = "node-inventory-snapshot.json"
	nodeConnectorPlacementInputArtifactName        = "node-placement-input-snapshot.json"
	nodeConnectorInventoryMaxFixtureBytes          = 256 << 10
	nodeConnectorInventoryMaxArtifactBytes         = 256 << 10
	nodeConnectorInventoryMaxNodes                 = 64
	nodeConnectorInventoryMaxReferencesPerNode     = 16
	nodeConnectorInventoryMaxReferenceBytes        = int64(32 << 20)
	nodeConnectorInventoryMaxReferenceAggregate    = int64(128 << 20)
	nodeConnectorInventoryMaxUtilizationBasisPoint = 10_000
	nodeConnectorInventoryMaxActiveTasks           = 1_000_000
	nodeConnectorInventoryMaxNormalizedCost        = 1_000_000
	nodeConnectorInventoryCostUnit                 = "normalized_placement_cost_unit"
)

var (
	nodeConnectorInventoryWriteAtomic      = writeJSONFileAtomic
	nodeConnectorPlacementInputWriteAtomic = writeJSONFileAtomic
)

type NodeConnectorInventoryTargetProfile struct {
	HostOS  string `json:"host_os"`
	Runtime string `json:"runtime"`
	GuestOS string `json:"guest_os"`
}

type NodeConnectorInventoryAvailabilityEvidence struct {
	Classification string `json:"classification"`
}

type NodeConnectorInventoryLoadEvidence struct {
	CPUUtilizationBasisPoints    int `json:"cpu_utilization_basis_points"`
	MemoryUtilizationBasisPoints int `json:"memory_utilization_basis_points"`
	ActiveTaskCount              int `json:"active_task_count"`
}

type NodeConnectorInventoryRiskEvidence struct {
	Classification string `json:"classification"`
}

type NodeConnectorInventoryCostEvidence struct {
	Classification  string `json:"classification"`
	NormalizedValue *int   `json:"normalized_value,omitempty"`
	LogicalUnit     string `json:"logical_unit,omitempty"`
}

type NodeConnectorInventoryEvidenceReference struct {
	Name      string `json:"name"`
	MediaType string `json:"media_type"`
	Digest    string `json:"digest"`
	Bytes     int64  `json:"bytes"`
}

type NodeConnectorInventoryNodeBinding struct {
	NodeID                        string                                    `json:"node_id"`
	MachineID                     string                                    `json:"machine_id"`
	CapabilitySnapshotID          string                                    `json:"capability_snapshot_id"`
	CapabilitySnapshotFingerprint string                                    `json:"capability_snapshot_fingerprint"`
	Profile                       NodeConnectorInventoryTargetProfile       `json:"profile"`
	References                    []NodeConnectorInventoryEvidenceReference `json:"references"`
}

type NodeConnectorInventoryNodeEvidence struct {
	NodeID                        string                                     `json:"node_id"`
	MachineID                     string                                     `json:"machine_id"`
	CapabilitySnapshotID          string                                     `json:"capability_snapshot_id"`
	CapabilitySnapshotFingerprint string                                     `json:"capability_snapshot_fingerprint"`
	Profile                       NodeConnectorInventoryTargetProfile        `json:"profile"`
	Availability                  NodeConnectorInventoryAvailabilityEvidence `json:"availability"`
	Load                          NodeConnectorInventoryLoadEvidence         `json:"load"`
	Risk                          NodeConnectorInventoryRiskEvidence         `json:"risk"`
	Cost                          NodeConnectorInventoryCostEvidence         `json:"cost"`
	References                    []NodeConnectorInventoryEvidenceReference  `json:"references"`
}

type NodeConnectorPlacementInputExpected struct {
	WorkloadID              string   `json:"workload_id"`
	RequirementsFingerprint string   `json:"requirements_fingerprint"`
	CandidateNodeIDs        []string `json:"candidate_node_ids"`
}

type NodeConnectorInventorySnapshotExpected struct {
	Nodes          []NodeConnectorInventoryNodeBinding `json:"nodes"`
	PlacementInput NodeConnectorPlacementInputExpected `json:"placement_input"`
}

// NodeConnectorInventoryAuthority is deliberately all false. Inventory and
// placement-input evidence describe immutable inputs but authorize no action.
type NodeConnectorInventoryAuthority struct {
	Discovery        bool `json:"discovery"`
	Refresh          bool `json:"refresh"`
	Matching         bool `json:"matching"`
	Scoring          bool `json:"scoring"`
	Ranking          bool `json:"ranking"`
	Selection        bool `json:"selection"`
	Reservation      bool `json:"reservation"`
	Placement        bool `json:"placement"`
	Lease            bool `json:"lease"`
	Dispatch         bool `json:"dispatch"`
	Execution        bool `json:"execution"`
	Validation       bool `json:"validation"`
	Retry            bool `json:"retry"`
	Repair           bool `json:"repair"`
	Quarantine       bool `json:"quarantine"`
	Transport        bool `json:"transport"`
	Provider         bool `json:"provider"`
	Network          bool `json:"network"`
	Service          bool `json:"service"`
	Billing          bool `json:"billing"`
	QuotaEnforcement bool `json:"quota_enforcement"`
	Mutation         bool `json:"mutation"`
	Git              bool `json:"git"`
	Apply            bool `json:"apply"`
	Checkpoint       bool `json:"checkpoint"`
	Commit           bool `json:"commit"`
	Push             bool `json:"push"`
	Publication      bool `json:"publication"`
	Lifecycle        bool `json:"lifecycle"`
	Completion       bool `json:"completion"`
}

type NodeConnectorInventorySnapshotFixture struct {
	Schema              string                               `json:"schema"`
	InventorySnapshotID string                               `json:"inventory_snapshot_id"`
	ReplayIdentity      string                               `json:"replay_identity"`
	ObservedAt          string                               `json:"observed_at"`
	Nodes               []NodeConnectorInventoryNodeEvidence `json:"nodes"`
	Provenance          string                               `json:"provenance"`
}

type NodeConnectorInventorySnapshot struct {
	Schema                       string                               `json:"schema"`
	InventorySnapshotID          string                               `json:"inventory_snapshot_id"`
	ReplayIdentity               string                               `json:"replay_identity"`
	ObservedAt                   string                               `json:"observed_at"`
	NodeCount                    int                                  `json:"node_count"`
	Nodes                        []NodeConnectorInventoryNodeEvidence `json:"nodes"`
	Provenance                   string                               `json:"provenance"`
	FixtureOwned                 bool                                 `json:"fixture_owned"`
	EvidenceAuthoritative        bool                                 `json:"evidence_authoritative"`
	Authority                    NodeConnectorInventoryAuthority      `json:"authority"`
	InventorySnapshotFingerprint string                               `json:"inventory_snapshot_fingerprint"`
}

type NodeConnectorPlacementInputSnapshotFixture struct {
	Schema                       string   `json:"schema"`
	PlacementInputID             string   `json:"placement_input_id"`
	ReplayIdentity               string   `json:"replay_identity"`
	InventorySnapshotID          string   `json:"inventory_snapshot_id"`
	InventorySnapshotFingerprint string   `json:"inventory_snapshot_fingerprint"`
	WorkloadID                   string   `json:"workload_id"`
	RequirementsFingerprint      string   `json:"requirements_fingerprint"`
	CandidateNodeIDs             []string `json:"candidate_node_ids"`
	Provenance                   string   `json:"provenance"`
}

type NodeConnectorPlacementInputSnapshot struct {
	Schema                            string                          `json:"schema"`
	PlacementInputID                  string                          `json:"placement_input_id"`
	ReplayIdentity                    string                          `json:"replay_identity"`
	InventorySnapshotID               string                          `json:"inventory_snapshot_id"`
	InventorySnapshotFingerprint      string                          `json:"inventory_snapshot_fingerprint"`
	WorkloadID                        string                          `json:"workload_id"`
	RequirementsFingerprint           string                          `json:"requirements_fingerprint"`
	CandidateNodeCount                int                             `json:"candidate_node_count"`
	CandidateNodeIDs                  []string                        `json:"candidate_node_ids"`
	CompleteInventoryCandidateSet     bool                            `json:"complete_inventory_candidate_set"`
	Provenance                        string                          `json:"provenance"`
	FixtureOwned                      bool                            `json:"fixture_owned"`
	EvidenceAuthoritative             bool                            `json:"evidence_authoritative"`
	Authority                         NodeConnectorInventoryAuthority `json:"authority"`
	PlacementInputSnapshotFingerprint string                          `json:"placement_input_snapshot_fingerprint"`
}

type NodeConnectorInventorySnapshots struct {
	root      string
	expected  NodeConnectorInventorySnapshotExpected
	inventory *NodeConnectorInventorySnapshot
	placement *NodeConnectorPlacementInputSnapshot
	mu        sync.Mutex
}

func OpenNodeConnectorInventorySnapshots(root string, expected NodeConnectorInventorySnapshotExpected) (*NodeConnectorInventorySnapshots, error) {
	normalized, err := normalizeNodeConnectorInventoryExpected(expected)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("node inventory snapshot root must be an existing regular directory")
	}
	snapshots := &NodeConnectorInventorySnapshots{root: root, expected: normalized}
	inventory, inventoryExists, err := loadNodeConnectorInventorySnapshot(root, normalized)
	if err != nil {
		return nil, err
	}
	placement, placementExists, err := loadNodeConnectorPlacementInputSnapshot(root, normalized, inventory, inventoryExists)
	if err != nil {
		return nil, err
	}
	if placementExists && !inventoryExists {
		return nil, errors.New("node placement-input snapshot exists without its exact durable inventory snapshot")
	}
	if inventoryExists {
		snapshots.inventory = &inventory
	}
	if placementExists {
		snapshots.placement = &placement
	}
	return snapshots, nil
}

func (snapshots *NodeConnectorInventorySnapshots) RecordInventory(raw []byte) (NodeConnectorInventorySnapshot, error) {
	snapshots.mu.Lock()
	defer snapshots.mu.Unlock()
	if len(raw) == 0 || len(raw) > nodeConnectorInventoryMaxFixtureBytes {
		return NodeConnectorInventorySnapshot{}, errors.New("node inventory snapshot fixture exceeds its encoded bound")
	}
	var fixture NodeConnectorInventorySnapshotFixture
	if err := decodeNodeExecutionCanonical(raw, &fixture); err != nil {
		return NodeConnectorInventorySnapshot{}, errors.New("node inventory snapshot fixture is not strict canonical JSON")
	}
	inventory, err := deriveNodeConnectorInventorySnapshot(snapshots.expected, fixture)
	if err != nil {
		return NodeConnectorInventorySnapshot{}, err
	}
	if snapshots.inventory != nil {
		if !nodeExecutionEqual(*snapshots.inventory, inventory) {
			return NodeConnectorInventorySnapshot{}, errors.New("changed or conflicting node inventory snapshot replay is rejected")
		}
		return cloneNodeConnectorInventorySnapshot(*snapshots.inventory), nil
	}
	path := filepath.Join(snapshots.root, nodeConnectorInventoryArtifactName)
	if err := requireNodeConnectorInventoryArtifactAbsent(path, "inventory snapshot"); err != nil {
		return NodeConnectorInventorySnapshot{}, err
	}
	if err := nodeConnectorInventoryWriteAtomic(path, inventory); err != nil {
		return NodeConnectorInventorySnapshot{}, errors.New("node inventory snapshot could not be published")
	}
	snapshots.inventory = &inventory
	return cloneNodeConnectorInventorySnapshot(inventory), nil
}

func (snapshots *NodeConnectorInventorySnapshots) RecordPlacementInput(raw []byte) (NodeConnectorPlacementInputSnapshot, error) {
	snapshots.mu.Lock()
	defer snapshots.mu.Unlock()
	if len(raw) == 0 || len(raw) > nodeConnectorInventoryMaxFixtureBytes {
		return NodeConnectorPlacementInputSnapshot{}, errors.New("node placement-input snapshot fixture exceeds its encoded bound")
	}
	if snapshots.inventory == nil {
		return NodeConnectorPlacementInputSnapshot{}, errors.New("node placement-input snapshot requires an exact durable inventory snapshot")
	}
	inventory, exists, err := loadNodeConnectorInventorySnapshot(snapshots.root, snapshots.expected)
	if err != nil || !exists || !nodeExecutionEqual(inventory, *snapshots.inventory) {
		return NodeConnectorPlacementInputSnapshot{}, errors.New("node placement-input snapshot could not revalidate its durable inventory snapshot")
	}
	var fixture NodeConnectorPlacementInputSnapshotFixture
	if err := decodeNodeExecutionCanonical(raw, &fixture); err != nil {
		return NodeConnectorPlacementInputSnapshot{}, errors.New("node placement-input snapshot fixture is not strict canonical JSON")
	}
	placement, err := deriveNodeConnectorPlacementInputSnapshot(snapshots.expected, inventory, fixture)
	if err != nil {
		return NodeConnectorPlacementInputSnapshot{}, err
	}
	if snapshots.placement != nil {
		if !nodeExecutionEqual(*snapshots.placement, placement) {
			return NodeConnectorPlacementInputSnapshot{}, errors.New("changed or conflicting node placement-input snapshot replay is rejected")
		}
		return cloneNodeConnectorPlacementInputSnapshot(*snapshots.placement), nil
	}
	path := filepath.Join(snapshots.root, nodeConnectorPlacementInputArtifactName)
	if err := requireNodeConnectorInventoryArtifactAbsent(path, "placement-input snapshot"); err != nil {
		return NodeConnectorPlacementInputSnapshot{}, err
	}
	if err := nodeConnectorPlacementInputWriteAtomic(path, placement); err != nil {
		return NodeConnectorPlacementInputSnapshot{}, errors.New("node placement-input snapshot could not be published after the durable inventory snapshot")
	}
	snapshots.placement = &placement
	return cloneNodeConnectorPlacementInputSnapshot(placement), nil
}

func (snapshots *NodeConnectorInventorySnapshots) Artifacts() (*NodeConnectorInventorySnapshot, *NodeConnectorPlacementInputSnapshot) {
	snapshots.mu.Lock()
	defer snapshots.mu.Unlock()
	var inventory *NodeConnectorInventorySnapshot
	var placement *NodeConnectorPlacementInputSnapshot
	if snapshots.inventory != nil {
		value := cloneNodeConnectorInventorySnapshot(*snapshots.inventory)
		inventory = &value
	}
	if snapshots.placement != nil {
		value := cloneNodeConnectorPlacementInputSnapshot(*snapshots.placement)
		placement = &value
	}
	return inventory, placement
}

func deriveNodeConnectorInventorySnapshot(expected NodeConnectorInventorySnapshotExpected, fixture NodeConnectorInventorySnapshotFixture) (NodeConnectorInventorySnapshot, error) {
	if fixture.Schema != NodeConnectorInventorySnapshotFixtureSchema || fixture.Provenance != nodeConnectorInventoryProvenance {
		return NodeConnectorInventorySnapshot{}, errors.New("node inventory snapshot fixture contract or provenance is invalid")
	}
	if err := validateNodeExecutionTypedID("inventory", fixture.InventorySnapshotID); err != nil {
		return NodeConnectorInventorySnapshot{}, errors.New("node inventory snapshot identity is invalid")
	}
	if err := validateNodeExecutionTypedID("replay", fixture.ReplayIdentity); err != nil || fixture.ReplayIdentity == fixture.InventorySnapshotID {
		return NodeConnectorInventorySnapshot{}, errors.New("node inventory snapshot replay identity is invalid or colliding")
	}
	if _, err := parseNodeExecutionTime(fixture.ObservedAt); err != nil {
		return NodeConnectorInventorySnapshot{}, err
	}
	if err := validateNodeConnectorInventoryNodes(fixture.Nodes); err != nil {
		return NodeConnectorInventorySnapshot{}, err
	}
	if !nodeExecutionEqual(nodeConnectorInventoryBindings(fixture.Nodes), expected.Nodes) {
		return NodeConnectorInventorySnapshot{}, errors.New("node inventory snapshot does not exactly bind the expected nodes, machines, capabilities, or profiles")
	}
	inventory := NodeConnectorInventorySnapshot{
		Schema: NodeConnectorInventorySnapshotSchema, InventorySnapshotID: fixture.InventorySnapshotID,
		ReplayIdentity: fixture.ReplayIdentity, ObservedAt: fixture.ObservedAt, NodeCount: len(fixture.Nodes),
		Nodes: cloneNodeConnectorInventoryNodes(fixture.Nodes), Provenance: nodeConnectorInventoryProvenance,
		FixtureOwned: true,
	}
	fingerprint, err := nodeConnectorInventorySnapshotFingerprint(inventory)
	if err != nil {
		return NodeConnectorInventorySnapshot{}, err
	}
	inventory.InventorySnapshotFingerprint = fingerprint
	if err := validateNodeConnectorInventorySnapshot(inventory, expected); err != nil {
		return NodeConnectorInventorySnapshot{}, err
	}
	return inventory, nil
}

func deriveNodeConnectorPlacementInputSnapshot(expected NodeConnectorInventorySnapshotExpected, inventory NodeConnectorInventorySnapshot, fixture NodeConnectorPlacementInputSnapshotFixture) (NodeConnectorPlacementInputSnapshot, error) {
	if fixture.Schema != NodeConnectorPlacementInputSnapshotFixtureSchema || fixture.Provenance != nodeConnectorPlacementInputProvenance ||
		fixture.InventorySnapshotID != inventory.InventorySnapshotID || fixture.InventorySnapshotFingerprint != inventory.InventorySnapshotFingerprint {
		return NodeConnectorPlacementInputSnapshot{}, errors.New("node placement-input snapshot does not exactly bind the durable inventory snapshot")
	}
	if err := validateNodeExecutionTypedID("placement-input", fixture.PlacementInputID); err != nil {
		return NodeConnectorPlacementInputSnapshot{}, errors.New("node placement-input snapshot identity is invalid")
	}
	if err := validateNodeExecutionTypedID("replay", fixture.ReplayIdentity); err != nil || fixture.ReplayIdentity == fixture.PlacementInputID || fixture.ReplayIdentity == inventory.ReplayIdentity {
		return NodeConnectorPlacementInputSnapshot{}, errors.New("node placement-input snapshot replay identity is invalid or colliding")
	}
	if fixture.PlacementInputID == inventory.InventorySnapshotID || fixture.WorkloadID != expected.PlacementInput.WorkloadID ||
		fixture.RequirementsFingerprint != expected.PlacementInput.RequirementsFingerprint || !nodeExecutionEqual(fixture.CandidateNodeIDs, expected.PlacementInput.CandidateNodeIDs) {
		return NodeConnectorPlacementInputSnapshot{}, errors.New("node placement-input snapshot workload, requirements, or expected candidate binding is invalid")
	}
	if err := validateNodeConnectorCandidateIDs(fixture.CandidateNodeIDs); err != nil || !nodeExecutionEqual(fixture.CandidateNodeIDs, nodeConnectorInventoryNodeIDs(inventory.Nodes)) {
		return NodeConnectorPlacementInputSnapshot{}, errors.New("node placement-input snapshot must contain the exact complete sorted durable inventory node set")
	}
	placement := NodeConnectorPlacementInputSnapshot{
		Schema: NodeConnectorPlacementInputSnapshotSchema, PlacementInputID: fixture.PlacementInputID,
		ReplayIdentity: fixture.ReplayIdentity, InventorySnapshotID: inventory.InventorySnapshotID,
		InventorySnapshotFingerprint: inventory.InventorySnapshotFingerprint, WorkloadID: fixture.WorkloadID,
		RequirementsFingerprint: fixture.RequirementsFingerprint, CandidateNodeCount: len(fixture.CandidateNodeIDs),
		CandidateNodeIDs: append([]string{}, fixture.CandidateNodeIDs...), CompleteInventoryCandidateSet: true,
		Provenance: nodeConnectorPlacementInputProvenance, FixtureOwned: true,
	}
	fingerprint, err := nodeConnectorPlacementInputSnapshotFingerprint(placement)
	if err != nil {
		return NodeConnectorPlacementInputSnapshot{}, err
	}
	placement.PlacementInputSnapshotFingerprint = fingerprint
	if err := validateNodeConnectorPlacementInputSnapshot(placement, expected, inventory); err != nil {
		return NodeConnectorPlacementInputSnapshot{}, err
	}
	return placement, nil
}

func normalizeNodeConnectorInventoryExpected(value NodeConnectorInventorySnapshotExpected) (NodeConnectorInventorySnapshotExpected, error) {
	value.Nodes = cloneNodeConnectorInventoryBindings(value.Nodes)
	value.PlacementInput.CandidateNodeIDs = append([]string{}, value.PlacementInput.CandidateNodeIDs...)
	if len(value.Nodes) < 1 || len(value.Nodes) > nodeConnectorInventoryMaxNodes {
		return NodeConnectorInventorySnapshotExpected{}, errors.New("node inventory expected node count is invalid or unbounded")
	}
	last := ""
	machines, capabilities := map[string]bool{}, map[string]bool{}
	totalReferenceBytes := int64(0)
	for _, node := range value.Nodes {
		if node.NodeID <= last || validateNodeConnectorInventoryBinding(node) != nil || machines[node.MachineID] || capabilities[node.CapabilitySnapshotID] {
			return NodeConnectorInventorySnapshotExpected{}, errors.New("node inventory expected bindings are invalid, duplicate, or unsorted")
		}
		referenceBytes, _ := validateNodeConnectorInventoryReferences(node.References)
		if totalReferenceBytes > nodeConnectorInventoryMaxReferenceAggregate-referenceBytes {
			return NodeConnectorInventorySnapshotExpected{}, errors.New("node inventory expected references exceed their aggregate byte bound")
		}
		totalReferenceBytes += referenceBytes
		last, machines[node.MachineID], capabilities[node.CapabilitySnapshotID] = node.NodeID, true, true
	}
	if err := validateNodeExecutionTypedID("workload", value.PlacementInput.WorkloadID); err != nil || !nodeExecutionFingerprint.MatchString(value.PlacementInput.RequirementsFingerprint) {
		return NodeConnectorInventorySnapshotExpected{}, errors.New("node placement-input expected workload or requirements binding is invalid")
	}
	if err := validateNodeConnectorCandidateIDs(value.PlacementInput.CandidateNodeIDs); err != nil || !nodeExecutionEqual(value.PlacementInput.CandidateNodeIDs, nodeConnectorExpectedNodeIDs(value.Nodes)) {
		return NodeConnectorInventorySnapshotExpected{}, errors.New("node placement-input expected candidates must exactly equal the sorted expected nodes")
	}
	return value, nil
}

func validateNodeConnectorInventoryBinding(value NodeConnectorInventoryNodeBinding) error {
	if validateNodeExecutionTypedID("node", value.NodeID) != nil || validateNodeExecutionTypedID("machine", value.MachineID) != nil {
		return errors.New("node inventory node or machine identity is invalid")
	}
	if !nodeExecutionFingerprint.MatchString(value.CapabilitySnapshotID) || !nodeExecutionFingerprint.MatchString(value.CapabilitySnapshotFingerprint) {
		return errors.New("node inventory capability identity or fingerprint is invalid")
	}
	if err := validateNodeConnectorInventoryProfile(value.Profile); err != nil {
		return err
	}
	_, err := validateNodeConnectorInventoryReferences(value.References)
	return err
}

func validateNodeConnectorInventoryProfile(value NodeConnectorInventoryTargetProfile) error {
	linuxHost := value == (NodeConnectorInventoryTargetProfile{HostOS: "linux", Runtime: "host", GuestOS: "none"})
	windowsHost := value == (NodeConnectorInventoryTargetProfile{HostOS: "windows", Runtime: "host", GuestOS: "none"})
	qemuWindows := value == (NodeConnectorInventoryTargetProfile{HostOS: "linux", Runtime: "qemu", GuestOS: "windows"})
	if !linuxHost && !windowsHost && !qemuWindows {
		return errors.New("node inventory target profile is unsupported or ambiguous")
	}
	return nil
}

func validateNodeConnectorInventoryNodes(values []NodeConnectorInventoryNodeEvidence) error {
	if len(values) < 1 || len(values) > nodeConnectorInventoryMaxNodes || !sort.SliceIsSorted(values, func(i, j int) bool { return values[i].NodeID < values[j].NodeID }) {
		return errors.New("node inventory nodes exceed their count bound or are not ordinally sorted")
	}
	last := ""
	machines, capabilities := map[string]bool{}, map[string]bool{}
	totalReferenceBytes := int64(0)
	for _, value := range values {
		binding := nodeConnectorInventoryBinding(value)
		if value.NodeID <= last || validateNodeConnectorInventoryBinding(binding) != nil || machines[value.MachineID] || capabilities[value.CapabilitySnapshotID] {
			return errors.New("node inventory node bindings are invalid, duplicate, or unsorted")
		}
		if !isNodeConnectorInventoryAvailability(value.Availability.Classification) {
			return errors.New("node inventory availability classification is invalid")
		}
		if value.Load.CPUUtilizationBasisPoints < 0 || value.Load.CPUUtilizationBasisPoints > nodeConnectorInventoryMaxUtilizationBasisPoint ||
			value.Load.MemoryUtilizationBasisPoints < 0 || value.Load.MemoryUtilizationBasisPoints > nodeConnectorInventoryMaxUtilizationBasisPoint ||
			value.Load.ActiveTaskCount < 0 || value.Load.ActiveTaskCount > nodeConnectorInventoryMaxActiveTasks {
			return errors.New("node inventory load evidence is negative, overflowing, or out of range")
		}
		if !isNodeConnectorInventoryRisk(value.Risk.Classification) {
			return errors.New("node inventory risk classification is invalid")
		}
		if err := validateNodeConnectorInventoryCost(value.Cost); err != nil {
			return err
		}
		bytes, err := validateNodeConnectorInventoryReferences(value.References)
		if err != nil || totalReferenceBytes > nodeConnectorInventoryMaxReferenceAggregate-bytes {
			return errors.New("node inventory references are invalid or exceed their aggregate byte bound")
		}
		totalReferenceBytes += bytes
		last, machines[value.MachineID], capabilities[value.CapabilitySnapshotID] = value.NodeID, true, true
	}
	return nil
}

func validateNodeConnectorInventoryCost(value NodeConnectorInventoryCostEvidence) error {
	if value.Classification != "local_unmetered" && value.Classification != "estimated_metered" && value.Classification != "unknown" {
		return errors.New("node inventory cost classification is invalid")
	}
	if value.NormalizedValue == nil {
		if value.LogicalUnit != "" {
			return errors.New("node inventory cost logical unit requires a normalized value")
		}
		return nil
	}
	if *value.NormalizedValue < 0 || *value.NormalizedValue > nodeConnectorInventoryMaxNormalizedCost || value.LogicalUnit != nodeConnectorInventoryCostUnit {
		return errors.New("node inventory normalized cost value or logical unit is invalid")
	}
	if value.Classification == "local_unmetered" && *value.NormalizedValue != 0 {
		return errors.New("node inventory local-unmetered cost must have a zero normalized value")
	}
	return nil
}

func validateNodeConnectorInventoryReferences(values []NodeConnectorInventoryEvidenceReference) (int64, error) {
	if len(values) > nodeConnectorInventoryMaxReferencesPerNode || !sort.SliceIsSorted(values, func(i, j int) bool { return values[i].Name < values[j].Name }) {
		return 0, errors.New("node inventory references exceed their count bound or are not ordinally sorted")
	}
	last := ""
	total := int64(0)
	for _, value := range values {
		if value.Name <= last || validateNodeExecutionName("inventory evidence reference name", value.Name) != nil || strings.ContainsAny(value.Name, `/\\`) || strings.Contains(value.Name, "..") {
			return 0, errors.New("node inventory reference name is invalid, duplicate, or unsorted")
		}
		if value.MediaType == "" || len(value.MediaType) > 128 || !strings.Contains(value.MediaType, "/") || strings.Contains(value.MediaType, "://") || strings.ContainsAny(value.MediaType, "\r\n\t") {
			return 0, errors.New("node inventory reference media type is invalid")
		}
		if !nodeExecutionFingerprint.MatchString(value.Digest) || value.Bytes < 0 || value.Bytes > nodeConnectorInventoryMaxReferenceBytes || total > nodeConnectorInventoryMaxReferenceAggregate-value.Bytes {
			return 0, errors.New("node inventory reference digest or byte bounds are invalid")
		}
		total += value.Bytes
		last = value.Name
	}
	return total, nil
}

func isNodeConnectorInventoryAvailability(value string) bool {
	return value == "available" || value == "unavailable" || value == "unknown"
}

func isNodeConnectorInventoryRisk(value string) bool {
	return value == "low" || value == "medium" || value == "high" || value == "unknown"
}

func validateNodeConnectorInventorySnapshot(value NodeConnectorInventorySnapshot, expected NodeConnectorInventorySnapshotExpected) error {
	if value.Schema != NodeConnectorInventorySnapshotSchema || value.Provenance != nodeConnectorInventoryProvenance || !value.FixtureOwned || value.EvidenceAuthoritative || value.Authority != (NodeConnectorInventoryAuthority{}) || value.NodeCount != len(value.Nodes) {
		return errors.New("node inventory snapshot contract, provenance, evidence claim, or authority is invalid")
	}
	if validateNodeExecutionTypedID("inventory", value.InventorySnapshotID) != nil || validateNodeExecutionTypedID("replay", value.ReplayIdentity) != nil || value.ReplayIdentity == value.InventorySnapshotID {
		return errors.New("node inventory snapshot or replay identity is invalid")
	}
	if _, err := parseNodeExecutionTime(value.ObservedAt); err != nil {
		return err
	}
	if err := validateNodeConnectorInventoryNodes(value.Nodes); err != nil || !nodeExecutionEqual(nodeConnectorInventoryBindings(value.Nodes), expected.Nodes) {
		return errors.New("node inventory durable node evidence or expected binding is invalid")
	}
	fingerprint, err := nodeConnectorInventorySnapshotFingerprint(value)
	if err != nil || fingerprint != value.InventorySnapshotFingerprint {
		return errors.New("node inventory snapshot fingerprint is invalid")
	}
	return validateNodeConnectorInventoryEncodedBound(value)
}

func validateNodeConnectorPlacementInputSnapshot(value NodeConnectorPlacementInputSnapshot, expected NodeConnectorInventorySnapshotExpected, inventory NodeConnectorInventorySnapshot) error {
	if value.Schema != NodeConnectorPlacementInputSnapshotSchema || value.Provenance != nodeConnectorPlacementInputProvenance || !value.FixtureOwned || value.EvidenceAuthoritative ||
		value.Authority != (NodeConnectorInventoryAuthority{}) || !value.CompleteInventoryCandidateSet || value.CandidateNodeCount != len(value.CandidateNodeIDs) ||
		value.InventorySnapshotID != inventory.InventorySnapshotID || value.InventorySnapshotFingerprint != inventory.InventorySnapshotFingerprint {
		return errors.New("node placement-input snapshot contract, inventory binding, or authority is invalid")
	}
	if validateNodeExecutionTypedID("placement-input", value.PlacementInputID) != nil || validateNodeExecutionTypedID("replay", value.ReplayIdentity) != nil ||
		value.ReplayIdentity == value.PlacementInputID || value.ReplayIdentity == inventory.ReplayIdentity || value.PlacementInputID == inventory.InventorySnapshotID {
		return errors.New("node placement-input snapshot identity or replay identity is invalid")
	}
	if value.WorkloadID != expected.PlacementInput.WorkloadID || value.RequirementsFingerprint != expected.PlacementInput.RequirementsFingerprint ||
		!nodeExecutionEqual(value.CandidateNodeIDs, expected.PlacementInput.CandidateNodeIDs) || !nodeExecutionEqual(value.CandidateNodeIDs, nodeConnectorInventoryNodeIDs(inventory.Nodes)) {
		return errors.New("node placement-input workload, requirements, or complete candidate binding is invalid")
	}
	if err := validateNodeConnectorCandidateIDs(value.CandidateNodeIDs); err != nil {
		return err
	}
	fingerprint, err := nodeConnectorPlacementInputSnapshotFingerprint(value)
	if err != nil || fingerprint != value.PlacementInputSnapshotFingerprint {
		return errors.New("node placement-input snapshot fingerprint is invalid")
	}
	return validateNodeConnectorInventoryEncodedBound(value)
}

func validateNodeConnectorCandidateIDs(values []string) error {
	if len(values) < 1 || len(values) > nodeConnectorInventoryMaxNodes {
		return errors.New("node placement-input candidate count is invalid or unbounded")
	}
	last := ""
	for _, value := range values {
		if value <= last || validateNodeExecutionTypedID("node", value) != nil {
			return errors.New("node placement-input candidates are invalid, duplicate, or unsorted")
		}
		last = value
	}
	return nil
}

func loadNodeConnectorInventorySnapshot(root string, expected NodeConnectorInventorySnapshotExpected) (NodeConnectorInventorySnapshot, bool, error) {
	path := filepath.Join(root, nodeConnectorInventoryArtifactName)
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return NodeConnectorInventorySnapshot{}, false, nil
	}
	if err != nil || len(raw) == 0 || len(raw) > nodeConnectorInventoryMaxArtifactBytes {
		return NodeConnectorInventorySnapshot{}, false, errors.New("durable node inventory snapshot is missing, malformed, or oversized")
	}
	var value NodeConnectorInventorySnapshot
	if decodeNodeConnectorInventoryArtifact(raw, &value) != nil || validateNodeConnectorInventorySnapshot(value, expected) != nil {
		return NodeConnectorInventorySnapshot{}, false, errors.New("durable node inventory snapshot is malformed, noncanonical, or tampered")
	}
	return value, true, nil
}

func loadNodeConnectorPlacementInputSnapshot(root string, expected NodeConnectorInventorySnapshotExpected, inventory NodeConnectorInventorySnapshot, inventoryExists bool) (NodeConnectorPlacementInputSnapshot, bool, error) {
	path := filepath.Join(root, nodeConnectorPlacementInputArtifactName)
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return NodeConnectorPlacementInputSnapshot{}, false, nil
	}
	if err != nil || len(raw) == 0 || len(raw) > nodeConnectorInventoryMaxArtifactBytes || !inventoryExists {
		return NodeConnectorPlacementInputSnapshot{}, false, errors.New("durable node placement-input snapshot is missing its inventory, malformed, or oversized")
	}
	var value NodeConnectorPlacementInputSnapshot
	if decodeNodeConnectorInventoryArtifact(raw, &value) != nil || validateNodeConnectorPlacementInputSnapshot(value, expected, inventory) != nil {
		return NodeConnectorPlacementInputSnapshot{}, false, errors.New("durable node placement-input snapshot is malformed, noncanonical, or tampered")
	}
	return value, true, nil
}

func nodeConnectorInventorySnapshotFingerprint(value NodeConnectorInventorySnapshot) (string, error) {
	value.InventorySnapshotFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func nodeConnectorPlacementInputSnapshotFingerprint(value NodeConnectorPlacementInputSnapshot) (string, error) {
	value.PlacementInputSnapshotFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func validateNodeConnectorInventoryEncodedBound(value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil || len(raw)+1 > nodeConnectorInventoryMaxArtifactBytes {
		return errors.New("node inventory or placement-input durable artifact exceeds its encoded bound")
	}
	return nil
}

func decodeNodeConnectorInventoryArtifact(raw []byte, target any) error {
	if err := decodeNodeExecutionStrict(raw, target); err != nil {
		return err
	}
	canonical, err := json.MarshalIndent(target, "", "  ")
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return errors.New("node inventory or placement-input durable artifact is not canonical")
	}
	return nil
}

func requireNodeConnectorInventoryArtifactAbsent(path, kind string) error {
	if _, err := os.Lstat(path); err == nil {
		return errors.New("unexpected node " + kind + " artifact already exists")
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}

func nodeConnectorInventoryBindings(values []NodeConnectorInventoryNodeEvidence) []NodeConnectorInventoryNodeBinding {
	bindings := make([]NodeConnectorInventoryNodeBinding, 0, len(values))
	for _, value := range values {
		bindings = append(bindings, nodeConnectorInventoryBinding(value))
	}
	return bindings
}

func nodeConnectorInventoryBinding(value NodeConnectorInventoryNodeEvidence) NodeConnectorInventoryNodeBinding {
	return NodeConnectorInventoryNodeBinding{
		NodeID: value.NodeID, MachineID: value.MachineID,
		CapabilitySnapshotID: value.CapabilitySnapshotID, CapabilitySnapshotFingerprint: value.CapabilitySnapshotFingerprint,
		Profile: value.Profile, References: append([]NodeConnectorInventoryEvidenceReference{}, value.References...),
	}
}

func cloneNodeConnectorInventoryBindings(values []NodeConnectorInventoryNodeBinding) []NodeConnectorInventoryNodeBinding {
	bindings := make([]NodeConnectorInventoryNodeBinding, len(values))
	for index, value := range values {
		value.References = append([]NodeConnectorInventoryEvidenceReference{}, value.References...)
		bindings[index] = value
	}
	return bindings
}

func nodeConnectorInventoryNodeIDs(values []NodeConnectorInventoryNodeEvidence) []string {
	ids := make([]string, 0, len(values))
	for _, value := range values {
		ids = append(ids, value.NodeID)
	}
	return ids
}

func nodeConnectorExpectedNodeIDs(values []NodeConnectorInventoryNodeBinding) []string {
	ids := make([]string, 0, len(values))
	for _, value := range values {
		ids = append(ids, value.NodeID)
	}
	return ids
}

func cloneNodeConnectorInventoryNodes(values []NodeConnectorInventoryNodeEvidence) []NodeConnectorInventoryNodeEvidence {
	raw, _ := json.Marshal(values)
	var cloned []NodeConnectorInventoryNodeEvidence
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func cloneNodeConnectorInventorySnapshot(value NodeConnectorInventorySnapshot) NodeConnectorInventorySnapshot {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorInventorySnapshot
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func cloneNodeConnectorPlacementInputSnapshot(value NodeConnectorPlacementInputSnapshot) NodeConnectorPlacementInputSnapshot {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementInputSnapshot
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func EncodeNodeConnectorInventorySnapshotFixture(value NodeConnectorInventorySnapshotFixture) ([]byte, error) {
	return json.Marshal(value)
}

func EncodeNodeConnectorPlacementInputSnapshotFixture(value NodeConnectorPlacementInputSnapshotFixture) ([]byte, error) {
	return json.Marshal(value)
}
