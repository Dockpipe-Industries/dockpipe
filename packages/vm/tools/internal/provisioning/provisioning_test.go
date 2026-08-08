package provisioning

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"dockpipe.vm/tools/internal/manifest"
	"dockpipe.vm/tools/internal/xdg"
)

type fakeFileInfo struct {
	mode fs.FileMode
	size int64
}

func (f fakeFileInfo) Name() string       { return PinnedImageFilename }
func (f fakeFileInfo) Size() int64        { return f.size }
func (f fakeFileInfo) Mode() fs.FileMode  { return f.mode }
func (f fakeFileInfo) ModTime() time.Time { return time.Unix(0, 0) }
func (f fakeFileInfo) IsDir() bool        { return false }
func (f fakeFileInfo) Sys() any           { return nil }

type fakeInspector struct {
	info   os.FileInfo
	digest string
	err    error
}

func (f fakeInspector) Lstat(string) (os.FileInfo, error) { return f.info, f.err }
func (f fakeInspector) SHA256(string) (string, error)     { return f.digest, f.err }

func qualification(t *testing.T) manifest.Manifest {
	t.Helper()
	m, err := manifest.Load(filepath.Join("..", "..", "..", "manifests", "linux-qualification.json"))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func contractFixture(t *testing.T) (Contract, xdg.Paths, RenderMaterial) {
	t.Helper()
	root := t.TempDir()
	rootDigest := sha256.Sum256([]byte(root))
	volumeRoot := filepath.VolumeName(root) + string(filepath.Separator)
	shortRuntime := filepath.Join(volumeRoot, "dpvm-"+hex.EncodeToString(rootDigest[:4]))
	paths := xdg.Paths{
		Images: filepath.Join(root, "cache", "dockpipe", "vm", "images"), Instances: filepath.Join(root, "state", "dockpipe", "vm", "instances"),
		Evidence: filepath.Join(root, "state", "dockpipe", "evidence"), Config: filepath.Join(root, "config", "dockpipe", "vm"), Runtime: shortRuntime,
	}
	controllerPub, controllerPriv, _ := ed25519.GenerateKey(rand.Reader)
	guestPub, guestPriv, _ := ed25519.GenerateKey(rand.Reader)
	controllerBinary := []byte("controller-test-binary")
	guestBinary := []byte("guest-test-binary")
	harnessBinary := []byte("harness-test-binary")
	buildRoot := filepath.Join(root, "build")
	if err := os.MkdirAll(buildRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	controllerPath := filepath.Join(buildRoot, "controller")
	guestPath := filepath.Join(buildRoot, "guest")
	harnessPath := filepath.Join(buildRoot, "harness")
	if err := os.WriteFile(controllerPath, controllerBinary, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(guestPath, guestBinary, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(harnessPath, harnessBinary, 0o700); err != nil {
		t.Fatal(err)
	}
	toolchainRoot := filepath.Join(root, "toolchain")
	for _, dir := range []string{toolchainRoot, filepath.Join(toolchainRoot, "bin"), filepath.Join(toolchainRoot, "lib")} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	qemuSystem := []byte("qemu-system-test-binary")
	qemuImage := []byte("qemu-img-test-binary")
	runtimeLoader := []byte("runtime-loader-test-binary")
	for _, file := range []struct {
		path string
		data []byte
		mode os.FileMode
	}{
		{filepath.Join(toolchainRoot, "bin", ToolQEMUSystem), qemuSystem, 0o500},
		{filepath.Join(toolchainRoot, "bin", ToolQEMUImage), qemuImage, 0o500},
		{filepath.Join(toolchainRoot, "lib", "ld-linux-x86-64.so.2"), runtimeLoader, 0o500},
	} {
		if err := os.WriteFile(file.path, file.data, file.mode); err != nil {
			t.Fatal(err)
		}
	}
	toolchain := ToolchainManifest{
		Schema: ToolchainSchema, BundleID: ToolchainBundleID, BundleVersion: "11.0.3-linux-amd64.1", OS: "linux", Architecture: "amd64", QEMUVersion: ToolchainQEMUVersion,
		Source:            ToolchainSource{URL: ToolchainSourceURL, SignatureURL: ToolchainSignatureURL, ArchiveSHA256: strings.Repeat("a", 64), SignerFingerprint: ToolchainSigner},
		BuildRecipeSHA256: strings.Repeat("b", 64),
		Tools: []ToolPin{
			{ID: ToolQEMUImage, RelativePath: "bin/" + ToolQEMUImage, SHA256: digest(qemuImage), Version: "qemu-img version " + ToolchainQEMUVersion, Mode: 0o500},
			{ID: ToolQEMUSystem, RelativePath: "bin/" + ToolQEMUSystem, SHA256: digest(qemuSystem), Version: "QEMU emulator version " + ToolchainQEMUVersion, Mode: 0o500},
		},
		RuntimeFiles: []FilePin{{RelativePath: "lib/ld-linux-x86-64.so.2", SHA256: digest(runtimeLoader), Mode: 0o500}},
	}
	toolchainJSON, err := json.Marshal(toolchain)
	if err != nil {
		t.Fatal(err)
	}
	toolchainManifest := filepath.Join(toolchainRoot, ToolchainManifestName)
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
	assetsRoot, err := filepath.Abs(filepath.Join("..", "..", "..", "workflows", "linux-vm", "assets"))
	if err != nil {
		t.Fatal(err)
	}
	m := qualification(t)
	c := Contract{
		Schema: Schema, Purpose: "qualification", Disposable: true, InstanceCount: 1,
		RunID: m.RunID, CohortID: "cohort-001", MachineUUID: m.MachineUUID, DiskSerial: m.DataDisk.Serial, FilesystemUUID: m.Filesystem.UUID, BootstrapNonce: strings.Repeat("1", 64),
		SourceImage: SourceImage{Path: filepath.Join(paths.Images, PinnedImageFilename), SHA256: manifest.UbuntuImageSHA256, Bytes: PinnedImageBytes},
		Toolchain:   ToolchainReference{Root: toolchainRoot, Manifest: toolchainManifest, ManifestSHA256: digest(toolchainJSON)},
		Roots:       Roots{Instances: paths.Instances, Evidence: paths.Evidence, Config: paths.Config, Runtime: paths.Runtime},
		Artifacts:   Artifacts{AssetsRoot: assetsRoot, ControllerBinary: controllerPath, ControllerBinarySHA256: digest(controllerBinary), GuestAgentBinary: guestPath, GuestAgentBinarySHA256: digest(guestBinary), HarnessBinary: harnessPath, HarnessBinarySHA256: digest(harnessBinary), ControllerPublicKeySHA256: digest(controllerPub), GuestPublicKeySHA256: digest(guestPub)},
		Execution:   RequiredExecutionPolicy(),
	}
	return c, paths, RenderMaterial{Keys: KeyMaterial{ControllerPublic: controllerPub, ControllerPrivate: controllerPriv, GuestPublic: guestPub, GuestPrivate: guestPriv}, ControllerBinary: controllerBinary, GuestAgentBinary: guestBinary, HarnessBinary: harnessBinary}
}

func qualificationForContract(t *testing.T, c Contract) manifest.Manifest {
	t.Helper()
	m := qualification(t)
	toolchain, err := LoadToolchain(c.Toolchain)
	if err != nil {
		t.Fatal(err)
	}
	qemu, err := toolchain.Tool(ToolQEMUSystem)
	if err != nil {
		t.Fatal(err)
	}
	m.QEMU.BinaryPath = filepath.Join(c.Toolchain.Root, filepath.FromSlash(qemu.RelativePath))
	m.QEMU.BinarySHA256 = qemu.SHA256
	m.QEMU.Version = qemu.Version
	m.RunID = c.RunID
	m.MachineUUID = c.MachineUUID
	m.DataDisk.Serial = c.DiskSerial
	m.Filesystem.UUID = c.FilesystemUUID
	m.Protocol.ControllerBinarySHA256 = c.Artifacts.ControllerBinarySHA256
	m.Protocol.GuestAgentBinarySHA256 = c.Artifacts.GuestAgentBinarySHA256
	m.Protocol.HarnessSHA256 = c.Artifacts.HarnessBinarySHA256
	m.Protocol.ControllerPublicKeySHA256 = c.Artifacts.ControllerPublicKeySHA256
	m.Protocol.GuestPublicKeySHA256 = c.Artifacts.GuestPublicKeySHA256
	m.QEMU.ConfigurationSHA256, err = manifest.ConfigurationSHA256(m)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func TestRequiredExecutionPolicyAllowsPinnedNetworklessCloudInitBoot(t *testing.T) {
	policy := RequiredExecutionPolicy()
	if policy.CloneTimeoutSeconds != 120 || policy.LaunchTimeoutSeconds != 120 || policy.GuestVerificationTimeoutSeconds != 240 || policy.ShutdownTimeoutSeconds != 120 {
		t.Fatalf("reviewed execution deadlines changed: %+v", policy)
	}
	if policy.AutomaticRetry || policy.AutomaticCleanup || policy.FallbackSignal || !policy.PreserveCompleteFailure {
		t.Fatalf("fail-closed execution policy changed: %+v", policy)
	}
}

func TestProvisioningContractRejectsVirtioSerialOverTwentyBytes(t *testing.T) {
	c, _, _ := contractFixture(t)
	if len(c.DiskSerial) != manifest.VirtioBlockSerialMaxBytes {
		t.Fatalf("fixture must exercise the exact virtio-blk serial boundary: %q", c.DiskSerial)
	}
	if err := validateContractIdentities(c); err != nil {
		t.Fatalf("exact virtio-blk serial boundary was rejected: %v", err)
	}
	c.DiskSerial += "x"
	if err := validateContractIdentities(c); err == nil {
		t.Fatal("expected overlength virtio-blk serial rejection")
	}
}

func TestProvisioningPlanIsDeterministicInertAndClosed(t *testing.T) {
	c, paths, _ := contractFixture(t)
	m := qualificationForContract(t, c)
	inspector := fakeInspector{info: fakeFileInfo{mode: 0o600, size: PinnedImageBytes}, digest: manifest.UbuntuImageSHA256}
	first, err := BuildPlan(c, paths, m, "/checkout", inspector)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildPlan(c, paths, m, "/checkout", inspector)
	if err != nil {
		t.Fatal(err)
	}
	firstJSON, _ := json.Marshal(first)
	secondJSON, _ := json.Marshal(second)
	if !slices.Equal(firstJSON, secondJSON) || first.Execute || first.LiveAuthorized || !first.AuthorizationRequired || len(first.Operations) != 14 || first.FirstBootObservation.Validate() != nil {
		t.Fatalf("provisioning plan is not deterministic and inert: %s", firstJSON)
	}
	joined := string(firstJSON)
	for _, required := range []string{"verify-source-image", "create-private-os-clone", "create-private-data-disk", "render-nocloud", "launch-qemu", "capture-first-boot-console", "controller-owned-bounded-file", "controlled-shutdown", "preserve-failure", "cleanup"} {
		if !strings.Contains(joined, required) {
			t.Fatalf("plan missing %q", required)
		}
	}
	for _, forbidden := range []string{"exec/v1", "hostfwd=", "physical-disks", "virtio-9p", "virtiofs"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("plan contains forbidden surface %q", forbidden)
		}
	}
	var clone PinnedCommand
	if err := json.Unmarshal([]byte(first.Operations[2].Inputs[0]), &clone); err != nil {
		t.Fatal(err)
	}
	wantCloneArgs := []string{"create", "-f", "qcow2", "-F", "qcow2", "-b", c.SourceImage.Path, filepath.Join(c.Roots.Instances, c.RunID, c.CohortID, "os-private.qcow2")}
	if clone.ToolID != ToolQEMUImage || clone.BinarySHA256 == "" || clone.TimeoutSeconds != 120 || !slices.Equal(clone.Args, wantCloneArgs) || first.ToolchainSHA256 != c.Toolchain.ManifestSHA256 {
		t.Fatalf("clone or toolchain binding changed: clone=%+v plan=%+v", clone, first)
	}
}

func TestProvisioningPlanRejectsOverlengthRuntimeSockets(t *testing.T) {
	c, paths, _ := contractFixture(t)
	c.RunID = "run-" + strings.Repeat("r", 60)
	c.CohortID = "cohort-" + strings.Repeat("c", 60)
	m := qualificationForContract(t, c)
	inspector := fakeInspector{info: fakeFileInfo{mode: 0o600, size: PinnedImageBytes}, digest: manifest.UbuntuImageSHA256}
	if _, err := BuildPlan(c, paths, m, "/checkout", inspector); err == nil || !strings.Contains(err.Error(), "Unix socket path") {
		t.Fatalf("expected overlength runtime socket rejection before authorization, got %v", err)
	}
}

func TestLiveAuthorizationFailsClosedAndNeverChangesExecute(t *testing.T) {
	c, paths, _ := contractFixture(t)
	plan, err := BuildPlan(c, paths, qualificationForContract(t, c), "/checkout", fakeInspector{info: fakeFileInfo{mode: 0o600, size: PinnedImageBytes}, digest: manifest.UbuntuImageSHA256})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_800_000_000, 0)
	digest, _ := c.Digest()
	base := LiveAuthorization{Schema: AuthorizationSchema, Approved: true, ContractSHA256: digest, PlanSHA256: plan.PlanSHA256, RunID: c.RunID, CohortID: c.CohortID, BootstrapNonce: c.BootstrapNonce, ExpiresAtUnix: now.Add(5 * time.Minute).Unix()}
	if _, err := AuthorizePlan(plan, c, LiveAuthorization{}, now); err == nil {
		t.Fatal("expected missing live authorization rejection")
	}
	falseAuth := base
	falseAuth.Approved = false
	if _, err := AuthorizePlan(plan, c, falseAuth, now); err == nil {
		t.Fatal("expected false live authorization rejection")
	}
	wrong := base
	wrong.ContractSHA256 = strings.Repeat("0", 64)
	if _, err := AuthorizePlan(plan, c, wrong, now); err == nil {
		t.Fatal("expected contract substitution rejection")
	}
	tampered := plan
	tampered.Operations = slices.Clone(plan.Operations)
	tampered.Operations[0].Assertions = []string{"replacement permitted"}
	if _, err := AuthorizePlan(tampered, c, base, now); err == nil {
		t.Fatal("expected exact plan digest substitution rejection")
	}
	tampered = plan
	tampered.ToolchainSHA256 = strings.Repeat("f", 64)
	if _, err := AuthorizePlan(tampered, c, base, now); err == nil {
		t.Fatal("expected toolchain digest substitution rejection")
	}
	authorized, err := AuthorizePlan(plan, c, base, now)
	if err != nil {
		t.Fatal(err)
	}
	if !authorized.LiveAuthorized || authorized.Execute {
		t.Fatalf("authorization must remain an inert reviewed plan: %+v", authorized)
	}
}

func TestLiveAuthorizationFileMustBeAbsoluteOwnerOnlyAndNonSymlink(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "authorization.json")
	data := []byte(`{"schema":"dockpipe.vm.live-authorization.v3","approved":false,"contract_sha256":"` + strings.Repeat("a", 64) + `","plan_sha256":"` + strings.Repeat("b", 64) + `","run_id":"run-001","cohort_id":"cohort-001","bootstrap_nonce":"` + strings.Repeat("c", 64) + `","expires_at_unix":0}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadAuthorization(path); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadAuthorization("relative.json"); err == nil {
		t.Fatal("expected relative authorization path rejection")
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadAuthorization(path); err == nil {
		t.Fatal("expected permissive authorization mode rejection")
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	symlink := filepath.Join(root, "authorization-link.json")
	if err := os.Symlink(path, symlink); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadAuthorization(symlink); err == nil {
		t.Fatal("expected authorization symlink rejection")
	}
}

func TestImageAndXDGValidationRejectMismatches(t *testing.T) {
	c, paths, _ := contractFixture(t)
	m := qualification(t)
	valid := fakeInspector{info: fakeFileInfo{mode: 0o600, size: PinnedImageBytes}, digest: manifest.UbuntuImageSHA256}
	c.SourceImage.Path = filepath.Join(paths.Images, "wrong.img")
	if err := c.Validate(paths, m, "/checkout"); err == nil {
		t.Fatal("expected pinned image path rejection")
	}
	c, paths, _ = contractFixture(t)
	for name, inspector := range map[string]fakeInspector{
		"symlink":   {info: fakeFileInfo{mode: os.ModeSymlink | 0o600, size: PinnedImageBytes}, digest: manifest.UbuntuImageSHA256},
		"directory": {info: fakeFileInfo{mode: os.ModeDir | 0o600, size: PinnedImageBytes}, digest: manifest.UbuntuImageSHA256},
		"mode":      {info: fakeFileInfo{mode: 0o644, size: PinnedImageBytes}, digest: manifest.UbuntuImageSHA256},
		"size":      {info: fakeFileInfo{mode: 0o600, size: PinnedImageBytes - 1}, digest: manifest.UbuntuImageSHA256},
		"hash":      {info: fakeFileInfo{mode: 0o600, size: PinnedImageBytes}, digest: strings.Repeat("0", 64)},
	} {
		t.Run(name, func(t *testing.T) {
			if err := ValidateSourceImage(c.SourceImage, inspector); err == nil {
				t.Fatal("expected image rejection")
			}
		})
	}
	_ = valid
	c, paths, _ = contractFixture(t)
	c.Roots.Instances = filepath.Join("relative", "instances")
	if err := c.Validate(paths, m, "/checkout"); err == nil {
		t.Fatal("expected relative XDG root rejection")
	}
	c, paths, _ = contractFixture(t)
	paths.Instances = "/checkout/bin/.dockpipe/internal/vm"
	c.Roots.Instances = paths.Instances
	if err := c.Validate(paths, m, "/checkout"); err == nil {
		t.Fatal("expected checkout/generated-store rejection")
	}
	c, paths, _ = contractFixture(t)
	paths.Config = filepath.Join(paths.Instances, "nested-config")
	c.Roots.Config = paths.Config
	if err := c.Validate(paths, m, "/checkout"); err == nil {
		t.Fatal("expected overlapping generated-root rejection")
	}
}

func TestPinnedBinaryRejectsTypeAndChecksumSubstitution(t *testing.T) {
	c, _, _ := contractFixture(t)
	if err := ValidatePinnedBinary(c.Artifacts.ControllerBinary, c.Artifacts.ControllerBinarySHA256); err != nil {
		t.Fatal(err)
	}
	if err := ValidatePinnedBinary(c.Artifacts.ControllerBinary, strings.Repeat("0", 64)); err == nil {
		t.Fatal("expected binary checksum rejection")
	}
	symlink := filepath.Join(t.TempDir(), "controller")
	if err := os.Symlink(c.Artifacts.ControllerBinary, symlink); err != nil {
		t.Fatal(err)
	}
	if err := ValidatePinnedBinary(symlink, c.Artifacts.ControllerBinarySHA256); err == nil {
		t.Fatal("expected binary symlink rejection")
	}
}

func TestToolchainIsExactTaskOwnedAndHasNoFallbackSurface(t *testing.T) {
	c, _, _ := contractFixture(t)
	toolchain, err := LoadToolchain(c.Toolchain)
	if err != nil {
		t.Fatal(err)
	}
	if err := toolchain.Validate(c.Toolchain, "/checkout", c.Roots); err != nil {
		t.Fatal(err)
	}
	if toolchain.QEMUVersion != "11.0.3" || len(toolchain.Tools) != 2 || len(toolchain.RuntimeFiles) == 0 {
		t.Fatalf("unexpected toolchain closure: %+v", toolchain)
	}
	encoded, _ := json.Marshal(toolchain)
	for _, forbidden := range []string{"$PATH", "cloud-localds", "xorriso", "genisoimage", "ssh", "network", "fallback"} {
		if strings.Contains(strings.ToLower(string(encoded)), strings.ToLower(forbidden)) {
			t.Fatalf("toolchain exposes forbidden fallback surface %q", forbidden)
		}
	}
}

func TestToolchainRejectsExtraFilesHashesModesSymlinksAndRootOverlap(t *testing.T) {
	t.Run("extra file", func(t *testing.T) {
		c, _, _ := contractFixture(t)
		toolchain, _ := LoadToolchain(c.Toolchain)
		if err := os.Chmod(c.Toolchain.Root, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(c.Toolchain.Root, "unexpected"), []byte("not authorized"), 0o400); err != nil {
			t.Fatal(err)
		}
		if err := toolchain.Validate(c.Toolchain, "/checkout", c.Roots); err == nil {
			t.Fatal("expected unmanifested file rejection")
		}
	})
	t.Run("hash", func(t *testing.T) {
		c, _, _ := contractFixture(t)
		toolchain, _ := LoadToolchain(c.Toolchain)
		toolchain.Tools[0].SHA256 = strings.Repeat("0", 64)
		if err := toolchain.Validate(c.Toolchain, "/checkout", c.Roots); err == nil {
			t.Fatal("expected tool hash substitution rejection")
		}
	})
	t.Run("mode", func(t *testing.T) {
		c, _, _ := contractFixture(t)
		toolchain, _ := LoadToolchain(c.Toolchain)
		path := filepath.Join(c.Toolchain.Root, filepath.FromSlash(toolchain.Tools[0].RelativePath))
		if err := os.Chmod(path, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := toolchain.Validate(c.Toolchain, "/checkout", c.Roots); err == nil {
			t.Fatal("expected executable mode substitution rejection")
		}
	})
	t.Run("symlink", func(t *testing.T) {
		c, _, _ := contractFixture(t)
		toolchain, _ := LoadToolchain(c.Toolchain)
		path := filepath.Join(c.Toolchain.Root, filepath.FromSlash(toolchain.RuntimeFiles[0].RelativePath))
		backup := path + ".real"
		if err := os.Chmod(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(path, backup); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(backup, path); err != nil {
			t.Fatal(err)
		}
		if err := toolchain.Validate(c.Toolchain, "/checkout", c.Roots); err == nil {
			t.Fatal("expected runtime symlink substitution rejection")
		}
	})
	t.Run("generated root overlap", func(t *testing.T) {
		c, _, _ := contractFixture(t)
		toolchain, _ := LoadToolchain(c.Toolchain)
		c.Roots.Instances = filepath.Dir(c.Toolchain.Root)
		if err := toolchain.Validate(c.Toolchain, "/checkout", c.Roots); err == nil {
			t.Fatal("expected toolchain and generated root overlap rejection")
		}
	})
}

func TestExclusiveIdentityCreationNeverReplaces(t *testing.T) {
	c, _, material := contractFixture(t)
	root := filepath.Join(t.TempDir(), "identity")
	reserved, err := ReserveIdentity(root, c, material.Keys)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadReservedKeyMaterial(root, c)
	if err != nil || !bytes.Equal(loaded.ControllerPrivate, material.Keys.ControllerPrivate) || !bytes.Equal(loaded.GuestPublic, material.Keys.GuestPublic) {
		t.Fatalf("reload exact reserved identity: err=%v", err)
	}
	before, err := os.ReadFile(reserved.ControllerPrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReserveIdentity(root, c, material.Keys); err == nil {
		t.Fatal("expected exclusive identity root rejection")
	}
	after, _ := os.ReadFile(reserved.ControllerPrivateKey)
	if !slices.Equal(before, after) {
		t.Fatal("existing private key was replaced")
	}
	for _, path := range []string{reserved.ControllerPrivateKey, reserved.GuestPrivateKey, reserved.BootstrapNoncePath} {
		info, _ := os.Stat(path)
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("private identity mode changed for %s", path)
		}
	}
	other, err := GenerateKeyMaterial()
	if err != nil {
		t.Fatal(err)
	}
	mismatchRoot := filepath.Join(t.TempDir(), "mismatched-identity")
	if _, err := ReserveIdentity(mismatchRoot, c, other); err == nil {
		t.Fatal("expected pre-authorization public-key pin mismatch rejection")
	}
	if _, err := os.Lstat(mismatchRoot); !os.IsNotExist(err) {
		t.Fatal("mismatched key material created an identity root")
	}
}

func TestReservedIdentityRecordIsStrictAndOwnerOnly(t *testing.T) {
	t.Run("widened mode", func(t *testing.T) {
		c, _, material := contractFixture(t)
		root := filepath.Join(t.TempDir(), "identity")
		if _, err := ReserveIdentity(root, c, material.Keys); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(filepath.Join(root, "identity.json"), 0o640); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadReservedKeyMaterial(root, c); err == nil {
			t.Fatal("expected widened identity-record mode rejection")
		}
	})
	t.Run("unknown field", func(t *testing.T) {
		c, _, material := contractFixture(t)
		root := filepath.Join(t.TempDir(), "identity")
		if _, err := ReserveIdentity(root, c, material.Keys); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, "identity.json")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var record map[string]any
		if err := json.Unmarshal(data, &record); err != nil {
			t.Fatal(err)
		}
		record["unexpected"] = true
		data, _ = json.Marshal(record)
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadReservedKeyMaterial(root, c); err == nil {
			t.Fatal("expected unknown identity-record field rejection")
		}
	})
}

func TestNoCloudRenderingIsExactPinnedAndRestricted(t *testing.T) {
	c, _, material := contractFixture(t)
	first, err := RenderNoCloud(c, qualification(t), material)
	if err != nil {
		t.Fatal(err)
	}
	second, err := RenderNoCloud(c, qualification(t), material)
	if err != nil {
		t.Fatal(err)
	}
	firstJSON, _ := json.Marshal(first)
	secondJSON, _ := json.Marshal(second)
	if !slices.Equal(firstJSON, secondJSON) || len(first) != 3 {
		t.Fatal("NoCloud rendering is not deterministic")
	}
	var all strings.Builder
	for _, file := range first {
		all.WriteString(file.Name)
		all.Write(file.Content)
	}
	rendered := all.String()
	for _, required := range []string{"network-config", "package_update: false", "package_upgrade: false", "ssh_pwauth: false", "system: true", "shell: /usr/sbin/nologin", "lock_passwd: true", "ethernets: {}", c.DiskSerial, c.FilesystemUUID, "dockpipe-agent.service", "/etc/udev/rules.d/99-dockpipe-agent.rules", "/usr/libexec/dockpipe-guest-agent", "/usr/libexec/dockpipe-sqlite-vm-harness", "[/usr/bin/chgrp, --dereference, dockpipe-agent, /dev/virtio-ports/org.dockpipe.agent.1]", "[/usr/bin/chmod, \"0660\", /dev/virtio-ports/org.dockpipe.agent.1]"} {
		if !strings.Contains(rendered, required) {
			t.Fatalf("rendered seed missing %q", required)
		}
	}
	if strings.Count(rendered, "    defer: true\n") != 3 {
		t.Fatalf("agent-owned NoCloud files must be deferred until after user creation")
	}
	if strings.Contains(rendered, "ssh_redirect_user:") {
		t.Fatalf("system user must not declare cloud-init ssh_redirect_user")
	}
	if strings.Contains(rendered, "ssh_authorized_keys: []") {
		t.Fatalf("rendered NoCloud config contains schema-invalid empty SSH key list")
	}
	var renderedConfig AgentConfig
	renderedUdevRule := ""
	for _, line := range strings.Split(rendered, "\n") {
		encoded, ok := strings.CutPrefix(strings.TrimSpace(line), "content: ")
		if !ok {
			continue
		}
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err == nil {
			if strings.Contains(string(decoded), `ATTR{name}=="org.dockpipe.agent.1"`) {
				renderedUdevRule = string(decoded)
			}
			var candidate AgentConfig
			if json.Unmarshal(decoded, &candidate) == nil && candidate.Schema == "dockpipe.vm.guest-agent-config.v3" {
				renderedConfig = candidate
			}
		}
	}
	if renderedConfig.BootstrapNonce != c.BootstrapNonce || renderedConfig.BootIDSource != manifest.KernelBootIDSource {
		t.Fatalf("rendered guest config does not bind bootstrap nonce and kernel boot-ID source: %+v", renderedConfig)
	}
	if renderedUdevRule != "SUBSYSTEM==\"virtio-ports\", ATTR{name}==\"org.dockpipe.agent.1\", GROUP=\"dockpipe-agent\", MODE=\"0660\"\n" {
		t.Fatalf("rendered virtio-port ownership rule changed: %q", renderedUdevRule)
	}
	bad := material
	bad.GuestAgentBinary = []byte("substituted")
	if _, err := RenderNoCloud(c, qualification(t), bad); err == nil {
		t.Fatal("expected binary pin rejection")
	}
	bad = material
	bad.Keys.ControllerPublic = slices.Clone(material.Keys.GuestPublic)
	if _, err := RenderNoCloud(c, qualification(t), bad); err == nil {
		t.Fatal("expected public-key pin rejection")
	}
	mutatedRoot := filepath.Join(t.TempDir(), "assets")
	for relative := range reviewedAssetSHA256 {
		source := filepath.Join(c.Artifacts.AssetsRoot, filepath.FromSlash(relative))
		target := filepath.Join(mutatedRoot, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}
		if relative == "systemd/dockpipe-agent.service" {
			data = append(data, []byte("ExecStartPost=/bin/sh\n")...)
		}
		if err := os.WriteFile(target, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	c.Artifacts.AssetsRoot = mutatedRoot
	if _, err := RenderNoCloud(c, qualification(t), material); err == nil {
		t.Fatal("expected substituted reviewed-asset rejection")
	}
	c, _, material = contractFixture(t)
	c.RunID = "run-001\nwrite_files:"
	if _, err := RenderNoCloud(c, qualification(t), material); err == nil {
		t.Fatal("expected injectable render identity rejection")
	}
}
