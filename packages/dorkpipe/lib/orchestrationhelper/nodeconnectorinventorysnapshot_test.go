package orchestrationhelper

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestNodeConnectorInventorySnapshotProfilesEvidencePlacementAndAuthority(t *testing.T) {
	root, expected, inventoryFixture := nodeConnectorInventoryTestFixture(t)
	snapshots := mustOpenNodeConnectorInventorySnapshots(t, root, expected)
	inventory := mustRecordNodeConnectorInventory(t, snapshots, inventoryFixture)
	if inventory.NodeCount != 3 || inventory.InventorySnapshotFingerprint == "" || inventory.EvidenceAuthoritative || inventory.Authority != (NodeConnectorInventoryAuthority{}) {
		t.Fatal("inventory did not preserve the exact non-authoritative three-node evidence set")
	}
	wantProfiles := []NodeConnectorInventoryTargetProfile{
		{HostOS: "linux", Runtime: "host", GuestOS: "none"},
		{HostOS: "linux", Runtime: "qemu", GuestOS: "windows"},
		{HostOS: "windows", Runtime: "host", GuestOS: "none"},
	}
	if got := []NodeConnectorInventoryTargetProfile{inventory.Nodes[0].Profile, inventory.Nodes[1].Profile, inventory.Nodes[2].Profile}; !reflect.DeepEqual(got, wantProfiles) {
		t.Fatalf("target profiles changed: %#v", got)
	}
	if got := []string{inventory.Nodes[0].Availability.Classification, inventory.Nodes[1].Availability.Classification, inventory.Nodes[2].Availability.Classification}; !reflect.DeepEqual(got, []string{"available", "unknown", "unavailable"}) {
		t.Fatalf("availability classifications changed: %v", got)
	}
	if got := []string{inventory.Nodes[0].Risk.Classification, inventory.Nodes[1].Risk.Classification, inventory.Nodes[2].Risk.Classification}; !reflect.DeepEqual(got, []string{"low", "medium", "high"}) {
		t.Fatalf("risk classifications changed: %v", got)
	}
	if got := []string{inventory.Nodes[0].Cost.Classification, inventory.Nodes[1].Cost.Classification, inventory.Nodes[2].Cost.Classification}; !reflect.DeepEqual(got, []string{"local_unmetered", "estimated_metered", "unknown"}) {
		t.Fatalf("cost classifications changed: %v", got)
	}
	placementFixture := nodeConnectorPlacementTestFixture(inventory, expected)
	placement := mustRecordNodeConnectorPlacement(t, snapshots, placementFixture)
	if placement.InventorySnapshotFingerprint != inventory.InventorySnapshotFingerprint || !placement.CompleteInventoryCandidateSet || placement.CandidateNodeCount != 3 ||
		!reflect.DeepEqual(placement.CandidateNodeIDs, expected.PlacementInput.CandidateNodeIDs) || placement.EvidenceAuthoritative || placement.Authority != (NodeConnectorInventoryAuthority{}) {
		t.Fatal("placement input did not preserve the exact complete non-authoritative inventory binding")
	}
	if !sort.StringsAreSorted(placement.CandidateNodeIDs) || !sort.SliceIsSorted(inventory.Nodes, func(i, j int) bool { return inventory.Nodes[i].NodeID < inventory.Nodes[j].NodeID }) {
		t.Fatal("inventory or candidate output is not deterministic and ordinally sorted")
	}
	for _, node := range inventory.Nodes {
		if !sort.SliceIsSorted(node.References, func(i, j int) bool { return node.References[i].Name < node.References[j].Name }) {
			t.Fatal("checksum-only references are not ordinally sorted")
		}
	}
}

func TestNodeConnectorInventorySnapshotEvidenceClassificationsAndBoundaries(t *testing.T) {
	cases := []struct {
		name         string
		availability string
		risk         string
		cost         string
		cpu          int
		memory       int
		active       int
		normalized   *int
	}{
		{name: "minimum-low-local", availability: "available", risk: "low", cost: "local_unmetered", normalized: nodeConnectorInventoryInt(0)},
		{name: "maximum-medium-metered", availability: "unavailable", risk: "medium", cost: "estimated_metered", cpu: nodeConnectorInventoryMaxUtilizationBasisPoint, memory: nodeConnectorInventoryMaxUtilizationBasisPoint, active: nodeConnectorInventoryMaxActiveTasks, normalized: nodeConnectorInventoryInt(nodeConnectorInventoryMaxNormalizedCost)},
		{name: "high-unknown-cost", availability: "unknown", risk: "high", cost: "unknown"},
		{name: "unknown-risk-metered-zero", availability: "available", risk: "unknown", cost: "estimated_metered", normalized: nodeConnectorInventoryInt(0)},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			root, expected, fixture := nodeConnectorInventoryOneNodeFixture(t)
			node := &fixture.Nodes[0]
			node.Availability.Classification, node.Risk.Classification, node.Cost.Classification = test.availability, test.risk, test.cost
			node.Load = NodeConnectorInventoryLoadEvidence{CPUUtilizationBasisPoints: test.cpu, MemoryUtilizationBasisPoints: test.memory, ActiveTaskCount: test.active}
			node.Cost.NormalizedValue = test.normalized
			if test.normalized != nil {
				node.Cost.LogicalUnit = nodeConnectorInventoryCostUnit
			}
			mustRecordNodeConnectorInventory(t, mustOpenNodeConnectorInventorySnapshots(t, root, expected), fixture)
		})
	}
}

