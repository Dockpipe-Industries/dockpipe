package executor

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"dockpipe.vm/tools/internal/manifest"
	"dockpipe.vm/tools/internal/provisioning"
)

func testSHA(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

type fakeRunner struct {
	calls  []string
	failAt string
}

func (f *fakeRunner) call(name string) error {
	f.calls = append(f.calls, name)
	if f.failAt == name {
		return errors.New("injected failure")
	}
	return nil
}

func (f *fakeRunner) CreateOSClone(context.Context, OSCloneRequest) error {
	return f.call("create-private-os-clone")
}
func (f *fakeRunner) CreateSparseRawDisk(context.Context, SparseRawDiskRequest) error {
	return f.call("create-private-data-disk")
}
func (f *fakeRunner) CreateNoCloudSeed(context.Context, NoCloudSeedRequest) error {
	return f.call("create-nocloud-seed")
}
func (f *fakeRunner) LaunchQEMU(context.Context, LaunchRequest) error { return f.call("launch-qemu") }
func (f *fakeRunner) VerifyGuest(context.Context, GuestVerificationRequest) error {
	return f.call("verify-guest")
}
func (f *fakeRunner) ControlledShutdown(context.Context, ShutdownRequest) error {
	return f.call("controlled-shutdown")
}
func (f *fakeRunner) PreserveFailure(context.Context, PreservationRequest) error {
	return f.call("preserve-failure")
}
func (f *fakeRunner) Cleanup(context.Context, CleanupRequest) error { return f.call("cleanup") }

func executorFixture(t *testing.T) Contract {
	t.Helper()
	root := t.TempDir()
	volumeRoot := filepath.VolumeName(root) + string(filepath.Separator)
	runtimeBase := filepath.Join(volumeRoot, "dpvm-fixture-"+testSHA([]byte(root))[:8])
	instance := filepath.Join(root, "instances", "run-001", "cohort-001")
	evidence := filepath.Join(root, "evidence", "run-001", "cohort-001")
	runtime := filepath.Join(runtimeBase, "run-001", "cohort-001")
	config := filepath.Join(root, "config", "instances", "run-001", "cohort-001")
	osDisk := filepath.Join(instance, "os-private.qcow2")
	dataDisk := filepath.Join(instance, "qualification.raw")
	seed := filepath.Join(instance, "nocloud-seed.iso")
	qmp := filepath.Join(runtime, "run-001.qmp")
	agent := filepath.Join(runtime, "run-001.agent")
	observation, err := provisioning.PlanFirstBootObservation(filepath.Join(root, "evidence"), runtimeBase, "run-001", "cohort-001")
	if err != nil {
		t.Fatal(err)
	}
	qemuImage := provisioning.PinnedCommand{ToolID: provisioning.ToolQEMUImage, Binary: filepath.Join(root, "toolchain", "bin", provisioning.ToolQEMUImage), BinarySHA256: strings.Repeat("1", 64), Version: "qemu-img version 11.0.3", Args: []string{"create", "-f", "qcow2", "-F", "qcow2", "-b", filepath.Join(root, "source.img"), osDisk}, TimeoutSeconds: 120}
	qemuSystem := provisioning.PinnedCommand{ToolID: provisioning.ToolQEMUSystem, Binary: filepath.Join(root, "toolchain", "bin", provisioning.ToolQEMUSystem), BinarySHA256: strings.Repeat("2", 64), Version: "QEMU emulator version 11.0.3", Args: []string{"-nodefaults", "-nic", "none", "-chardev", "socket,id=dockpipe-first-boot-console,path=" + observation.SocketPath + ",server=off,reconnect-ms=0", "-serial", "chardev:dockpipe-first-boot-console"}, TimeoutSeconds: 120}
	c := Contract{
		Schema: Schema, ContractSHA256: strings.Repeat("a", 64), PlanSHA256: strings.Repeat("b", 64), ToolchainSHA256: strings.Repeat("c", 64), RunID: "run-001", CohortID: "cohort-001",
		ProvisioningRoots: &provisioning.Roots{Instances: filepath.Join(root, "instances"), Evidence: filepath.Join(root, "evidence"), Config: filepath.Join(root, "config"), Runtime: runtimeBase},
		OSClone:           OSCloneRequest{Command: qemuImage, Source: filepath.Join(root, "source.img"), SourceSHA256: strings.Repeat("d", 64), Target: osDisk, Format: "qcow2", BackingFormat: "qcow2", Exclusive: true, SourceReadOnly: true},
		DataDisk:          SparseRawDiskRequest{Target: dataDisk, Bytes: manifest.DataDiskBytes, Mode: 0o600, Exclusive: true, Sparse: true},
		NoCloud: NoCloudSeedRequest{Builder: NoCloudBuilder, Label: provisioning.SeedLabel, Target: seed, Mode: 0o600, Exclusive: true, Files: []SeedFile{
			{Name: "meta-data", Mode: 0o600, SHA256: testSHA([]byte("meta-data")), Content: []byte("meta-data")},
			{Name: "network-config", Mode: 0o600, SHA256: testSHA([]byte("network-config")), Content: []byte("network-config")},
			{Name: "user-data", Mode: 0o600, SHA256: testSHA([]byte("user-data")), Content: []byte("user-data")},
		}},
		Launch:               LaunchRequest{Command: qemuSystem, QMP: qmp, AgentSocket: agent, ProcessRecord: filepath.Join(runtime, "process.json")},
		FirstBootObservation: &observation,
		Guest: GuestVerificationRequest{
			Socket: agent,
			Bootstrap: IdentityBootstrapRequest{
				Kind: "bootstrap", Capability: "identity/v1", BootstrapNonce: strings.Repeat("3", 64), BootIDSource: manifest.KernelBootIDSource, Sequence: 1, Phase: "bootstrap",
				GuestWritesFirst: true, ControllerReadsFirst: true, GuestSigned: true,
				ExclusiveEvidencePath: filepath.Join(evidence, "bootstrap.json"), EvidenceMode: 0o600, EvidenceExclusive: true, FsyncEvidenceFile: true, FsyncEvidenceDir: true,
			},
			Capabilities: []string{"identity/v1", "health/v1", "launch-hash-pinned/v1"}, FirstRequestSequence: 2,
			BootIDFromBootstrap: true, ContiguousSequence: true, RejectNonceReuse: true,
			TimeoutSeconds: 300, ControllerSigned: true, GuestSigned: true, Evidence: filepath.Join(evidence, "verification.json"), FailureEvidence: filepath.Join(evidence, "verification-failure.json"),
		},
		Shutdown:     ShutdownRequest{QMP: qmp, ProcessRecord: filepath.Join(runtime, "process.json"), Command: ControlledPowerdown, TimeoutSeconds: 120, Evidence: filepath.Join(evidence, "shutdown.json")},
		Preservation: PreservationRequest{Roots: []string{instance, evidence, config, runtime}, TimeoutSeconds: PreservationDeadline},
		Cleanup:      CleanupRequest{Resources: []string{seed, dataDisk, osDisk, filepath.Join(instance, "seed-tree"), filepath.Join(runtime, "process.json"), agent, qmp, observation.SocketPath, runtime, config, evidence, instance}, SeparateAuthorization: true},
		sealed:       true,
	}
	c.ExecutionSHA256, err = c.Digest()
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestInjectedRunnerExecutesOnlyClosedOrderAndNeverCleansAutomatically(t *testing.T) {
	c := executorFixture(t)
	runner := &fakeRunner{}
	result, err := Execute(context.Background(), c, runner)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"create-private-os-clone", "create-private-data-disk", "create-nocloud-seed", "launch-qemu", "verify-guest", "controlled-shutdown"}
	if !slices.Equal(runner.calls, want) || !slices.Equal(result.Completed, want) || result.Preserved || result.CleanupRun {
		t.Fatalf("unexpected closed execution: calls=%v result=%+v", runner.calls, result)
	}
}

func TestBuildDerivesClosedExecutorContractFromAuthorizedPlan(t *testing.T) {
	root := t.TempDir()
	rootDigest := sha256.Sum256([]byte(root))
	volumeRoot := filepath.VolumeName(root) + string(filepath.Separator)
	shortRuntime := filepath.Join(volumeRoot, "dpvm-ex-"+hex.EncodeToString(rootDigest[:4]))
	m, err := manifest.Load(filepath.Join("..", "..", "..", "manifests", "linux-qualification.json"))
	if err != nil {
		t.Fatal(err)
	}
	toolchainRoot := filepath.Join(root, "toolchain")
	for _, dir := range []string{toolchainRoot, filepath.Join(toolchainRoot, "bin"), filepath.Join(toolchainRoot, "lib")} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	qemuImageData := []byte("qemu-img")
	qemuSystemData := []byte("qemu-system")
	loaderData := []byte("loader")
	for _, file := range []struct {
		path string
		data []byte
		mode os.FileMode
	}{
		{filepath.Join(toolchainRoot, "bin", "qemu-img"), qemuImageData, 0o500},
		{filepath.Join(toolchainRoot, "bin", "qemu-system-x86_64"), qemuSystemData, 0o500},
		{filepath.Join(toolchainRoot, "lib", "loader"), loaderData, 0o500},
	} {
		if err := os.WriteFile(file.path, file.data, file.mode); err != nil {
			t.Fatal(err)
		}
	}
	qemuImage := provisioning.ToolPin{ID: provisioning.ToolQEMUImage, RelativePath: "bin/qemu-img", SHA256: testSHA(qemuImageData), Version: "qemu-img version 11.0.3", Mode: 0o500}
	qemuSystem := provisioning.ToolPin{ID: provisioning.ToolQEMUSystem, RelativePath: "bin/qemu-system-x86_64", SHA256: testSHA(qemuSystemData), Version: "QEMU emulator version 11.0.3", Mode: 0o500}
	toolchain := provisioning.ToolchainManifest{
		Schema: provisioning.ToolchainSchema, BundleID: provisioning.ToolchainBundleID, BundleVersion: "11.0.3-linux-amd64.1", OS: "linux", Architecture: "amd64", QEMUVersion: provisioning.ToolchainQEMUVersion,
		Source:            provisioning.ToolchainSource{URL: provisioning.ToolchainSourceURL, SignatureURL: provisioning.ToolchainSignatureURL, ArchiveSHA256: strings.Repeat("1", 64), SignerFingerprint: provisioning.ToolchainSigner},
		BuildRecipeSHA256: strings.Repeat("2", 64), Tools: []provisioning.ToolPin{qemuImage, qemuSystem}, RuntimeFiles: []provisioning.FilePin{{RelativePath: "lib/loader", SHA256: testSHA(loaderData), Mode: 0o500}},
	}
	toolchainJSON, err := json.Marshal(toolchain)
	if err != nil {
		t.Fatal(err)
	}
	toolchainManifest := filepath.Join(toolchainRoot, provisioning.ToolchainManifestName)
	if err := os.WriteFile(toolchainManifest, toolchainJSON, 0o400); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{filepath.Join(toolchainRoot, "bin"), filepath.Join(toolchainRoot, "lib"), toolchainRoot} {
		if err := os.Chmod(dir, 0o500); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		_ = os.Chmod(toolchainRoot, 0o700)
		_ = os.Chmod(filepath.Join(toolchainRoot, "bin"), 0o700)
		_ = os.Chmod(filepath.Join(toolchainRoot, "lib"), 0o700)
	})
	m.QEMU.BinaryPath = filepath.Join(toolchainRoot, filepath.FromSlash(qemuSystem.RelativePath))
	m.QEMU.BinarySHA256 = qemuSystem.SHA256
	m.QEMU.Version = qemuSystem.Version
	controllerPublic, controllerPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	guestPublic, guestPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	controllerBinary := []byte("controller")
	guestBinary := []byte("guest")
	harnessBinary := []byte("harness")
	assetsRoot, err := filepath.Abs(filepath.Join("..", "..", "..", "workflows", "linux-vm", "assets"))
	if err != nil {
		t.Fatal(err)
	}
	c := provisioning.Contract{
		Schema: provisioning.Schema, Purpose: manifest.QualificationPurpose, Disposable: true, InstanceCount: 1,
		RunID: m.RunID, CohortID: "cohort-001", MachineUUID: m.MachineUUID, DiskSerial: m.DataDisk.Serial, FilesystemUUID: m.Filesystem.UUID, BootstrapNonce: strings.Repeat("3", 64),
		SourceImage: provisioning.SourceImage{Path: filepath.Join(root, "source.img"), SHA256: manifest.UbuntuImageSHA256, Bytes: provisioning.PinnedImageBytes},
		Toolchain:   provisioning.ToolchainReference{Root: toolchainRoot, Manifest: toolchainManifest, ManifestSHA256: testSHA(toolchainJSON)},
		Roots:       provisioning.Roots{Instances: filepath.Join(root, "instances"), Evidence: filepath.Join(root, "evidence"), Config: filepath.Join(root, "config"), Runtime: shortRuntime},
		Artifacts:   provisioning.Artifacts{AssetsRoot: assetsRoot, ControllerBinary: filepath.Join(root, "controller"), ControllerBinarySHA256: testSHA(controllerBinary), GuestAgentBinary: filepath.Join(root, "guest"), GuestAgentBinarySHA256: testSHA(guestBinary), HarnessBinary: filepath.Join(root, "harness"), HarnessBinarySHA256: testSHA(harnessBinary), ControllerPublicKeySHA256: testSHA(controllerPublic), GuestPublicKeySHA256: testSHA(guestPublic)},
		Execution:   provisioning.RequiredExecutionPolicy(),
	}
	contractSHA, _ := c.Digest()
	wantKinds := []string{"reserve-identities", "verify-source-image", "create-private-os-clone", "create-private-data-disk", "render-nocloud", "create-nocloud-seed", "install-hash-pinned-assets", "format-and-mount-data-disk", "launch-qemu", "capture-first-boot-console", "verify-guest", "controlled-shutdown", "preserve-failure", "cleanup"}
	operations := make([]provisioning.Operation, len(wantKinds))
	for i, kind := range wantKinds {
		operations[i] = provisioning.Operation{Order: i + 1, Kind: kind}
	}
	observation, err := provisioning.PlanFirstBootObservation(c.Roots.Evidence, c.Roots.Runtime, c.RunID, c.CohortID)
	if err != nil {
		t.Fatal(err)
	}
	p := provisioning.Plan{Schema: provisioning.PlanSchema, ContractSHA256: contractSHA, ToolchainSHA256: c.Toolchain.ManifestSHA256, RunID: c.RunID, CohortID: c.CohortID, LiveAuthorized: true, AuthorizationRequired: true, FirstBootObservation: observation, Operations: operations}
	instance := filepath.Join(c.Roots.Instances, c.RunID, c.CohortID)
	osDisk := filepath.Join(instance, "os-private.qcow2")
	clone := provisioning.PinnedCommand{ToolID: provisioning.ToolQEMUImage, Binary: filepath.Join(toolchainRoot, "bin", "qemu-img"), BinarySHA256: qemuImage.SHA256, Version: qemuImage.Version, Args: []string{"create", "-f", "qcow2", "-F", "qcow2", "-b", c.SourceImage.Path, osDisk}, TimeoutSeconds: 120}
	cloneJSON, _ := json.Marshal(clone)
	operations[2].Inputs = []string{string(cloneJSON)}
	qemuPlan, err := manifest.PlanProvisioningQEMU(m, filepath.Join(c.Roots.Runtime, c.RunID, c.CohortID), osDisk, filepath.Join(instance, "qualification.raw"), filepath.Join(instance, "nocloud-seed.iso"), observation.SocketPath)
	if err != nil {
		t.Fatal(err)
	}
	launch := provisioning.PinnedCommand{ToolID: provisioning.ToolQEMUSystem, Binary: filepath.Join(toolchainRoot, "bin", "qemu-system-x86_64"), BinarySHA256: qemuSystem.SHA256, Version: qemuSystem.Version, Args: qemuPlan.Args, TimeoutSeconds: 120}
	launchJSON, _ := json.Marshal(launch)
	operations[8].Inputs = []string{string(launchJSON)}
	observationJSON, _ := observation.CanonicalJSON()
	operations[9].Inputs = []string{observationJSON}
	operations[9].Outputs = []string{observation.EvidencePath}
	operations[10].Outputs = []string{filepath.Join(c.Roots.Evidence, c.RunID, c.CohortID, "bootstrap.json"), filepath.Join(c.Roots.Evidence, c.RunID, c.CohortID, "verification.json"), filepath.Join(c.Roots.Evidence, c.RunID, c.CohortID, "verification-failure.json")}
	p.Operations = operations
	p.PlanSHA256, _ = p.Digest()
	material := provisioning.RenderMaterial{Keys: provisioning.KeyMaterial{ControllerPublic: controllerPublic, ControllerPrivate: controllerPrivate, GuestPublic: guestPublic, GuestPrivate: guestPrivate}, ControllerBinary: controllerBinary, GuestAgentBinary: guestBinary, HarnessBinary: harnessBinary}
	execution, err := Build(c, p, m, "/checkout", material)
	if err != nil {
		t.Fatal(err)
	}
	if execution.ToolchainSHA256 != c.Toolchain.ManifestSHA256 || execution.OSClone.Command.Binary != filepath.Join(toolchainRoot, "bin", "qemu-img") || execution.NoCloud.Builder != NoCloudBuilder || execution.FirstBootObservation == nil || *execution.FirstBootObservation != observation || execution.Guest.Bootstrap.BootstrapNonce != c.BootstrapNonce || execution.Guest.Bootstrap.BootIDSource != manifest.KernelBootIDSource || !execution.Guest.Bootstrap.GuestWritesFirst || !execution.Guest.Bootstrap.ControllerReadsFirst || execution.Guest.Bootstrap.ControllerSigned || execution.Guest.Bootstrap.EvidenceMode != 0o600 || !execution.Guest.Bootstrap.EvidenceExclusive || !execution.Guest.Bootstrap.FsyncEvidenceFile || !execution.Guest.Bootstrap.FsyncEvidenceDir || execution.Guest.FirstRequestSequence != 2 || !execution.Guest.BootIDFromBootstrap || !execution.Guest.ContiguousSequence || !execution.Guest.RejectNonceReuse || execution.Guest.FailureEvidence != filepath.Join(c.Roots.Evidence, c.RunID, c.CohortID, "verification-failure.json") || execution.Shutdown.FallbackSignal || !execution.Cleanup.SeparateAuthorization {
		t.Fatalf("derived executor contract changed: %+v", execution)
	}
	tampered := p
	tampered.Operations = slices.Clone(p.Operations)
	tampered.Operations[2].Inputs = []string{`{"tool_id":"qemu-img","args":["arbitrary"]}`}
	tampered.PlanSHA256, _ = tampered.Digest()
	if _, err := Build(c, tampered, m, "/checkout", material); err == nil {
		t.Fatal("expected authorized-plan command substitution rejection")
	}
	tamperedObservation := p
	tamperedObservation.FirstBootObservation.SocketPath = filepath.Join(c.Roots.Runtime, "substituted.sock")
	tamperedObservation.PlanSHA256, _ = tamperedObservation.Digest()
	if _, err := Build(c, tamperedObservation, m, "/checkout", material); err == nil {
		t.Fatal("expected authorized-plan observation path substitution rejection")
	}
	unauthorized := p
	unauthorized.LiveAuthorized = false
	if _, err := Build(c, unauthorized, m, "/checkout", material); err == nil {
		t.Fatal("expected no executor production reach before exact authorization")
	}
	badMaterial := material
	badMaterial.GuestAgentBinary = []byte("substituted guest")
	if _, err := Build(c, p, m, "/checkout", badMaterial); err == nil {
		t.Fatal("expected NoCloud render material substitution rejection")
	}
}

