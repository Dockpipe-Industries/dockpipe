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
	Schema                string                   `json:"schema"`
	ContractSHA256        string                   `json:"contract_sha256"`
	ToolchainSHA256       string                   `json:"toolchain_sha256"`
	PlanSHA256            string                   `json:"plan_sha256"`
	RunID                 string                   `json:"run_id"`
	CohortID              string                   `json:"cohort_id"`
	LiveAuthorized        bool                     `json:"live_authorized"`
	Execute               bool                     `json:"execute"`
	AuthorizationRequired bool                     `json:"authorization_required"`
	FirstBootObservation  FirstBootObservationPlan `json:"first_boot_observation"`
	Operations            []Operation              `json:"operations"`
}

// Digest binds authorization to the complete immutable plan rather than only
// its contract metadata. Runtime authorization flags and the digest field are
// deliberately excluded so authorization cannot change what was reviewed.
func (p Plan) Digest() (string, error) {
	material := struct {
		Schema                string                   `json:"schema"`
		ContractSHA256        string                   `json:"contract_sha256"`
		ToolchainSHA256       string                   `json:"toolchain_sha256"`
		RunID                 string                   `json:"run_id"`
		CohortID              string                   `json:"cohort_id"`
		AuthorizationRequired bool                     `json:"authorization_required"`
		FirstBootObservation  FirstBootObservationPlan `json:"first_boot_observation"`
		Operations            []Operation              `json:"operations"`
	}{p.Schema, p.ContractSHA256, p.ToolchainSHA256, p.RunID, p.CohortID, p.AuthorizationRequired, p.FirstBootObservation, p.Operations}
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

type PinnedCommand struct {
	ToolID         string   `json:"tool_id"`
	Binary         string   `json:"binary"`
	BinarySHA256   string   `json:"binary_sha256"`
	Version        string   `json:"version"`
	Args           []string `json:"args"`
	TimeoutSeconds int      `json:"timeout_seconds"`
}

var operationKinds = map[string]struct{}{
	"reserve-identities": {}, "verify-source-image": {}, "create-private-os-clone": {},
	"create-private-data-disk": {}, "render-nocloud": {}, "create-nocloud-seed": {},
	"install-hash-pinned-assets": {}, "format-and-mount-data-disk": {}, "launch-qemu": {},
	"capture-first-boot-console": {}, "verify-guest": {}, "controlled-shutdown": {}, "preserve-failure": {}, "cleanup": {},
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
	if err := ValidatePinnedBinary(c.Artifacts.HarnessBinary, c.Artifacts.HarnessBinarySHA256); err != nil {
		return Plan{}, err
	}
	if err := ValidateReviewedAssets(c.Artifacts.AssetsRoot); err != nil {
		return Plan{}, err
	}
	toolchain, err := LoadToolchain(c.Toolchain)
	if err != nil {
		return Plan{}, err
	}
	if err := toolchain.Validate(c.Toolchain, checkoutRoot, c.Roots); err != nil {
		return Plan{}, err
	}
	qemuSystem, err := toolchain.Tool(ToolQEMUSystem)
	if err != nil {
		return Plan{}, err
	}
	qemuImage, err := toolchain.Tool(ToolQEMUImage)
	if err != nil {
		return Plan{}, err
	}
	qemuSystemPath := filepath.Join(c.Toolchain.Root, filepath.FromSlash(qemuSystem.RelativePath))
	if m.QEMU.BinaryPath != qemuSystemPath || m.QEMU.BinarySHA256 != qemuSystem.SHA256 || m.QEMU.Version != qemuSystem.Version {
		return Plan{}, fmt.Errorf("qualification QEMU identity does not match the exact task-owned toolchain")
	}
	configurationSHA256, err := manifest.ConfigurationSHA256(m)
	if err != nil || m.QEMU.ConfigurationSHA256 != configurationSHA256 {
		return Plan{}, fmt.Errorf("qualification configuration SHA-256 mismatch")
	}
	if m.Protocol.ControllerBinarySHA256 != c.Artifacts.ControllerBinarySHA256 || m.Protocol.GuestAgentBinarySHA256 != c.Artifacts.GuestAgentBinarySHA256 || m.Protocol.HarnessSHA256 != c.Artifacts.HarnessBinarySHA256 || m.Protocol.ControllerPublicKeySHA256 != c.Artifacts.ControllerPublicKeySHA256 || m.Protocol.GuestPublicKeySHA256 != c.Artifacts.GuestPublicKeySHA256 {
		return Plan{}, fmt.Errorf("qualification protocol pins do not match the provisioning contract")
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
	observation, err := PlanFirstBootObservation(c.Roots.Evidence, c.Roots.Runtime, c.RunID, c.CohortID)
	if err != nil {
		return Plan{}, err
	}
	observationJSON, err := observation.CanonicalJSON()
	if err != nil {
		return Plan{}, err
	}
	qemuPlan, err := manifest.PlanProvisioningQEMU(m, runtime, osDisk, dataDisk, seed, observation.SocketPath)
	if err != nil {
		return Plan{}, err
	}
	qemuJSON, err := qemuPlan.CanonicalJSON()
	if err != nil {
		return Plan{}, err
	}
	cloneCommand := PinnedCommand{
		ToolID: ToolQEMUImage, Binary: filepath.Join(c.Toolchain.Root, filepath.FromSlash(qemuImage.RelativePath)),
		BinarySHA256: qemuImage.SHA256, Version: qemuImage.Version,
		Args:           []string{"create", "-f", "qcow2", "-F", "qcow2", "-b", c.SourceImage.Path, osDisk},
		TimeoutSeconds: c.Execution.CloneTimeoutSeconds,
	}
	cloneJSON, err := json.Marshal(cloneCommand)
	if err != nil {
		return Plan{}, err
	}
	launchCommand := PinnedCommand{
		ToolID: ToolQEMUSystem, Binary: qemuPlan.Binary, BinarySHA256: qemuSystem.SHA256,
		Version: qemuSystem.Version, Args: qemuPlan.Args, TimeoutSeconds: c.Execution.LaunchTimeoutSeconds,
	}
	launchJSON, err := json.Marshal(launchCommand)
	if err != nil {
		return Plan{}, err
	}
	operations := []Operation{
		{1, "reserve-identities", []string{c.RunID, c.CohortID, c.MachineUUID, c.DiskSerial, c.FilesystemUUID, c.BootstrapNonce}, []string{instance, evidence, config, runtime}, []string{"exclusive creation", "owner-only private keys and bootstrap nonce", "no replacement"}},
		{2, "verify-source-image", []string{c.SourceImage.Path, c.SourceImage.SHA256, fmt.Sprint(c.SourceImage.Bytes)}, nil, []string{"regular non-symlink", "owner-only", "read-only source"}},
		{3, "create-private-os-clone", []string{string(cloneJSON)}, []string{osDisk}, []string{"exclusive instance root", "target absent", "never mutate source", "no fallback tool"}},
		{4, "create-private-data-disk", []string{"raw", fmt.Sprint(manifest.DataDiskBytes), "sparse"}, []string{dataDisk}, []string{"exclusive create", "single private data disk"}},
		{5, "render-nocloud", []string{c.Artifacts.AssetsRoot, c.Artifacts.ControllerBinarySHA256, c.Artifacts.GuestAgentBinarySHA256, c.Artifacts.ControllerPublicKeySHA256, c.Artifacts.GuestPublicKeySHA256, c.DiskSerial, c.FilesystemUUID}, []string{filepath.Join(instance, "seed-tree")}, []string{"network disabled", "SSH disabled", "no packages", "no arbitrary commands", "compiled reviewed-asset hashes exact"}},
		{6, "create-nocloud-seed", []string{filepath.Join(instance, "seed-tree"), SeedLabel, "dockpipe-go-iso9660-v1"}, []string{seed}, []string{"exclusive create", "deterministic ISO9660", "local NoCloud only", "no external seed tool"}},
		{7, "install-hash-pinned-assets", []string{c.Artifacts.ControllerBinary, c.Artifacts.GuestAgentBinary, c.Artifacts.HarnessBinary, c.Artifacts.AssetsRoot}, []string{"/usr/libexec/dockpipe-guest-agent", "/usr/libexec/dockpipe-sqlite-vm-harness", "/etc/systemd/system/dockpipe-agent.service", "/etc/udev/rules.d/99-dockpipe-agent.rules"}, []string{"binary hashes exact", "mutual public-key pins exact", "reviewed systemd sandbox", "exact virtio-port group and mode persist across boots", "test-only harness has no arbitrary argument surface"}},
		{8, "format-and-mount-data-disk", []string{"/dev/disk/by-id/virtio-" + c.DiskSerial, c.FilesystemUUID, strings.Join(manifest.RequiredMountOptions, ",")}, []string{manifest.QualificationMount}, []string{"whole-device ext4", "lazy initialization disabled", "mount by UUID"}},
		{9, "launch-qemu", []string{string(launchJSON), qemuJSON, qmp, agentSocket}, []string{filepath.Join(runtime, "process.json")}, []string{"KVM only", "network none", "exact two private writable disks plus one read-only NoCloud seed", "no passthrough or shares", "no fallback tool"}},
		{10, "capture-first-boot-console", []string{observationJSON}, []string{observation.EvidencePath}, []string{"controller creates listener and exclusive owner-only evidence", "QEMU is a one-shot client", "capture only isa-serial/ttyS0", "4 MiB prefix cap", "overflow fails closed", "stop and join before verification returns, shutdown, or preservation"}},
		{11, "verify-guest", []string{agentSocket, "guest-first-signed-identity/v1", c.BootstrapNonce, manifest.KernelBootIDSource, "identity/v1", "health/v1", "launch-hash-pinned/v1", fmt.Sprint(c.Execution.GuestVerificationTimeoutSeconds)}, []string{filepath.Join(evidence, "bootstrap.json"), filepath.Join(evidence, "verification.json"), filepath.Join(evidence, "verification-failure.json")}, []string{"guest-signed bootstrap before controller writes", "controller-signed requests from sequence 2", "guest-signed responses", "durable non-secret timeout receipt", "replay and identity protection", "hash pins exact"}},
		{12, "controlled-shutdown", []string{qmp, filepath.Join(runtime, "process.json"), "system_powerdown", fmt.Sprint(c.Execution.ShutdownTimeoutSeconds)}, []string{filepath.Join(evidence, "shutdown.json")}, []string{"exact owned process", "bounded wait", "no fallback signal"}},
		{13, "preserve-failure", []string{instance, evidence, config, runtime}, nil, []string{"any failure preserves complete instance", "no automatic retry", "no automatic cleanup", "first-boot prefix and parent fsynced"}},
		{14, "cleanup", []string{c.RunID, c.CohortID, instance, evidence, config, runtime}, nil, []string{"later explicit approval", "exact ordered enumeration", "refuse failed or completed roots"}},
	}
	for i, op := range operations {
		if op.Order != i+1 {
			return Plan{}, fmt.Errorf("provisioning operation order is not contiguous")
		}
		if _, ok := operationKinds[op.Kind]; !ok {
			return Plan{}, fmt.Errorf("unknown provisioning operation %q", op.Kind)
		}
	}
	plan := Plan{Schema: PlanSchema, ContractSHA256: digest, ToolchainSHA256: c.Toolchain.ManifestSHA256, RunID: c.RunID, CohortID: c.CohortID, Execute: false, AuthorizationRequired: true, FirstBootObservation: observation, Operations: operations}
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
	if plan.Schema != PlanSchema || plan.ContractSHA256 != auth.ContractSHA256 || plan.ToolchainSHA256 != c.Toolchain.ManifestSHA256 || plan.PlanSHA256 != auth.PlanSHA256 || plan.RunID != auth.RunID || plan.CohortID != auth.CohortID || plan.Execute || plan.FirstBootObservation.Validate() != nil {
		return Plan{}, fmt.Errorf("authorization target is not the exact inert provisioning plan")
	}
	plan.LiveAuthorized = true
	// Authorization keeps the reviewed plan inert. A separately selected closed
	// executor path may derive and run only the exact sealed operations.
	plan.Execute = false
	return plan, nil
}