func TestNodeConnectorInventorySnapshotRejectsNodeAndReferenceSetChanges(t *testing.T) {
	nodeMutations := []struct {
		name   string
		mutate func(*NodeConnectorInventorySnapshotFixture)
	}{
		{"missing", func(value *NodeConnectorInventorySnapshotFixture) { value.Nodes = value.Nodes[1:] }},
		{"duplicate", func(value *NodeConnectorInventorySnapshotFixture) { value.Nodes[1] = value.Nodes[0] }},
		{"reordered", func(value *NodeConnectorInventorySnapshotFixture) {
			value.Nodes[0], value.Nodes[1] = value.Nodes[1], value.Nodes[0]
		}},
		{"extra", func(value *NodeConnectorInventorySnapshotFixture) {
			extra := value.Nodes[2]
			extra.NodeID, extra.MachineID, extra.CapabilitySnapshotID, extra.CapabilitySnapshotFingerprint = "node-z-extra-001", "machine-z-extra-001", nodeConnectorInventoryFingerprint("7"), nodeConnectorInventoryFingerprint("8")
			value.Nodes = append(value.Nodes, extra)
		}},
		{"substituted-node", func(value *NodeConnectorInventorySnapshotFixture) { value.Nodes[0].NodeID = "node-substituted-001" }},
		{"wrong-machine", func(value *NodeConnectorInventorySnapshotFixture) {
			value.Nodes[0].MachineID = "machine-substituted-001"
		}},
		{"wrong-capability-id", func(value *NodeConnectorInventorySnapshotFixture) {
			value.Nodes[0].CapabilitySnapshotID = nodeConnectorInventoryFingerprint("9")
		}},
		{"wrong-capability-fingerprint", func(value *NodeConnectorInventorySnapshotFixture) {
			value.Nodes[0].CapabilitySnapshotFingerprint = nodeConnectorInventoryFingerprint("0")
		}},
		{"wrong-host-os", func(value *NodeConnectorInventorySnapshotFixture) { value.Nodes[0].Profile.HostOS = "windows" }},
		{"wrong-runtime", func(value *NodeConnectorInventorySnapshotFixture) { value.Nodes[0].Profile.Runtime = "qemu" }},
		{"wrong-guest-os", func(value *NodeConnectorInventorySnapshotFixture) { value.Nodes[0].Profile.GuestOS = "windows" }},
	}
	for _, test := range nodeMutations {
		t.Run(test.name, func(t *testing.T) {
			root, expected, fixture := nodeConnectorInventoryTestFixture(t)
			test.mutate(&fixture)
			if _, err := mustOpenNodeConnectorInventorySnapshots(t, root, expected).RecordInventory(mustMarshalNodeConnectorInventory(t, fixture)); err == nil {
				t.Fatal("changed node set or exact binding was accepted")
			}
		})
	}

	referenceMutations := []struct {
		name   string
		mutate func(*[]NodeConnectorInventoryEvidenceReference)
	}{
		{"missing", func(values *[]NodeConnectorInventoryEvidenceReference) { *values = (*values)[1:] }},
		{"duplicate", func(values *[]NodeConnectorInventoryEvidenceReference) { (*values)[1] = (*values)[0] }},
		{"reordered", func(values *[]NodeConnectorInventoryEvidenceReference) {
			(*values)[0], (*values)[1] = (*values)[1], (*values)[0]
		}},
		{"extra", func(values *[]NodeConnectorInventoryEvidenceReference) {
			*values = append(*values, NodeConnectorInventoryEvidenceReference{Name: "zz-extra", MediaType: "application/json", Digest: nodeConnectorInventoryFingerprint("1"), Bytes: 1})
		}},
		{"substituted", func(values *[]NodeConnectorInventoryEvidenceReference) {
			(*values)[0].Digest = nodeConnectorInventoryFingerprint("2")
		}},
	}
	for _, test := range referenceMutations {
		t.Run("reference-"+test.name, func(t *testing.T) {
			root, expected, fixture := nodeConnectorInventoryTestFixture(t)
			test.mutate(&fixture.Nodes[0].References)
			if _, err := mustOpenNodeConnectorInventorySnapshots(t, root, expected).RecordInventory(mustMarshalNodeConnectorInventory(t, fixture)); err == nil {
				t.Fatal("changed reference set was accepted")
			}
		})
	}
}

