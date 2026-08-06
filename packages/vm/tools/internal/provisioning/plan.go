package provisioning

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dockpipe.vm/tools/internal/manifest"
	"dockpipe.vm/tools/internal/xdg"
)

type Plan struct {
	Schema                string      `json:"schema"`
	ContractSHA256        string      `json:"contract_sha256"`
	PlanSHA256            string      `json:"plan_sha256"`
	RunID                 string      `json:"run_id"`
	CohortID              string      `json:"cohort_id"`
	LiveAuthorized        bool        `json:"live_authorized"`
	Execute               bool        `json:"execute"`
	AuthorizationRequired bool        `json:"authorization_required"`
	Operations            []Operation `json:"operations"`
}

// Digest binds authorization to the complete immutable plan rather than only
// its contract metadata. Runtime authorization flags and the digest field are
// deliberately excluded so authorization cannot change what was reviewed.
func (p Plan) Digest() (string, error) {
	material := struct {
		Schema                string      `json:"schema"`
		ContractSHA256        string      `json:"contract_sha256"`
		RunID                 string      `json:"run_id"`
		CohortID              string      `json:"cohort_id"`
		AuthorizationRequired bool        `json:"authorization_required"`
		Operations            []Operation `json:"operations"`
	}{p.Schema, p.ContractSHA256, p.RunID, p.CohortID, p.AuthorizationRequired, p.Operations}
	b, err := json.Marshal(material)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

// Operation is a closed set of typed package actions. It is not a command or
// shell surface, and planning never invokes a subprocess.
type Operation struct {
	Order      int      `json:"order"`
	Kind       string   `json:"kind"`
	Inputs     []string `json:"inputs"`
	Outputs    []string `json:"outputs"`
	Assertions []string `json:"assertions"`
}

var operationKinds = map[string]struct{}{
	"reserve-identities": {}, "verify-source-image": {}, "create-private-os-clone": {},
	"create-private-data-disk": {}, "render-nocloud": {}, "create-nocloud-seed": {},
	"install-hash-pinned-assets": {}, "format-and-mount-data-disk": {}, "launch-qemu": {},
	"verify-guest": {}, "controlled-shutdown": {}, "preserve-failure": {}, "cleanup": {},
}

func BuildPlan(c Contract, paths xdg.Paths, m manifest.Manifest, checkoutRoot string, inspector ImageInspector) (Plan, error) {
	if err := m.Validate(); err != nil {
		return Plan{}, err
	}
	if err := c.Validate(paths, m, checkoutRoot); err != nil {
		return Plan{}, err
	}
	if inspector == nil {
		return Plan{}, fmt.Errorf("source image inspector is required")
	}
	if err := ValidateSourceImage(c.SourceImage, inspector); err != nil {
		return Plan{}, err
	}
	if err := ValidatePinnedBinary(c.Artifacts.ControllerBinary, c.Artifacts.ControllerBinarySHA256); err != nil {
		return Plan{}, err
	}
	if err := ValidatePinnedBinary(c.Artifacts.GuestAgentBinary, c.Artifacts.GuestAgentBinarySHA256); err != nil {
		return Plan{}, err
	}
	if err := ValidateReviewedAssets(c.Artifacts.AssetsRoot); err != nil {
		return Plan{}, err
	}
	instance := filepath.Join(c.Roots.Instances, c.RunID, c.CohortID)
	evidence := filepath.Join(c.Roots.Evidence, c.RunID, c.CohortID)
	config := filepath.Join(c.Roots.Config, "instances", c.RunID, c.CohortID)
	runtime := filepath.Join(c.Roots.Runtime, c.RunID, c.CohortID)
	for _, path := range []string{instance, evidence, config, runtime} {
		if _, err := os.Lstat(path); err == nil {
			return Plan{}, fmt.Errorf("fresh exclusive path already exists: %s", path)
		} else if !os.IsNotExist(err) {
			return Plan{}, fmt.Errorf("inspect exclusive path %s: %w", path, err)
		}
	}
	digest, err := c.Digest()
	if err != nil {
		return Plan{}, err
	}
	osDisk := filepath.Join(instance, "os-private.qcow2")
	dataDisk := filepath.Join(instance, "qualification.raw")
	seed := filepath.Join(instance, "nocloud-seed.iso")
	qmp := filepath.Join(runtime, m.RunID+".qmp")
	agentSocket := filepath.Join(runtime, m.RunID+".agent")
	qemuPlan, err := manifest.PlanProvisioningQEMU(m, runtime, osDisk, dataDisk, seed)
	if err != nil {
		return Plan{}, err
	}
	qemuJSON, err := qemuPlan.CanonicalJSON()
	if err != nil {
		return Plan{}, err
	}
	operations := []Operation{
		{1, "reserve-identities", []string{c.RunID, c.CohortID, c.MachineUUID, c.DiskSerial, c.FilesystemUUID, c.Nonce}, []string{instance, evidence, config, runtime}, []string{"exclusive creation", "owner-only private keys", "no replacement"}},
		{2, "verify-source-image", []string{c.SourceImage.Path, c.SourceImage.SHA256, fmt.Sprint(c.SourceImage.Bytes)}, nil, []string{"regular non-symlink", "owner-only", "read-only source"}},
		{3, "create-private-os-clone", []string{c.SourceImage.Path, "qcow2", "backing-read-only"}, []string{osDisk}, []string{"exclusive create", "never mutate source"}},
		{4, "create-private-data-disk", []string{"raw", fmt.Sprint(manifest.DataDiskBytes), "sparse"}, []string{dataDisk}, []string{"exclusive create", "single private data disk"}},
		{5, "render-nocloud", []string{c.Artifacts.AssetsRoot, c.Artifacts.ControllerBinarySHA256, c.Artifacts.GuestAgentBinarySHA256, c.Artifacts.ControllerPublicKeySHA256, c.Artifacts.GuestPublicKeySHA256, c.DiskSerial, c.FilesystemUUID}, []string{filepath.Join(instance, "seed-tree")}, []string{"network disabled", "SSH disabled", "no packages", "no arbitrary commands", "compiled reviewed-asset hashes exact"}},
		{6, "create-nocloud-seed", []string{filepath.Join(instance, "seed-tree"), SeedLabel}, []string{seed}, []string{"exclusive create", "local NoCloud only"}},
		{7, "install-hash-pinned-assets", []string{c.Artifacts.ControllerBinary, c.Artifacts.GuestAgentBinary, c.Artifacts.AssetsRoot}, []string{"/usr/libexec/dockpipe-guest-agent", "/etc/systemd/system/dockpipe-agent.service"}, []string{"binary hashes exact", "mutual public-key pins exact", "reviewed systemd sandbox"}},
		{8, "format-and-mount-data-disk", []string{"/dev/disk/by-id/virtio-" + c.DiskSerial, c.FilesystemUUID, strings.Join(manifest.RequiredMountOptions, ",")}, []string{manifest.QualificationMount}, []string{"whole-device ext4", "lazy initialization disabled", "mount by UUID"}},
		{9, "launch-qemu", []string{m.QEMU.BinarySHA256, qemuJSON, qmp, agentSocket}, []string{filepath.Join(runtime, "process.json")}, []string{"KVM only", "network none", "exact two private writable disks plus one read-only NoCloud seed", "no passthrough or shares"}},
		{10, "verify-guest", []string{agentSocket, "identity/v1", "health/v1", "launch-hash-pinned/v1"}, []string{filepath.Join(evidence, "verification.json")}, []string{"signed framed protocol", "replay and identity protection", "hash pins exact"}},
		{11, "controlled-shutdown", []string{qmp, filepath.Join(runtime, "process.json")}, []string{filepath.Join(evidence, "shutdown.json")}, []string{"separate authorization", "exact owned process", "bounded wait", "no fallback signal"}},
		{12, "preserve-failure", []string{instance, evidence, config, runtime}, nil, []string{"any failure preserves complete instance", "no automatic retry", "no automatic cleanup"}},
		{13, "cleanup", []string{c.RunID, c.CohortID, instance, evidence, config, runtime}, nil, []string{"later explicit approval", "exact ordered enumeration", "refuse failed or completed roots"}},
	}
	for i, op := range operations {
		if op.Order != i+1 {
			return Plan{}, fmt.Errorf("provisioning operation order is not contiguous")
		}
		if _, ok := operationKinds[op.Kind]; !ok {
			return Plan{}, fmt.Errorf("unknown provisioning operation %q", op.Kind)
		}
	}
	plan := Plan{Schema: PlanSchema, ContractSHA256: digest, RunID: c.RunID, CohortID: c.CohortID, Execute: false, AuthorizationRequired: true, Operations: operations}
	plan.PlanSHA256, err = plan.Digest()
	if err != nil {
		return Plan{}, err
	}
	return plan, nil
}

func AuthorizePlan(plan Plan, c Contract, auth LiveAuthorization, now time.Time) (Plan, error) {
	if err := auth.Validate(c, plan, now); err != nil {
		return Plan{}, err
	}
	if plan.Schema != PlanSchema || plan.ContractSHA256 != auth.ContractSHA256 || plan.PlanSHA256 != auth.PlanSHA256 || plan.RunID != auth.RunID || plan.CohortID != auth.CohortID || plan.Execute {
		return Plan{}, fmt.Errorf("authorization target is not the exact inert provisioning plan")
	}
	plan.LiveAuthorized = true
	// This slice intentionally has no executor. Gate 2 must consume this exact
	// authorized plan through a separately reviewed package-owned execution gate.
	plan.Execute = false
	return plan, nil
}