func TestAnyFailureStopsOncePreservesAndNeverRetriesOrCleans(t *testing.T) {
	c := executorFixture(t)
	runner := &fakeRunner{failAt: "launch-qemu"}
	result, err := Execute(context.Background(), c, runner)
	if err == nil || !result.Preserved {
		t.Fatalf("expected preserved failure, got result=%+v err=%v", result, err)
	}
	want := []string{"create-private-os-clone", "create-private-data-disk", "create-nocloud-seed", "launch-qemu", "preserve-failure"}
	if !slices.Equal(runner.calls, want) {
		t.Fatalf("failure retried, fell back, or cleaned: %v", runner.calls)
	}
}

func TestCleanupRequiresFreshExactSeparateAuthorization(t *testing.T) {
	c := executorFixture(t)
	now := time.Unix(1_800_000_000, 0)
	auth := CleanupAuthorization{Schema: CleanupSchema, Approved: true, ContractSHA256: c.ContractSHA256, PlanSHA256: c.PlanSHA256, ExecutionSHA256: c.ExecutionSHA256, RunID: c.RunID, CohortID: c.CohortID, Resources: slices.Clone(c.Cleanup.Resources), ExpiresAtUnix: now.Add(5 * time.Minute).Unix()}
	runner := &fakeRunner{}
	if _, err := ExecuteCleanup(context.Background(), c, CleanupAuthorization{}, now, runner); err == nil {
		t.Fatal("expected missing cleanup authorization rejection")
	}
	changed := auth
	changed.Resources = slices.Clone(auth.Resources)
	changed.Resources[0] = filepath.Join(t.TempDir(), "substitution")
	if _, err := ExecuteCleanup(context.Background(), c, changed, now, runner); err == nil {
		t.Fatal("expected cleanup resource substitution rejection")
	}
	result, err := ExecuteCleanup(context.Background(), c, auth, now, runner)
	if err != nil || !result.CleanupRun || !slices.Equal(runner.calls, []string{"cleanup"}) {
		t.Fatalf("exact cleanup failed: result=%+v calls=%v err=%v", result, runner.calls, err)
	}
}