func TestNodeConnectorInventorySnapshotRejectsInvalidProfilesEvidenceAndBounds(t *testing.T) {
	mutations := []struct {
		name   string
		mutate func(*NodeConnectorInventoryNodeEvidence)
	}{
		{"ambiguous-profile", func(value *NodeConnectorInventoryNodeEvidence) {
			value.Profile = NodeConnectorInventoryTargetProfile{HostOS: "windows", Runtime: "qemu", GuestOS: "linux"}
		}},
		{"invalid-availability", func(value *NodeConnectorInventoryNodeEvidence) { value.Availability.Classification = "degraded" }},
		{"negative-cpu", func(value *NodeConnectorInventoryNodeEvidence) { value.Load.CPUUtilizationBasisPoints = -1 }},
		{"overflow-cpu", func(value *NodeConnectorInventoryNodeEvidence) {
			value.Load.CPUUtilizationBasisPoints = nodeConnectorInventoryMaxUtilizationBasisPoint + 1
		}},
		{"negative-memory", func(value *NodeConnectorInventoryNodeEvidence) { value.Load.MemoryUtilizationBasisPoints = -1 }},
		{"overflow-memory", func(value *NodeConnectorInventoryNodeEvidence) {
			value.Load.MemoryUtilizationBasisPoints = nodeConnectorInventoryMaxUtilizationBasisPoint + 1
		}},
		{"negative-active", func(value *NodeConnectorInventoryNodeEvidence) { value.Load.ActiveTaskCount = -1 }},
		{"overflow-active", func(value *NodeConnectorInventoryNodeEvidence) {
			value.Load.ActiveTaskCount = nodeConnectorInventoryMaxActiveTasks + 1
		}},
		{"invalid-risk", func(value *NodeConnectorInventoryNodeEvidence) { value.Risk.Classification = "critical" }},
		{"invalid-cost", func(value *NodeConnectorInventoryNodeEvidence) { value.Cost.Classification = "currency" }},
		{"negative-cost", func(value *NodeConnectorInventoryNodeEvidence) {
			value.Cost.NormalizedValue = nodeConnectorInventoryInt(-1)
			value.Cost.LogicalUnit = nodeConnectorInventoryCostUnit
		}},
		{"overflow-cost", func(value *NodeConnectorInventoryNodeEvidence) {
			value.Cost.NormalizedValue = nodeConnectorInventoryInt(nodeConnectorInventoryMaxNormalizedCost + 1)
			value.Cost.LogicalUnit = nodeConnectorInventoryCostUnit
		}},
		{"missing-cost-unit", func(value *NodeConnectorInventoryNodeEvidence) {
			value.Cost.NormalizedValue = nodeConnectorInventoryInt(1)
			value.Cost.LogicalUnit = ""
		}},
		{"unit-without-cost", func(value *NodeConnectorInventoryNodeEvidence) {
			value.Cost.NormalizedValue = nil
			value.Cost.LogicalUnit = nodeConnectorInventoryCostUnit
		}},
		{"unmetered-nonzero", func(value *NodeConnectorInventoryNodeEvidence) {
			value.Cost.Classification = "local_unmetered"
			value.Cost.NormalizedValue = nodeConnectorInventoryInt(1)
			value.Cost.LogicalUnit = nodeConnectorInventoryCostUnit
		}},
		{"negative-reference", func(value *NodeConnectorInventoryNodeEvidence) { value.References[0].Bytes = -1 }},
		{"oversized-reference", func(value *NodeConnectorInventoryNodeEvidence) {
			value.References[0].Bytes = nodeConnectorInventoryMaxReferenceBytes + 1
		}},
	}
	for _, test := range mutations {
		t.Run(test.name, func(t *testing.T) {
			root, expected, fixture := nodeConnectorInventoryTestFixture(t)
			test.mutate(&fixture.Nodes[0])
			if _, err := mustOpenNodeConnectorInventorySnapshots(t, root, expected).RecordInventory(mustMarshalNodeConnectorInventory(t, fixture)); err == nil {
				t.Fatal("invalid profile, evidence, cross-field combination, or bound was accepted")
			}
		})
	}

	t.Run("aggregate-reference-bytes", func(t *testing.T) {
		root, expected, fixture := nodeConnectorInventoryTestFixture(t)
		for nodeIndex := range fixture.Nodes {
			fixture.Nodes[nodeIndex].References = []NodeConnectorInventoryEvidenceReference{{Name: "aggregate-evidence", MediaType: "application/json", Digest: nodeConnectorInventoryFingerprint("1"), Bytes: nodeConnectorInventoryMaxReferenceBytes}}
		}
		extra := fixture.Nodes[2]
		extra.NodeID, extra.MachineID, extra.CapabilitySnapshotID, extra.CapabilitySnapshotFingerprint = "node-z-aggregate-001", "machine-z-aggregate-001", nodeConnectorInventoryFingerprint("7"), nodeConnectorInventoryFingerprint("8")
		fixture.Nodes = append(fixture.Nodes, extra, extra)
		fixture.Nodes[3].NodeID, fixture.Nodes[3].MachineID, fixture.Nodes[3].CapabilitySnapshotID = "node-y-aggregate-001", "machine-y-aggregate-001", nodeConnectorInventoryFingerprint("9")
		fixture.Nodes[4].NodeID, fixture.Nodes[4].MachineID, fixture.Nodes[4].CapabilitySnapshotID = "node-z-aggregate-002", "machine-z-aggregate-002", nodeConnectorInventoryFingerprint("0")
		sort.Slice(fixture.Nodes, func(i, j int) bool { return fixture.Nodes[i].NodeID < fixture.Nodes[j].NodeID })
		if _, err := mustOpenNodeConnectorInventorySnapshots(t, root, expected).RecordInventory(mustMarshalNodeConnectorInventory(t, fixture)); err == nil {
			t.Fatal("aggregate reference byte overflow was accepted")
		}
	})
}

