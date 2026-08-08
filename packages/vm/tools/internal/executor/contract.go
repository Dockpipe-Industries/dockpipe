package executor

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"dockpipe.vm/tools/internal/manifest"
	"dockpipe.vm/tools/internal/provisioning"
)

const (
	Schema                          = "dockpipe.vm.executor.v6"
	LegacyUserCreationCleanupSchema = "dockpipe.vm.executor.v5"
	LegacyDeadlineCleanupSchema     = "dockpipe.vm.executor.v4"
	LegacyObservationCleanupSchema  = "dockpipe.vm.executor.v3"
	LegacyCleanupSchema             = "dockpipe.vm.executor.v2"
	CleanupSchema                   = "dockpipe.vm.cleanup-authorization.v1"
	NoCloudBuilder                  = "dockpipe-go-iso9660-v1"
	ControlledPowerdown             = "system_powerdown"
	PreservationDeadline            = 30
)

type Contract struct {
	Schema               string                                 `json:"schema"`
	ContractSHA256       string                                 `json:"contract_sha256"`
	PlanSHA256           string                                 `json:"plan_sha256"`
	ToolchainSHA256      string                                 `json:"toolchain_sha256"`
	ExecutionSHA256      string                                 `json:"execution_sha256"`
	RunID                string                                 `json:"run_id"`
	CohortID             string                                 `json:"cohort_id"`
	ProvisioningRoots    *provisioning.Roots                    `json:"provisioning_roots,omitempty"`
	OSClone              OSCloneRequest                         `json:"os_clone"`
	DataDisk             SparseRawDiskRequest                   `json:"data_disk"`
	NoCloud              NoCloudSeedRequest                     `json:"nocloud"`
	Launch               LaunchRequest                          `json:"launch"`
	FirstBootObservation *provisioning.FirstBootObservationPlan `json:"first_boot_observation,omitempty"`
	Guest                GuestVerificationRequest               `json:"guest_verification"`
	Shutdown             ShutdownRequest                        `json:"shutdown"`
	Preservation         PreservationRequest                    `json:"preservation"`
	Cleanup              CleanupRequest                         `json:"cleanup"`
	sealed               bool
}

type OSCloneRequest struct {
	Command        provisioning.PinnedCommand `json:"command"`
	Source         string                     `json:"source"`
	SourceSHA256   string                     `json:"source_sha256"`
	Target         string                     `json:"target"`
	Format         string                     `json:"format"`
	BackingFormat  string                     `json:"backing_format"`
	Exclusive      bool                       `json:"exclusive"`
	SourceReadOnly bool                       `json:"source_read_only"`
}

type SparseRawDiskRequest struct {
	Target    string `json:"target"`
	Bytes     int64  `json:"bytes"`
	Mode      uint32 `json:"mode"`
	Exclusive bool   `json:"exclusive"`
	Sparse    bool   `json:"sparse"`
}

type SeedFile struct {
	Name    string `json:"name"`
	Mode    uint32 `json:"mode"`
	SHA256  string `json:"sha256"`
	Content []byte `json:"content"`
}

type NoCloudSeedRequest struct {
	Builder   string     `json:"builder"`
	Label     string     `json:"label"`
	Target    string     `json:"target"`
	Mode      uint32     `json:"mode"`
	Exclusive bool       `json:"exclusive"`
	Files     []SeedFile `json:"files"`
}

type LaunchRequest struct {
	Command       provisioning.PinnedCommand `json:"command"`
	QMP           string                     `json:"qmp"`
	AgentSocket   string                     `json:"agent_socket"`
	ProcessRecord string                     `json:"process_record"`
	Network       bool                       `json:"network"`
	SSH           bool                       `json:"ssh"`
}