func TestContractTamperingFailsBeforeRunner(t *testing.T) {
	c := executorFixture(t)
	c.Guest.Bootstrap.GuestWritesFirst = false
	c.ExecutionSHA256, _ = c.Digest()
	runner := &fakeRunner{}
	if _, err := Execute(context.Background(), c, runner); err == nil {
		t.Fatal("expected executor-contract substitution rejection")
	}
	if len(runner.calls) != 0 {
		t.Fatalf("runner called for invalid contract: %v", runner.calls)
	}
}

func TestObservationTransportTamperingFailsBeforeRunner(t *testing.T) {
	c := executorFixture(t)
	for i, arg := range c.Launch.Command.Args {
		if strings.Contains(arg, "id=dockpipe-first-boot-console") {
			c.Launch.Command.Args[i] = strings.Replace(arg, "server=off", "server=on", 1)
		}
	}
	c.ExecutionSHA256, _ = c.Digest()
	runner := &fakeRunner{}
	if _, err := Execute(context.Background(), c, runner); err == nil {
		t.Fatal("expected QEMU observation transport substitution rejection")
	}
	if len(runner.calls) != 0 {
		t.Fatalf("runner called for substituted observation transport: %v", runner.calls)
	}
}

func TestUnsealedExecutorContractCannotReachRunner(t *testing.T) {
	c := executorFixture(t)
	c.sealed = false
	runner := &fakeRunner{}
	if _, err := Execute(context.Background(), c, runner); err == nil {
		t.Fatal("expected directly constructed executor contract rejection")
	}
	if len(runner.calls) != 0 {
		t.Fatalf("runner called for unsealed contract: %v", runner.calls)
	}
}