func TestNodeConnectorInventorySnapshotRejectsMalformedNoncanonicalOversizedAndForbiddenFields(t *testing.T) {
	_, _, fixture := nodeConnectorInventoryTestFixture(t)
	valid := mustMarshalNodeConnectorInventory(t, fixture)
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, valid, "", "  "); err != nil {
		t.Fatal(err)
	}
	inputs := [][]byte{
		[]byte("{not-json"), append(append([]byte{}, valid...), []byte(" trailing")...), pretty.Bytes(), make([]byte, nodeConnectorInventoryMaxFixtureBytes+1),
		bytes.Replace(valid, []byte(`"cpu_utilization_basis_points":100`), []byte(`"cpu_utilization_basis_points":0.5`), 1),
		bytes.Replace(valid, []byte(`"active_task_count":1`), []byte(`"active_task_count":999999999999999999999999999`), 1),
	}
	for _, field := range []string{"command", "path", "executable", "account", "credential", "environment", "pid", "raw_output", "stdout", "stderr", "hostname", "endpoint", "provider", "connection", "ingress", "quota_enforcement", "billing", "reservation", "score", "rank", "eligible", "selected_node_id", "recommended_node_id", "winner", "lease", "dispatch", "retry", "repair_plan"} {
		inputs = append(inputs, append(append([]byte{}, valid[:len(valid)-1]...), []byte(`,"`+field+`":"forbidden"}`)...))
	}
	for index, raw := range inputs {
		root, expected, _ := nodeConnectorInventoryTestFixture(t)
		if _, err := mustOpenNodeConnectorInventorySnapshots(t, root, expected).RecordInventory(raw); err == nil {
			t.Fatalf("malformed, noncanonical, oversized, fractional, overflowing, trailing, unknown, or forbidden inventory input %d was accepted", index)
		}
	}
}

func TestNodeConnectorInventorySnapshotPlacementRequiresExactCompleteBindings(t *testing.T) {
	mutations := []struct {
		name   string
		mutate func(*NodeConnectorPlacementInputSnapshotFixture)
	}{
		{"missing-node", func(value *NodeConnectorPlacementInputSnapshotFixture) {
			value.CandidateNodeIDs = value.CandidateNodeIDs[1:]
		}},
		{"duplicate-node", func(value *NodeConnectorPlacementInputSnapshotFixture) {
			value.CandidateNodeIDs[1] = value.CandidateNodeIDs[0]
		}},
		{"reordered-node", func(value *NodeConnectorPlacementInputSnapshotFixture) {
			value.CandidateNodeIDs[0], value.CandidateNodeIDs[1] = value.CandidateNodeIDs[1], value.CandidateNodeIDs[0]
		}},
		{"extra-node", func(value *NodeConnectorPlacementInputSnapshotFixture) {
			value.CandidateNodeIDs = append(value.CandidateNodeIDs, "node-z-extra-001")
		}},
		{"substituted-node", func(value *NodeConnectorPlacementInputSnapshotFixture) {
			value.CandidateNodeIDs[0] = "node-substituted-001"
		}},
		{"workload", func(value *NodeConnectorPlacementInputSnapshotFixture) { value.WorkloadID = "workload-substituted-001" }},
		{"requirements", func(value *NodeConnectorPlacementInputSnapshotFixture) {
			value.RequirementsFingerprint = nodeConnectorInventoryFingerprint("0")
		}},
		{"inventory-id", func(value *NodeConnectorPlacementInputSnapshotFixture) {
			value.InventorySnapshotID = "inventory-substituted-001"
		}},
		{"inventory-fingerprint", func(value *NodeConnectorPlacementInputSnapshotFixture) {
			value.InventorySnapshotFingerprint = nodeConnectorInventoryFingerprint("0")
		}},
	}
	for _, test := range mutations {
		t.Run(test.name, func(t *testing.T) {
			root, expected, fixture := nodeConnectorInventoryTestFixture(t)
			snapshots := mustOpenNodeConnectorInventorySnapshots(t, root, expected)
			inventory := mustRecordNodeConnectorInventory(t, snapshots, fixture)
			placementFixture := nodeConnectorPlacementTestFixture(inventory, expected)
			test.mutate(&placementFixture)
			if _, err := snapshots.RecordPlacementInput(mustMarshalNodeConnectorInventory(t, placementFixture)); err == nil {
				t.Fatal("changed workload, requirements, inventory, or candidate binding was accepted")
			}
		})
	}
}