type GuestVerificationRequest struct {
	Socket               string                   `json:"socket"`
	Bootstrap            IdentityBootstrapRequest `json:"bootstrap"`
	Capabilities         []string                 `json:"capabilities"`
	FirstRequestSequence uint64                   `json:"first_request_sequence"`
	BootIDFromBootstrap  bool                     `json:"boot_id_from_bootstrap"`
	ContiguousSequence   bool                     `json:"contiguous_sequence"`
	RejectNonceReuse     bool                     `json:"reject_nonce_reuse"`
	TimeoutSeconds       int                      `json:"timeout_seconds"`
	ControllerSigned     bool                     `json:"controller_signed"`
	GuestSigned          bool                     `json:"guest_signed"`
	Evidence             string                   `json:"evidence"`
}

type IdentityBootstrapRequest struct {
	Kind                  string `json:"kind"`
	Capability            string `json:"capability"`
	BootstrapNonce        string `json:"bootstrap_nonce"`
	BootIDSource          string `json:"boot_id_source"`
	Sequence              uint64 `json:"sequence"`
	Phase                 string `json:"phase"`
	GuestWritesFirst      bool   `json:"guest_writes_first"`
	ControllerReadsFirst  bool   `json:"controller_reads_first"`
	GuestSigned           bool   `json:"guest_signed"`
	ControllerSigned      bool   `json:"controller_signed"`
	ExclusiveEvidencePath string `json:"exclusive_evidence_path"`
	EvidenceMode          uint32 `json:"evidence_mode"`
	EvidenceExclusive     bool   `json:"evidence_exclusive"`
	FsyncEvidenceFile     bool   `json:"fsync_evidence_file"`
	FsyncEvidenceDir      bool   `json:"fsync_evidence_dir"`
}

type ShutdownRequest struct {
	QMP            string `json:"qmp"`
	ProcessRecord  string `json:"process_record"`
	Command        string `json:"command"`
	TimeoutSeconds int    `json:"timeout_seconds"`
	FallbackSignal bool   `json:"fallback_signal"`
	Evidence       string `json:"evidence"`
}

type PreservationRequest struct {
	Roots            []string `json:"roots"`
	TimeoutSeconds   int      `json:"timeout_seconds"`
	AutomaticRetry   bool     `json:"automatic_retry"`
	AutomaticCleanup bool     `json:"automatic_cleanup"`
}

type CleanupRequest struct {
	Resources             []string `json:"resources"`
	SeparateAuthorization bool     `json:"separate_authorization"`
	Automatic             bool     `json:"automatic"`
}

type CleanupAuthorization struct {
	Schema          string   `json:"schema"`
	Approved        bool     `json:"approved"`
	ContractSHA256  string   `json:"contract_sha256"`
	PlanSHA256      string   `json:"plan_sha256"`
	ExecutionSHA256 string   `json:"execution_sha256"`
	RunID           string   `json:"run_id"`
	CohortID        string   `json:"cohort_id"`
	Resources       []string `json:"resources"`
	ExpiresAtUnix   int64    `json:"expires_at_unix"`
}

