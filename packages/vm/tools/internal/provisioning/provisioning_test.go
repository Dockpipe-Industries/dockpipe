package provisioning

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
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
	paths := xdg.Paths{
		Images: filepath.Join(root, "cache", "dockpipe", "vm", "images"), Instances: filepath.Join(root, "state", "dockpipe", "vm", "instances"),
		Evidence: filepath.Join(root, "state", "dockpipe", "evidence"), Config: filepath.Join(root, "config", "dockpipe", "vm"), Runtime: filepath.Join(root, "run", "dockpipe", "vm"),
	}
	controllerPub, controllerPriv, _ := ed25519.GenerateKey(rand.Reader)
	guestPub, guestPriv, _ := ed25519.GenerateKey(rand.Reader)
	controllerBinary := []byte("controller-test-binary")
	guestBinary := []byte("guest-test-binary")
	buildRoot := filepath.Join(root, "build")
	if err := os.MkdirAll(buildRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	controllerPath := filepath.Join(buildRoot, "controller")
	guestPath := filepath.Join(buildRoot, "guest")
	if err := os.WriteFile(controllerPath, controllerBinary, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(guestPath, guestBinary, 0o700); err != nil {
		t.Fatal(err)
	}
	assetsRoot, err := filepath.Abs(filepath.Join("..", "..", "..", "workflows", "linux-vm", "assets"))
	if err != nil {
		t.Fatal(err)
	}
	m := qualification(t)
	c := Contract{
		Schema: Schema, Purpose: "qualification", Disposable: true, InstanceCount: 1,
		RunID: m.RunID, CohortID: "cohort-001", MachineUUID: m.MachineUUID, DiskSerial: m.DataDisk.Serial, FilesystemUUID: m.Filesystem.UUID, Nonce: strings.Repeat("1", 64),
		SourceImage: SourceImage{Path: filepath.Join(paths.Images, PinnedImageFilename), SHA256: manifest.UbuntuImageSHA256, Bytes: PinnedImageBytes},
		Roots:       Roots{Instances: paths.Instances, Evidence: paths.Evidence, Config: paths.Config, Runtime: paths.Runtime},
		Artifacts:   Artifacts{AssetsRoot: assetsRoot, ControllerBinary: controllerPath, ControllerBinarySHA256: digest(controllerBinary), GuestAgentBinary: guestPath, GuestAgentBinarySHA256: digest(guestBinary), ControllerPublicKeySHA256: digest(controllerPub), GuestPublicKeySHA256: digest(guestPub)},
	}
	return c, paths, RenderMaterial{Keys: KeyMaterial{ControllerPublic: controllerPub, ControllerPrivate: controllerPriv, GuestPublic: guestPub, GuestPrivate: guestPriv}, ControllerBinary: controllerBinary, GuestAgentBinary: guestBinary}
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func TestProvisioningPlanIsDeterministicInertAndClosed(t *testing.T) {
	c, paths, _ := contractFixture(t)
	m := qualification(t)
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
	if !slices.Equal(firstJSON, secondJSON) || first.Execute || first.LiveAuthorized || !first.AuthorizationRequired || len(first.Operations) != 13 {
		t.Fatalf("provisioning plan is not deterministic and inert: %s", firstJSON)
	}
	joined := string(firstJSON)
	for _, required := range []string{"verify-source-image", "create-private-os-clone", "create-private-data-disk", "render-nocloud", "launch-qemu", "controlled-shutdown", "preserve-failure", "cleanup"} {
		if !strings.Contains(joined, required) {
			t.Fatalf("plan missing %q", required)
		}
	}
	for _, forbidden := range []string{"exec/v1", "hostfwd=", "physical-disks", "virtio-9p", "virtiofs"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("plan contains forbidden surface %q", forbidden)
		}
	}
}

func TestLiveAuthorizationFailsClosedAndNeverChangesExecute(t *testing.T) {
	c, paths, _ := contractFixture(t)
	plan, err := BuildPlan(c, paths, qualification(t), "/checkout", fakeInspector{info: fakeFileInfo{mode: 0o600, size: PinnedImageBytes}, digest: manifest.UbuntuImageSHA256})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_800_000_000, 0)
	digest, _ := c.Digest()
	base := LiveAuthorization{Schema: AuthorizationSchema, Approved: true, ContractSHA256: digest, PlanSHA256: plan.PlanSHA256, RunID: c.RunID, CohortID: c.CohortID, Nonce: c.Nonce, ExpiresAtUnix: now.Add(5 * time.Minute).Unix()}
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
	data := []byte(`{"schema":"dockpipe.vm.live-authorization.v1","approved":false,"contract_sha256":"` + strings.Repeat("a", 64) + `","plan_sha256":"` + strings.Repeat("b", 64) + `","run_id":"run-001","cohort_id":"cohort-001","nonce":"` + strings.Repeat("c", 64) + `","expires_at_unix":0}`)
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

func TestExclusiveIdentityCreationNeverReplaces(t *testing.T) {
	c, _, material := contractFixture(t)
	root := filepath.Join(t.TempDir(), "identity")
	reserved, err := ReserveIdentity(root, c, material.Keys)
	if err != nil {
		t.Fatal(err)
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
	for _, path := range []string{reserved.ControllerPrivateKey, reserved.GuestPrivateKey, reserved.NoncePath} {
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
	for _, required := range []string{"network-config", "package_update: false", "package_upgrade: false", "packages: []", "ssh_pwauth: false", "config: disabled", c.DiskSerial, c.FilesystemUUID, "dockpipe-agent.service", "/usr/libexec/dockpipe-guest-agent"} {
		if !strings.Contains(rendered, required) {
			t.Fatalf("rendered seed missing %q", required)
		}
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