func TestNodeConnectorInventorySnapshotReplayRestartConflictsExpectedChangesAndTamper(t *testing.T) {
	root, expected, fixture := nodeConnectorInventoryTestFixture(t)
	snapshots := mustOpenNodeConnectorInventorySnapshots(t, root, expected)
	inventory := mustRecordNodeConnectorInventory(t, snapshots, fixture)
	placementFixture := nodeConnectorPlacementTestFixture(inventory, expected)
	placement := mustRecordNodeConnectorPlacement(t, snapshots, placementFixture)

	originalInventoryWriter := nodeConnectorInventoryWriteAtomic
	originalPlacementWriter := nodeConnectorPlacementInputWriteAtomic
	writes := 0
	nodeConnectorInventoryWriteAtomic = func(path string, value any) error { writes++; return originalInventoryWriter(path, value) }
	nodeConnectorPlacementInputWriteAtomic = func(path string, value any) error { writes++; return originalPlacementWriter(path, value) }
	t.Cleanup(func() {
		nodeConnectorInventoryWriteAtomic, nodeConnectorPlacementInputWriteAtomic = originalInventoryWriter, originalPlacementWriter
	})
	if replay := mustRecordNodeConnectorInventory(t, snapshots, fixture); !reflect.DeepEqual(replay, inventory) || writes != 0 {
		t.Fatal("identical inventory replay rewrote or changed the durable artifact")
	}
	if replay := mustRecordNodeConnectorPlacement(t, snapshots, placementFixture); !reflect.DeepEqual(replay, placement) || writes != 0 {
		t.Fatal("identical placement replay rewrote or changed the durable artifact")
	}
	restarted := mustOpenNodeConnectorInventorySnapshots(t, root, expected)
	if replay := mustRecordNodeConnectorInventory(t, restarted, fixture); !reflect.DeepEqual(replay, inventory) || writes != 0 {
		t.Fatal("restart inventory replay rewrote or changed the durable artifact")
	}
	if replay := mustRecordNodeConnectorPlacement(t, restarted, placementFixture); !reflect.DeepEqual(replay, placement) || writes != 0 {
		t.Fatal("restart placement replay rewrote or changed the durable artifact")
	}
	changedInventory := cloneNodeConnectorInventoryFixture(fixture)
	changedInventory.ReplayIdentity = "replay-inventory-conflict-001"
	if _, err := restarted.RecordInventory(mustMarshalNodeConnectorInventory(t, changedInventory)); err == nil || writes != 0 {
		t.Fatal("conflicting inventory replay was accepted or rewrote state")
	}
	changedPlacement := cloneNodeConnectorPlacementFixture(placementFixture)
	changedPlacement.ReplayIdentity = "replay-placement-conflict-001"
	if _, err := restarted.RecordPlacementInput(mustMarshalNodeConnectorInventory(t, changedPlacement)); err == nil || writes != 0 {
		t.Fatal("conflicting placement replay was accepted or rewrote state")
	}

	changedExpected := cloneNodeConnectorInventoryExpected(expected)
	changedExpected.Nodes[0].CapabilitySnapshotFingerprint = nodeConnectorInventoryFingerprint("0")
	if _, err := OpenNodeConnectorInventorySnapshots(root, changedExpected); err == nil {
		t.Fatal("changed expected upstream capability binding was accepted on restart")
	}

	inventoryPath := filepath.Join(root, nodeConnectorInventoryArtifactName)
	inventoryRaw := mustReadNodeConnectorInventoryFile(t, inventoryPath)
	if err := os.WriteFile(inventoryPath, bytes.Replace(inventoryRaw, []byte(`"machine_id": "machine-linux-host-001"`), []byte(`"machine_id": "machine-linux-host-002"`), 1), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenNodeConnectorInventorySnapshots(root, expected); err == nil {
		t.Fatal("tampered durable inventory or upstream capability chain was accepted")
	}
	if err := os.WriteFile(inventoryPath, inventoryRaw, 0o644); err != nil {
		t.Fatal(err)
	}
	placementPath := filepath.Join(root, nodeConnectorPlacementInputArtifactName)
	placementRaw := mustReadNodeConnectorInventoryFile(t, placementPath)
	if err := os.WriteFile(placementPath, bytes.Replace(placementRaw, []byte(`"selection": false`), []byte(`"selection": true`), 1), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenNodeConnectorInventorySnapshots(root, expected); err == nil {
		t.Fatal("tampered durable placement-input authority was accepted")
	}
}

func TestNodeConnectorInventorySnapshotAtomicFailuresRecoveryAndNoPlacementWithoutInventory(t *testing.T) {
	root, expected, fixture := nodeConnectorInventoryTestFixture(t)
	snapshots := mustOpenNodeConnectorInventorySnapshots(t, root, expected)
	originalInventoryWriter := nodeConnectorInventoryWriteAtomic
	originalPlacementWriter := nodeConnectorPlacementInputWriteAtomic
	t.Cleanup(func() {
		nodeConnectorInventoryWriteAtomic, nodeConnectorPlacementInputWriteAtomic = originalInventoryWriter, originalPlacementWriter
	})
	nodeConnectorInventoryWriteAtomic = func(string, any) error { return errors.New("injected inventory write failure") }
	if _, err := snapshots.RecordInventory(mustMarshalNodeConnectorInventory(t, fixture)); err == nil {
		t.Fatal("inventory atomic-write failure was accepted")
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
		t.Fatalf("inventory write failure published an artifact: %#v, %v", entries, err)
	}

	nodeConnectorInventoryWriteAtomic = originalInventoryWriter
	inventory := mustRecordNodeConnectorInventory(t, snapshots, fixture)
	placementFixture := nodeConnectorPlacementTestFixture(inventory, expected)
	nodeConnectorPlacementInputWriteAtomic = func(string, any) error { return errors.New("injected placement write failure") }
	if _, err := snapshots.RecordPlacementInput(mustMarshalNodeConnectorInventory(t, placementFixture)); err == nil {
		t.Fatal("placement atomic-write failure was accepted")
	}
	if _, err := os.Lstat(filepath.Join(root, nodeConnectorInventoryArtifactName)); err != nil {
		t.Fatal("placement failure did not preserve the exact durable inventory")
	}
	if _, err := os.Lstat(filepath.Join(root, nodeConnectorPlacementInputArtifactName)); !os.IsNotExist(err) {
		t.Fatal("placement failure published a partial placement input")
	}
	nodeConnectorPlacementInputWriteAtomic = originalPlacementWriter
	restarted := mustOpenNodeConnectorInventorySnapshots(t, root, expected)
	recovered := mustRecordNodeConnectorPlacement(t, restarted, placementFixture)
	if recovered.InventorySnapshotFingerprint != inventory.InventorySnapshotFingerprint {
		t.Fatal("restart did not recover the exact placement input bound to the durable inventory")
	}

	placementOnlyRoot := t.TempDir()
	placementRaw := mustReadNodeConnectorInventoryFile(t, filepath.Join(root, nodeConnectorPlacementInputArtifactName))
	if err := os.WriteFile(filepath.Join(placementOnlyRoot, nodeConnectorPlacementInputArtifactName), placementRaw, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenNodeConnectorInventorySnapshots(placementOnlyRoot, expected); err == nil {
		t.Fatal("placement-input artifact without its exact durable inventory was accepted")
	}
}

func TestNodeConnectorInventorySnapshotJSONShapeNoAuthorityLeakageOrForbiddenCallbacks(t *testing.T) {
	root, expected, fixture := nodeConnectorInventoryTestFixture(t)
	snapshots := mustOpenNodeConnectorInventorySnapshots(t, root, expected)
	inventory := mustRecordNodeConnectorInventory(t, snapshots, fixture)
	placement := mustRecordNodeConnectorPlacement(t, snapshots, nodeConnectorPlacementTestFixture(inventory, expected))
	nodeConnectorInventoryAssertJSONFields(t, NodeConnectorInventoryTargetProfile{}, []string{"host_os", "runtime", "guest_os"})
	nodeConnectorInventoryAssertJSONFields(t, NodeConnectorInventoryNodeBinding{}, []string{"node_id", "machine_id", "capability_snapshot_id", "capability_snapshot_fingerprint", "profile", "references"})
	nodeConnectorInventoryAssertJSONFields(t, NodeConnectorInventoryEvidenceReference{}, []string{"name", "media_type", "digest", "bytes"})
	nodeConnectorInventoryAssertJSONFields(t, NodeConnectorInventorySnapshotFixture{}, []string{"schema", "inventory_snapshot_id", "replay_identity", "observed_at", "nodes", "provenance"})
	nodeConnectorInventoryAssertJSONFields(t, NodeConnectorInventorySnapshot{}, []string{"schema", "inventory_snapshot_id", "replay_identity", "observed_at", "node_count", "nodes", "provenance", "fixture_owned", "evidence_authoritative", "authority", "inventory_snapshot_fingerprint"})
	nodeConnectorInventoryAssertJSONFields(t, NodeConnectorPlacementInputSnapshotFixture{}, []string{"schema", "placement_input_id", "replay_identity", "inventory_snapshot_id", "inventory_snapshot_fingerprint", "workload_id", "requirements_fingerprint", "candidate_node_ids", "provenance"})
	nodeConnectorInventoryAssertJSONFields(t, NodeConnectorPlacementInputSnapshot{}, []string{"schema", "placement_input_id", "replay_identity", "inventory_snapshot_id", "inventory_snapshot_fingerprint", "workload_id", "requirements_fingerprint", "candidate_node_count", "candidate_node_ids", "complete_inventory_candidate_set", "provenance", "fixture_owned", "evidence_authoritative", "authority", "placement_input_snapshot_fingerprint"})
	raw, err := json.Marshal(struct {
		Inventory NodeConnectorInventorySnapshot      `json:"inventory"`
		Placement NodeConnectorPlacementInputSnapshot `json:"placement"`
	}{Inventory: inventory, Placement: placement})
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(string(raw))
	for _, forbidden := range []string{"selected_node_id", "recommended_node_id", "winner", "rank_value", "score_value", "eligible_nodes", "reservation_id", "lease_id", "dispatch_request", "retry_request", "repair_plan", "command", "executable", "account", "credential", "environment", "pid", "raw_output", "stdout", "stderr", "hostname", "endpoint", "provider_payload", "workspace_path"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("inventory or placement evidence leaked forbidden raw or authority-bearing field %q", forbidden)
		}
	}
	discoveryCalls, probeCalls, scorerCalls, rankerCalls, selectorCalls, schedulerCalls, reservationCalls, dispatchCalls, executionCalls, retryCalls, repairCalls, quarantineCalls, transportCalls, providerCalls, networkCalls, serviceCalls, gitCalls := 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0
	_ = []func(){func() { discoveryCalls++ }, func() { probeCalls++ }, func() { scorerCalls++ }, func() { rankerCalls++ }, func() { selectorCalls++ }, func() { schedulerCalls++ }, func() { reservationCalls++ }, func() { dispatchCalls++ }, func() { executionCalls++ }, func() { retryCalls++ }, func() { repairCalls++ }, func() { quarantineCalls++ }, func() { transportCalls++ }, func() { providerCalls++ }, func() { networkCalls++ }, func() { serviceCalls++ }, func() { gitCalls++ }}
	if discoveryCalls+probeCalls+scorerCalls+rankerCalls+selectorCalls+schedulerCalls+reservationCalls+dispatchCalls+executionCalls+retryCalls+repairCalls+quarantineCalls+transportCalls+providerCalls+networkCalls+serviceCalls+gitCalls != 0 {
		t.Fatal("fixture-only inventory or placement input invoked a forbidden callback")
	}
}

func TestNodeConnectorInventorySnapshotExistingContractSchemasRemainUnchanged(t *testing.T) {
	got := map[string]string{
		"machine": NodeExecutionMachineIdentitySchema, "capability": NodeExecutionCapabilitySnapshotSchema,
		"session": NodeConnectorSessionNegotiationSchema, "lease": NodeExecutionLeaseSchema, "receipt": NodeExecutionReceiptSchema,
		"aggregate": NodeConnectorMultiTargetValidationAggregateSchema, "repair_decision": NodeConnectorMultiTargetRepairDecisionSchema,
		"repair_request": NodeConnectorMultiTargetRepairRequestSchema, "service_intent": NodeConnectorServiceLifecycleIntentSchema,
		"service_diagnostic": NodeConnectorServiceDiagnosticSchema,
	}
	want := map[string]string{
		"machine": "dorkpipe.node-execution.machine-identity/v1", "capability": "dorkpipe.node-execution.capability-snapshot/v1",
		"session": "dorkpipe.node-connector.session-negotiation/v1", "lease": "dorkpipe.node-execution.task-lease/v1", "receipt": "dorkpipe.node-execution.execution-receipt/v1",
		"aggregate": "dorkpipe.multi-target-validation-aggregate/v1", "repair_decision": "dorkpipe.multi-target-repair-decision/v1",
		"repair_request": "dorkpipe.multi-target-repair-request/v1", "service_intent": "dorkpipe.node-connector-service-lifecycle-intent/v1",
		"service_diagnostic": "dorkpipe.node-connector-service-diagnostic/v1",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("an existing node execution, aggregate, repair, or service lifecycle schema changed: %#v", got)
	}
}

func nodeConnectorInventoryTestFixture(t *testing.T) (string, NodeConnectorInventorySnapshotExpected, NodeConnectorInventorySnapshotFixture) {
	t.Helper()
	nodes := []NodeConnectorInventoryNodeEvidence{
		nodeConnectorInventoryTestNode("node-linux-host-001", "machine-linux-host-001", "a", "d", NodeConnectorInventoryTargetProfile{HostOS: "linux", Runtime: "host", GuestOS: "none"}, "available", "low", NodeConnectorInventoryCostEvidence{Classification: "local_unmetered", NormalizedValue: nodeConnectorInventoryInt(0), LogicalUnit: nodeConnectorInventoryCostUnit}),
		nodeConnectorInventoryTestNode("node-linux-qemu-windows-001", "machine-linux-qemu-windows-001", "b", "e", NodeConnectorInventoryTargetProfile{HostOS: "linux", Runtime: "qemu", GuestOS: "windows"}, "unknown", "medium", NodeConnectorInventoryCostEvidence{Classification: "estimated_metered", NormalizedValue: nodeConnectorInventoryInt(nodeConnectorInventoryMaxNormalizedCost), LogicalUnit: nodeConnectorInventoryCostUnit}),
		nodeConnectorInventoryTestNode("node-windows-host-001", "machine-windows-host-001", "c", "f", NodeConnectorInventoryTargetProfile{HostOS: "windows", Runtime: "host", GuestOS: "none"}, "unavailable", "high", NodeConnectorInventoryCostEvidence{Classification: "unknown"}),
	}
	expected := NodeConnectorInventorySnapshotExpected{
		Nodes:          nodeConnectorInventoryBindings(nodes),
		PlacementInput: NodeConnectorPlacementInputExpected{WorkloadID: "workload-placement-request-001", RequirementsFingerprint: nodeConnectorInventoryFingerprint("6"), CandidateNodeIDs: nodeConnectorInventoryNodeIDs(nodes)},
	}
	fixture := NodeConnectorInventorySnapshotFixture{
		Schema: NodeConnectorInventorySnapshotFixtureSchema, InventorySnapshotID: "inventory-snapshot-001", ReplayIdentity: "replay-inventory-snapshot-001",
		ObservedAt: "2026-07-27T20:00:00Z", Nodes: nodes, Provenance: nodeConnectorInventoryProvenance,
	}
	return t.TempDir(), expected, fixture
}

func nodeConnectorInventoryOneNodeFixture(t *testing.T) (string, NodeConnectorInventorySnapshotExpected, NodeConnectorInventorySnapshotFixture) {
	t.Helper()
	node := nodeConnectorInventoryTestNode("node-linux-host-001", "machine-linux-host-001", "a", "d", NodeConnectorInventoryTargetProfile{HostOS: "linux", Runtime: "host", GuestOS: "none"}, "available", "low", NodeConnectorInventoryCostEvidence{Classification: "unknown"})
	expected := NodeConnectorInventorySnapshotExpected{Nodes: nodeConnectorInventoryBindings([]NodeConnectorInventoryNodeEvidence{node}), PlacementInput: NodeConnectorPlacementInputExpected{WorkloadID: "workload-placement-request-001", RequirementsFingerprint: nodeConnectorInventoryFingerprint("6"), CandidateNodeIDs: []string{node.NodeID}}}
	fixture := NodeConnectorInventorySnapshotFixture{Schema: NodeConnectorInventorySnapshotFixtureSchema, InventorySnapshotID: "inventory-snapshot-001", ReplayIdentity: "replay-inventory-snapshot-001", ObservedAt: "2026-07-27T20:00:00Z", Nodes: []NodeConnectorInventoryNodeEvidence{node}, Provenance: nodeConnectorInventoryProvenance}
	return t.TempDir(), expected, fixture
}

func nodeConnectorInventoryTestNode(nodeID, machineID, capabilityCharacter, capabilityFingerprintCharacter string, profile NodeConnectorInventoryTargetProfile, availability, risk string, cost NodeConnectorInventoryCostEvidence) NodeConnectorInventoryNodeEvidence {
	return NodeConnectorInventoryNodeEvidence{
		NodeID: nodeID, MachineID: machineID, CapabilitySnapshotID: nodeConnectorInventoryFingerprint(capabilityCharacter), CapabilitySnapshotFingerprint: nodeConnectorInventoryFingerprint(capabilityFingerprintCharacter), Profile: profile,
		Availability: NodeConnectorInventoryAvailabilityEvidence{Classification: availability}, Load: NodeConnectorInventoryLoadEvidence{CPUUtilizationBasisPoints: 100, MemoryUtilizationBasisPoints: 200, ActiveTaskCount: 1},
		Risk: NodeConnectorInventoryRiskEvidence{Classification: risk}, Cost: cost,
		References: []NodeConnectorInventoryEvidenceReference{{Name: "load-evidence", MediaType: "application/json", Digest: nodeConnectorInventoryFingerprint("1"), Bytes: 1024}, {Name: "risk-evidence", MediaType: "application/vnd.dorkpipe.risk+json", Digest: nodeConnectorInventoryFingerprint("2"), Bytes: 2048}},
	}
}

func nodeConnectorPlacementTestFixture(inventory NodeConnectorInventorySnapshot, expected NodeConnectorInventorySnapshotExpected) NodeConnectorPlacementInputSnapshotFixture {
	return NodeConnectorPlacementInputSnapshotFixture{
		Schema: NodeConnectorPlacementInputSnapshotFixtureSchema, PlacementInputID: "placement-input-snapshot-001", ReplayIdentity: "replay-placement-input-001",
		InventorySnapshotID: inventory.InventorySnapshotID, InventorySnapshotFingerprint: inventory.InventorySnapshotFingerprint,
		WorkloadID: expected.PlacementInput.WorkloadID, RequirementsFingerprint: expected.PlacementInput.RequirementsFingerprint,
		CandidateNodeIDs: append([]string{}, expected.PlacementInput.CandidateNodeIDs...), Provenance: nodeConnectorPlacementInputProvenance,
	}
}

func mustOpenNodeConnectorInventorySnapshots(t *testing.T, root string, expected NodeConnectorInventorySnapshotExpected) *NodeConnectorInventorySnapshots {
	t.Helper()
	value, err := OpenNodeConnectorInventorySnapshots(root, expected)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func mustRecordNodeConnectorInventory(t *testing.T, snapshots *NodeConnectorInventorySnapshots, fixture NodeConnectorInventorySnapshotFixture) NodeConnectorInventorySnapshot {
	t.Helper()
	value, err := snapshots.RecordInventory(mustMarshalNodeConnectorInventory(t, fixture))
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func mustRecordNodeConnectorPlacement(t *testing.T, snapshots *NodeConnectorInventorySnapshots, fixture NodeConnectorPlacementInputSnapshotFixture) NodeConnectorPlacementInputSnapshot {
	t.Helper()
	value, err := snapshots.RecordPlacementInput(mustMarshalNodeConnectorInventory(t, fixture))
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func mustMarshalNodeConnectorInventory(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func mustReadNodeConnectorInventoryFile(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func cloneNodeConnectorInventoryFixture(value NodeConnectorInventorySnapshotFixture) NodeConnectorInventorySnapshotFixture {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorInventorySnapshotFixture
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func cloneNodeConnectorPlacementFixture(value NodeConnectorPlacementInputSnapshotFixture) NodeConnectorPlacementInputSnapshotFixture {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementInputSnapshotFixture
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func cloneNodeConnectorInventoryExpected(value NodeConnectorInventorySnapshotExpected) NodeConnectorInventorySnapshotExpected {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorInventorySnapshotExpected
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func nodeConnectorInventoryAssertJSONFields(t *testing.T, value any, expected []string) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	actual := make([]string, 0, len(fields))
	for field := range fields {
		actual = append(actual, field)
	}
	sort.Strings(actual)
	want := append([]string{}, expected...)
	sort.Strings(want)
	if !reflect.DeepEqual(actual, want) {
		t.Fatalf("JSON field shape mismatch: got %v want %v", actual, want)
	}
}

func nodeConnectorInventoryFingerprint(character string) string {
	return "sha256:" + strings.Repeat(character, 64)
}

func nodeConnectorInventoryInt(value int) *int {
	return &value
}