func Build(c provisioning.Contract, p provisioning.Plan, m manifest.Manifest, checkoutRoot string, material provisioning.RenderMaterial) (Contract, error) {
	var out Contract
	contractSHA, err := c.Digest()
	if err != nil {
		return out, err
	}
	planSHA, err := p.Digest()
	if err != nil {
		return out, err
	}
	if p.Schema != provisioning.PlanSchema || !p.LiveAuthorized || p.Execute || p.ContractSHA256 != contractSHA || p.PlanSHA256 != planSHA || p.ToolchainSHA256 != c.Toolchain.ManifestSHA256 {
		return out, fmt.Errorf("executor requires the exact authorized inert provisioning plan")
	}
	if len(p.Operations) != 14 {
		return out, fmt.Errorf("executor requires the complete closed provisioning plan")
	}
	wantKinds := []string{"reserve-identities", "verify-source-image", "create-private-os-clone", "create-private-data-disk", "render-nocloud", "create-nocloud-seed", "install-hash-pinned-assets", "format-and-mount-data-disk", "launch-qemu", "capture-first-boot-console", "verify-guest", "controlled-shutdown", "preserve-failure", "cleanup"}
	for i, operation := range p.Operations {
		if operation.Order != i+1 || operation.Kind != wantKinds[i] {
			return out, fmt.Errorf("provisioning operation order or kind changed")
		}
	}
	toolchain, err := provisioning.LoadToolchain(c.Toolchain)
	if err != nil {
		return out, err
	}
	if err := toolchain.Validate(c.Toolchain, checkoutRoot, c.Roots); err != nil {
		return out, err
	}
	qemuImage, err := toolchain.Tool(provisioning.ToolQEMUImage)
	if err != nil {
		return out, err
	}
	qemuSystem, err := toolchain.Tool(provisioning.ToolQEMUSystem)
	if err != nil {
		return out, err
	}
	if m.QEMU.BinaryPath != filepath.Join(c.Toolchain.Root, filepath.FromSlash(qemuSystem.RelativePath)) || m.QEMU.BinarySHA256 != qemuSystem.SHA256 || m.QEMU.Version != qemuSystem.Version {
		return out, fmt.Errorf("executor QEMU identity does not match the authorized task-owned toolchain")
	}
	instance := filepath.Join(c.Roots.Instances, c.RunID, c.CohortID)
	evidence := filepath.Join(c.Roots.Evidence, c.RunID, c.CohortID)
	config := filepath.Join(c.Roots.Config, "instances", c.RunID, c.CohortID)
	runtime := filepath.Join(c.Roots.Runtime, c.RunID, c.CohortID)
	osDisk := filepath.Join(instance, "os-private.qcow2")
	dataDisk := filepath.Join(instance, "qualification.raw")
	seed := filepath.Join(instance, "nocloud-seed.iso")
	qmp := filepath.Join(runtime, m.RunID+".qmp")
	agent := filepath.Join(runtime, m.RunID+".agent")
	observation, err := provisioning.PlanFirstBootObservation(c.Roots.Evidence, c.Roots.Runtime, c.RunID, c.CohortID)
	if err != nil {
		return out, err
	}
	if p.FirstBootObservation != observation {
		return out, fmt.Errorf("executor first-boot observation policy does not match the authorized plan")
	}
	observationJSON, err := observation.CanonicalJSON()
	if err != nil {
		return out, err
	}
	qemuPlan, err := manifest.PlanProvisioningQEMU(m, runtime, osDisk, dataDisk, seed, observation.SocketPath)
	if err != nil {
		return out, err
	}
	clone := provisioning.PinnedCommand{
		ToolID: provisioning.ToolQEMUImage, Binary: filepath.Join(c.Toolchain.Root, filepath.FromSlash(qemuImage.RelativePath)), BinarySHA256: qemuImage.SHA256,
		Version: qemuImage.Version, Args: []string{"create", "-f", "qcow2", "-F", "qcow2", "-b", c.SourceImage.Path, osDisk}, TimeoutSeconds: c.Execution.CloneTimeoutSeconds,
	}
	launch := provisioning.PinnedCommand{
		ToolID: provisioning.ToolQEMUSystem, Binary: filepath.Join(c.Toolchain.Root, filepath.FromSlash(qemuSystem.RelativePath)), BinarySHA256: qemuSystem.SHA256,
		Version: qemuSystem.Version, Args: qemuPlan.Args, TimeoutSeconds: c.Execution.LaunchTimeoutSeconds,
	}
	cloneJSON, err := json.Marshal(clone)
	if err != nil {
		return out, err
	}
	launchJSON, err := json.Marshal(launch)
	if err != nil {
		return out, err
	}
	if len(p.Operations[2].Inputs) != 1 || p.Operations[2].Inputs[0] != string(cloneJSON) || len(p.Operations[8].Inputs) < 1 || p.Operations[8].Inputs[0] != string(launchJSON) || !slices.Equal(p.Operations[9].Inputs, []string{observationJSON}) || !slices.Equal(p.Operations[9].Outputs, []string{observation.EvidencePath}) {
		return out, fmt.Errorf("executor command tuple does not match the authorized plan")
	}
	rendered, err := provisioning.RenderNoCloud(c, m, material)
	if err != nil {
		return out, err
	}
	seedFiles, err := exactSeedFiles(rendered)
	if err != nil {
		return out, err
	}
	out = Contract{
		Schema: Schema, ContractSHA256: contractSHA, PlanSHA256: planSHA, ToolchainSHA256: c.Toolchain.ManifestSHA256, RunID: c.RunID, CohortID: c.CohortID,
		ProvisioningRoots:    &c.Roots,
		OSClone:              OSCloneRequest{Command: clone, Source: c.SourceImage.Path, SourceSHA256: c.SourceImage.SHA256, Target: osDisk, Format: "qcow2", BackingFormat: "qcow2", Exclusive: true, SourceReadOnly: true},
		DataDisk:             SparseRawDiskRequest{Target: dataDisk, Bytes: manifest.DataDiskBytes, Mode: 0o600, Exclusive: true, Sparse: true},
		NoCloud:              NoCloudSeedRequest{Builder: NoCloudBuilder, Label: provisioning.SeedLabel, Target: seed, Mode: 0o600, Exclusive: true, Files: seedFiles},
		Launch:               LaunchRequest{Command: launch, QMP: qmp, AgentSocket: agent, ProcessRecord: filepath.Join(runtime, "process.json")},
		FirstBootObservation: &observation,
		Guest: GuestVerificationRequest{
			Socket: agent,
			Bootstrap: IdentityBootstrapRequest{
				Kind: "bootstrap", Capability: "identity/v1", BootstrapNonce: c.BootstrapNonce, BootIDSource: m.BootIDSource, Sequence: 1, Phase: "bootstrap",
				GuestWritesFirst: true, ControllerReadsFirst: true, GuestSigned: true, ControllerSigned: false,
				ExclusiveEvidencePath: filepath.Join(evidence, "bootstrap.json"), EvidenceMode: 0o600, EvidenceExclusive: true, FsyncEvidenceFile: true, FsyncEvidenceDir: true,
			},
			Capabilities: []string{"identity/v1", "health/v1", "launch-hash-pinned/v1"}, FirstRequestSequence: 2,
			BootIDFromBootstrap: true, ContiguousSequence: true, RejectNonceReuse: true,
			TimeoutSeconds: c.Execution.GuestVerificationTimeoutSeconds, ControllerSigned: true, GuestSigned: true, Evidence: filepath.Join(evidence, "verification.json"),
		},
		Shutdown:     ShutdownRequest{QMP: qmp, ProcessRecord: filepath.Join(runtime, "process.json"), Command: ControlledPowerdown, TimeoutSeconds: c.Execution.ShutdownTimeoutSeconds, FallbackSignal: false, Evidence: filepath.Join(evidence, "shutdown.json")},
		Preservation: PreservationRequest{Roots: []string{instance, evidence, config, runtime}, TimeoutSeconds: PreservationDeadline},
		Cleanup:      CleanupRequest{Resources: []string{seed, dataDisk, osDisk, filepath.Join(instance, "seed-tree"), filepath.Join(runtime, "process.json"), agent, qmp, observation.SocketPath, runtime, config, evidence, instance}, SeparateAuthorization: true},
		sealed:       true,
	}
	out.ExecutionSHA256, err = out.Digest()
	if err != nil {
		return Contract{}, err
	}
	return out, out.Validate()
}

func exactSeedFiles(rendered []provisioning.RenderedFile) ([]SeedFile, error) {
	want := []string{"meta-data", "network-config", "user-data"}
	if len(rendered) != len(want) {
		return nil, fmt.Errorf("NoCloud seed must contain exactly three rendered files")
	}
	out := make([]SeedFile, len(rendered))
	for i, file := range rendered {
		if file.Name != want[i] || file.Mode != 0o600 || !isSHA256(file.SHA256) {
			return nil, fmt.Errorf("NoCloud rendered file set, order, mode, or hash changed")
		}
		sum := sha256.Sum256(file.Content)
		if hex.EncodeToString(sum[:]) != file.SHA256 {
			return nil, fmt.Errorf("NoCloud rendered file %s hash mismatch", file.Name)
		}
		out[i] = SeedFile{Name: file.Name, Mode: uint32(file.Mode), SHA256: file.SHA256, Content: slices.Clone(file.Content)}
	}
	return out, nil
}

func (c Contract) Digest() (string, error) {
	copy := c
	copy.ExecutionSHA256 = ""
	b, err := json.Marshal(copy)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func (c Contract) Validate() error {
	digest, err := c.Digest()
	if err != nil || !c.sealed || (c.Schema != Schema && c.Schema != LegacyUserCreationCleanupSchema && c.Schema != LegacyDeadlineCleanupSchema && c.Schema != LegacyObservationCleanupSchema && c.Schema != LegacyCleanupSchema) || c.ExecutionSHA256 != digest || !isSHA256(c.ContractSHA256) || !isSHA256(c.PlanSHA256) || !isSHA256(c.ToolchainSHA256) {
		return fmt.Errorf("executor contract identity or digest is invalid")
	}
	if c.Schema == LegacyCleanupSchema {
		if c.FirstBootObservation != nil || c.ProvisioningRoots != nil {
			return fmt.Errorf("legacy cleanup executor cannot contain current provisioning-root or observation fields")
		}
	} else {
		if c.FirstBootObservation == nil || c.ProvisioningRoots == nil || c.FirstBootObservation.Validate() != nil {
			return fmt.Errorf("first-boot observation contract changed")
		}
		if err := c.validateCurrentPaths(); err != nil {
			return err
		}
		if c.FirstBootObservation.EvidencePath == c.Guest.Evidence || c.FirstBootObservation.SocketPath == c.Launch.QMP || c.FirstBootObservation.SocketPath == c.Launch.AgentSocket {
			return fmt.Errorf("first-boot observation path binding changed")
		}
		if !exactConsoleArgs(c.Launch.Command.Args, *c.FirstBootObservation) {
			return fmt.Errorf("first-boot observation QEMU transport changed")
		}
	}
	if !c.OSClone.Exclusive || !c.OSClone.SourceReadOnly || c.OSClone.Format != "qcow2" || c.OSClone.BackingFormat != "qcow2" || c.OSClone.Command.ToolID != provisioning.ToolQEMUImage || c.OSClone.Command.TimeoutSeconds != 120 {
		return fmt.Errorf("OS clone contract differs from the closed qemu-img tuple")
	}
	wantClone := []string{"create", "-f", "qcow2", "-F", "qcow2", "-b", c.OSClone.Source, c.OSClone.Target}
	if !slices.Equal(c.OSClone.Command.Args, wantClone) {
		return fmt.Errorf("OS clone argument vector differs from the closed qemu-img tuple")
	}
	if !c.DataDisk.Exclusive || !c.DataDisk.Sparse || c.DataDisk.Bytes != manifest.DataDiskBytes || c.DataDisk.Mode != 0o600 {
		return fmt.Errorf("data disk contract differs from the exclusive sparse raw tuple")
	}
	if c.NoCloud.Builder != NoCloudBuilder || c.NoCloud.Label != provisioning.SeedLabel || !c.NoCloud.Exclusive || c.NoCloud.Mode != 0o600 || len(c.NoCloud.Files) != 3 {
		return fmt.Errorf("NoCloud contract differs from the exact internal builder tuple")
	}
	wantSeedNames := []string{"meta-data", "network-config", "user-data"}
	for i, file := range c.NoCloud.Files {
		sum := sha256.Sum256(file.Content)
		if file.Name != wantSeedNames[i] || file.Mode != 0o600 || hex.EncodeToString(sum[:]) != file.SHA256 {
			return fmt.Errorf("NoCloud content, order, mode, or hash changed")
		}
	}
	bootstrap := c.Guest.Bootstrap
	wantGuestTimeout := provisioning.RequiredExecutionPolicy().GuestVerificationTimeoutSeconds
	if c.Schema == LegacyUserCreationCleanupSchema {
		wantGuestTimeout = 240
	} else if c.Schema == LegacyDeadlineCleanupSchema {
		wantGuestTimeout = 180
	} else if c.Schema != Schema {
		wantGuestTimeout = 60
	}
	if c.Launch.Command.ToolID != provisioning.ToolQEMUSystem || c.Launch.Command.TimeoutSeconds != 120 || c.Launch.Network || c.Launch.SSH || c.Guest.TimeoutSeconds != wantGuestTimeout || !c.Guest.ControllerSigned || !c.Guest.GuestSigned || c.Guest.FirstRequestSequence != 2 || !c.Guest.BootIDFromBootstrap || !c.Guest.ContiguousSequence || !c.Guest.RejectNonceReuse || !slices.Equal(c.Guest.Capabilities, []string{"identity/v1", "health/v1", "launch-hash-pinned/v1"}) {
		return fmt.Errorf("launch or signed guest-verification contract changed")
	}
	if bootstrap.Kind != "bootstrap" || bootstrap.Capability != "identity/v1" || !isSHA256(bootstrap.BootstrapNonce) || bootstrap.BootIDSource != manifest.KernelBootIDSource || bootstrap.Sequence != 1 || bootstrap.Phase != "bootstrap" || !bootstrap.GuestWritesFirst || !bootstrap.ControllerReadsFirst || !bootstrap.GuestSigned || bootstrap.ControllerSigned || filepath.Base(bootstrap.ExclusiveEvidencePath) != "bootstrap.json" || bootstrap.EvidenceMode != 0o600 || !bootstrap.EvidenceExclusive || !bootstrap.FsyncEvidenceFile || !bootstrap.FsyncEvidenceDir {
		return fmt.Errorf("guest-first signed identity bootstrap contract changed")
	}
	if c.Shutdown.Command != ControlledPowerdown || c.Shutdown.TimeoutSeconds != 120 || c.Shutdown.FallbackSignal || c.Preservation.TimeoutSeconds != PreservationDeadline || c.Preservation.AutomaticRetry || c.Preservation.AutomaticCleanup || !c.Cleanup.SeparateAuthorization || c.Cleanup.Automatic {
		return fmt.Errorf("shutdown, preservation, or cleanup policy changed")
	}
	return nil
}

func (c Contract) validateCurrentPaths() error {
	roots := *c.ProvisioningRoots
	for _, root := range []string{roots.Instances, roots.Evidence, roots.Config, roots.Runtime} {
		if !filepath.IsAbs(root) || filepath.Clean(root) != root {
			return fmt.Errorf("executor provisioning root binding changed")
		}
	}
	instance := filepath.Join(roots.Instances, c.RunID, c.CohortID)
	evidence := filepath.Join(roots.Evidence, c.RunID, c.CohortID)
	config := filepath.Join(roots.Config, "instances", c.RunID, c.CohortID)
	runtime := filepath.Join(roots.Runtime, c.RunID, c.CohortID)
	observation := c.FirstBootObservation
	if observation.EvidenceRoot != roots.Evidence || observation.RuntimeRoot != roots.Runtime || observation.EvidencePath != filepath.Join(evidence, provisioning.FirstBootObservationFilename) || observation.SocketPath != filepath.Join(runtime, provisioning.FirstBootObservationSocketFilename) {
		return fmt.Errorf("first-boot observation root or path binding changed")
	}
	if c.Launch.QMP != filepath.Join(runtime, c.RunID+".qmp") || c.Launch.AgentSocket != filepath.Join(runtime, c.RunID+".agent") || c.Launch.ProcessRecord != filepath.Join(runtime, "process.json") || c.Guest.Socket != c.Launch.AgentSocket || c.Guest.Bootstrap.ExclusiveEvidencePath != filepath.Join(evidence, "bootstrap.json") || c.Guest.Evidence != filepath.Join(evidence, "verification.json") || c.Shutdown.QMP != c.Launch.QMP || c.Shutdown.ProcessRecord != c.Launch.ProcessRecord || c.Shutdown.Evidence != filepath.Join(evidence, "shutdown.json") {
		return fmt.Errorf("executor launch, verification, or shutdown path binding changed")
	}
	if c.OSClone.Target != filepath.Join(instance, "os-private.qcow2") || c.DataDisk.Target != filepath.Join(instance, "qualification.raw") || c.NoCloud.Target != filepath.Join(instance, "nocloud-seed.iso") {
		return fmt.Errorf("executor instance artifact path binding changed")
	}
	wantPreservation := []string{instance, evidence, config, runtime}
	wantCleanup := []string{c.NoCloud.Target, c.DataDisk.Target, c.OSClone.Target, filepath.Join(instance, "seed-tree"), c.Launch.ProcessRecord, c.Launch.AgentSocket, c.Launch.QMP, observation.SocketPath, runtime, config, evidence, instance}
	if !slices.Equal(c.Preservation.Roots, wantPreservation) || !slices.Equal(c.Cleanup.Resources, wantCleanup) {
		return fmt.Errorf("executor preservation or cleanup path binding changed")
	}
	return nil
}

func exactConsoleArgs(args []string, observation provisioning.FirstBootObservationPlan) bool {
	wantChardev := "socket,id=dockpipe-first-boot-console,path=" + observation.SocketPath + ",server=off,reconnect-ms=0"
	wantSerial := "chardev:dockpipe-first-boot-console"
	consoleChardevCount, expectedSerialCount, totalSerialCount := 0, 0, 0
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "-chardev" && strings.Contains(args[i+1], "id=dockpipe-first-boot-console") {
			if args[i+1] != wantChardev {
				return false
			}
			consoleChardevCount++
		}
		if args[i] == "-serial" {
			totalSerialCount++
			if args[i+1] == wantSerial {
				expectedSerialCount++
			}
		}
	}
	return consoleChardevCount == 1 && expectedSerialCount == 1 && totalSerialCount == 1
}

func (a CleanupAuthorization) Validate(c Contract, now time.Time) error {
	if a.Schema != CleanupSchema || !a.Approved || a.ContractSHA256 != c.ContractSHA256 || a.PlanSHA256 != c.PlanSHA256 || a.ExecutionSHA256 != c.ExecutionSHA256 || a.RunID != c.RunID || a.CohortID != c.CohortID || !slices.Equal(a.Resources, c.Cleanup.Resources) {
		return fmt.Errorf("cleanup authorization does not match the exact executor contract and resources")
	}
	if a.ExpiresAtUnix <= now.Unix() || a.ExpiresAtUnix > now.Add(15*time.Minute).Unix() {
		return fmt.Errorf("cleanup authorization must be fresh and expire within 15 minutes")
	}
	return nil
}

func Store(path string, c Contract) error {
	if err := c.Validate(); err != nil {
		return err
	}
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return writeDurableExclusive(path, b)
}

func Load(path string) (Contract, error) {
	var c Contract
	if err := decodeOwnerOnly(path, &c); err != nil {
		return c, err
	}
	c.sealed = true
	return c, c.Validate()
}

func LoadCleanupAuthorization(path string) (CleanupAuthorization, error) {
	var authorization CleanupAuthorization
	if err := decodeOwnerOnly(path, &authorization); err != nil {
		return authorization, err
	}
	return authorization, nil
}

func decodeOwnerOnly(path string, out any) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("executor input path must be absolute")
	}
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		return fmt.Errorf("executor input must be an owner-only regular non-symlink file")
	}
	if stat, ok := info.Sys().(*syscall.Stat_t); ok && int(stat.Uid) != os.Geteuid() {
		return fmt.Errorf("executor input must be owned by the current user")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return fmt.Errorf("executor input contains trailing JSON")
	}
	return nil
}

func writeDurableExclusive(path string, data []byte) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("executor evidence path must be absolute")
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func isSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, r := range value {
		if !strings.ContainsRune("0123456789abcdef", r) {
			return false
		}
	}
	return true
}
