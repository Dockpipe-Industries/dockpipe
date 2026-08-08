package provisioning

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"dockpipe.vm/tools/internal/manifest"
	"dockpipe.vm/tools/internal/xdg"
)

const (
	Schema              = "dockpipe.vm.provisioning.v3"
	PlanSchema          = "dockpipe.vm.provisioning-plan.v3"
	AuthorizationSchema = "dockpipe.vm.live-authorization.v3"
	PinnedImageFilename = "ubuntu-24.04-server-cloudimg-amd64-20260801.img"
	PinnedImageBytes    = int64(624239616)
	SeedLabel           = "cidata"
)

var (
	idPattern     = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{2,127}$`)
	uuidPattern   = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	serialPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{7,63}$`)
	noncePattern  = regexp.MustCompile(`^[0-9a-f]{64}$`)
	shaPattern    = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

type Contract struct {
	Schema         string             `json:"schema"`
	Purpose        string             `json:"purpose"`
	Disposable     bool               `json:"disposable"`
	InstanceCount  int                `json:"instance_count"`
	RunID          string             `json:"run_id"`
	CohortID       string             `json:"cohort_id"`
	MachineUUID    string             `json:"machine_uuid"`
	DiskSerial     string             `json:"disk_serial"`
	FilesystemUUID string             `json:"filesystem_uuid"`
	BootstrapNonce string             `json:"bootstrap_nonce"`
	SourceImage    SourceImage        `json:"source_image"`
	Toolchain      ToolchainReference `json:"toolchain"`
	Roots          Roots              `json:"roots"`
	Artifacts      Artifacts          `json:"artifacts"`
	Execution      ExecutionPolicy    `json:"execution"`
}

type ExecutionPolicy struct {
	CloneTimeoutSeconds             int  `json:"clone_timeout_seconds"`
	LaunchTimeoutSeconds            int  `json:"launch_timeout_seconds"`
	GuestVerificationTimeoutSeconds int  `json:"guest_verification_timeout_seconds"`
	ShutdownTimeoutSeconds          int  `json:"shutdown_timeout_seconds"`
	AutomaticRetry                  bool `json:"automatic_retry"`
	AutomaticCleanup                bool `json:"automatic_cleanup"`
	FallbackSignal                  bool `json:"fallback_signal"`
	PreserveCompleteFailure         bool `json:"preserve_complete_failure"`
}

func RequiredExecutionPolicy() ExecutionPolicy {
	return ExecutionPolicy{
		CloneTimeoutSeconds: 120, LaunchTimeoutSeconds: 120,
		GuestVerificationTimeoutSeconds: 180, ShutdownTimeoutSeconds: 120,
		PreserveCompleteFailure: true,
	}
}

type SourceImage struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

type Roots struct {
	Instances string `json:"instances"`
	Evidence  string `json:"evidence"`
	Config    string `json:"config"`
	Runtime   string `json:"runtime"`
}

type Artifacts struct {
	AssetsRoot                string `json:"assets_root"`
	ControllerBinary          string `json:"controller_binary"`
	ControllerBinarySHA256    string `json:"controller_binary_sha256"`
	GuestAgentBinary          string `json:"guest_agent_binary"`
	GuestAgentBinarySHA256    string `json:"guest_agent_binary_sha256"`
	ControllerPublicKeySHA256 string `json:"controller_public_key_sha256"`
	GuestPublicKeySHA256      string `json:"guest_public_key_sha256"`
}

type LiveAuthorization struct {
	Schema         string `json:"schema"`
	Approved       bool   `json:"approved"`
	ContractSHA256 string `json:"contract_sha256"`
	PlanSHA256     string `json:"plan_sha256"`
	RunID          string `json:"run_id"`
	CohortID       string `json:"cohort_id"`
	BootstrapNonce string `json:"bootstrap_nonce"`
	ExpiresAtUnix  int64  `json:"expires_at_unix"`
}

func Load(path string) (Contract, error) {
	var c Contract
	if err := decodeStrictFile(path, &c); err != nil {
		return c, err
	}
	return c, nil
}

func LoadAuthorization(path string) (LiveAuthorization, error) {
	var auth LiveAuthorization
	if strings.TrimSpace(path) == "" {
		return auth, fmt.Errorf("a distinct live authorization file is required")
	}
	if !filepath.IsAbs(path) {
		return auth, fmt.Errorf("live authorization path must be absolute")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return auth, fmt.Errorf("inspect live authorization: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		return auth, fmt.Errorf("live authorization must be a regular non-symlink owner-only file")
	}
	if stat, ok := info.Sys().(*syscall.Stat_t); ok && int(stat.Uid) != os.Geteuid() {
		return auth, fmt.Errorf("live authorization must be owned by the current user")
	}
	if err := decodeStrictFile(path, &auth); err != nil {
		return auth, err
	}
	return auth, nil
}

func decodeStrictFile(path string, out any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("decode %s: %w", filepath.Base(path), err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return fmt.Errorf("%s contains trailing JSON", filepath.Base(path))
	}
	return nil
}

func (c Contract) Validate(paths xdg.Paths, qualification manifest.Manifest, checkoutRoot string) error {
	if err := validateContractIdentities(c); err != nil {
		return err
	}
	if c.MachineUUID != qualification.MachineUUID || c.RunID != qualification.RunID || c.DiskSerial != qualification.DataDisk.Serial || c.FilesystemUUID != qualification.Filesystem.UUID {
		return fmt.Errorf("provisioning identities must exactly match the qualification manifest")
	}
	wantRoots := Roots{Instances: paths.Instances, Evidence: paths.Evidence, Config: paths.Config, Runtime: paths.Runtime}
	if c.Roots != wantRoots {
		return fmt.Errorf("instance, evidence, configuration, and runtime roots must come from the package XDG model")
	}
	if err := validateRoots(c.Roots, checkoutRoot); err != nil {
		return err
	}
	wantImage := filepath.Join(paths.Images, PinnedImageFilename)
	if c.SourceImage.Path != wantImage || c.SourceImage.SHA256 != manifest.UbuntuImageSHA256 || c.SourceImage.Bytes != PinnedImageBytes {
		return fmt.Errorf("source image must be the exact pinned XDG cache artifact")
	}
	if !filepath.IsAbs(c.Toolchain.Root) || !filepath.IsAbs(c.Toolchain.Manifest) || filepath.Clean(c.Toolchain.Manifest) != filepath.Join(filepath.Clean(c.Toolchain.Root), ToolchainManifestName) || !shaPattern.MatchString(c.Toolchain.ManifestSHA256) {
		return fmt.Errorf("an exact absolute task-owned toolchain root and manifest SHA-256 are required")
	}
	if c.Execution != RequiredExecutionPolicy() {
		return fmt.Errorf("execution timeouts and fail-closed preservation policy differ from the reviewed tuple")
	}
	for label, value := range map[string]string{
		"assets root":        c.Artifacts.AssetsRoot,
		"controller binary":  c.Artifacts.ControllerBinary,
		"guest-agent binary": c.Artifacts.GuestAgentBinary,
	} {
		if !filepath.IsAbs(value) {
			return fmt.Errorf("%s must be absolute", label)
		}
	}
	for _, sum := range []string{c.Artifacts.ControllerBinarySHA256, c.Artifacts.GuestAgentBinarySHA256, c.Artifacts.ControllerPublicKeySHA256, c.Artifacts.GuestPublicKeySHA256} {
		if !shaPattern.MatchString(sum) {
			return fmt.Errorf("binary and mutually pinned public-key SHA-256 values are required")
		}
	}
	return nil
}

func validateContractIdentities(c Contract) error {
	if c.Schema != Schema || c.Purpose != manifest.QualificationPurpose || !c.Disposable || c.InstanceCount != 1 {
		return fmt.Errorf("provisioning is restricted to exactly one disposable qualification instance")
	}
	if !idPattern.MatchString(c.RunID) || !idPattern.MatchString(c.CohortID) || !uuidPattern.MatchString(c.MachineUUID) || !serialPattern.MatchString(c.DiskSerial) || !uuidPattern.MatchString(c.FilesystemUUID) || !noncePattern.MatchString(c.BootstrapNonce) {
		return fmt.Errorf("fresh run, cohort, machine, disk, filesystem, and bootstrap nonce identities are required")
	}
	return nil
}

func validateRoots(roots Roots, checkoutRoot string) error {
	values := []string{roots.Instances, roots.Evidence, roots.Config, roots.Runtime}
	seen := map[string]struct{}{}
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		if !filepath.IsAbs(value) {
			return fmt.Errorf("all generated roots must be absolute")
		}
		clean := filepath.Clean(value)
		if _, exists := seen[clean]; exists {
			return fmt.Errorf("generated roots must be distinct")
		}
		seen[clean] = struct{}{}
		if containsPathSegment(clean, ".dockpipe") || containsPathSegment(clean, ".dorkpipe") {
			return fmt.Errorf("checkout and generated-store roots are prohibited")
		}
		if checkoutRoot != "" && pathWithin(clean, checkoutRoot) {
			return fmt.Errorf("generated roots must not be inside the checkout")
		}
		cleaned = append(cleaned, clean)
	}
	for i := range cleaned {
		for j := i + 1; j < len(cleaned); j++ {
			if pathWithin(cleaned[i], cleaned[j]) || pathWithin(cleaned[j], cleaned[i]) {
				return fmt.Errorf("generated roots must not overlap")
			}
		}
	}
	return nil
}

func containsPathSegment(path, segment string) bool {
	for _, part := range strings.Split(filepath.Clean(path), string(filepath.Separator)) {
		if part == segment {
			return true
		}
	}
	return false
}

func pathWithin(path, root string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	return err == nil && rel != ".." && !filepath.IsAbs(rel) && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func (c Contract) Digest() (string, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func (a LiveAuthorization) Validate(c Contract, plan Plan, now time.Time) error {
	contractDigest, err := c.Digest()
	if err != nil {
		return err
	}
	if a.Schema != AuthorizationSchema || !a.Approved {
		return fmt.Errorf("live execution authorization is missing or false")
	}
	planDigest, err := plan.Digest()
	if err != nil {
		return err
	}
	if a.ContractSHA256 != contractDigest || a.PlanSHA256 != planDigest || plan.PlanSHA256 != planDigest || a.RunID != c.RunID || a.CohortID != c.CohortID || a.BootstrapNonce != c.BootstrapNonce {
		return fmt.Errorf("live authorization does not match the exact provisioning contract and plan")
	}
	if a.ExpiresAtUnix <= now.Unix() || a.ExpiresAtUnix > now.Add(15*time.Minute).Unix() {
		return fmt.Errorf("live authorization must be fresh and expire within 15 minutes")
	}
	return nil
}

type ImageInspector interface {
	Lstat(path string) (os.FileInfo, error)
	SHA256(path string) (string, error)
}

type OSImageInspector struct{}

func (OSImageInspector) Lstat(path string) (os.FileInfo, error) { return os.Lstat(path) }
func (OSImageInspector) SHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func ValidateSourceImage(source SourceImage, inspector ImageInspector) error {
	info, err := inspector.Lstat(source.Path)
	if err != nil {
		return fmt.Errorf("inspect source image: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() != source.Bytes || info.Mode().Perm() != 0o600 {
		return fmt.Errorf("source image must be a regular non-symlink owner-only file with the pinned byte size")
	}
	if stat, ok := info.Sys().(*syscall.Stat_t); ok && int(stat.Uid) != os.Geteuid() {
		return fmt.Errorf("source image must be owned by the current user")
	}
	digest, err := inspector.SHA256(source.Path)
	if err != nil {
		return fmt.Errorf("hash source image: %w", err)
	}
	if digest != source.SHA256 {
		return fmt.Errorf("source image SHA-256 mismatch")
	}
	return nil
}

func ValidatePinnedBinary(path, wantSHA256 string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect pinned binary: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Mode().Perm()&0o500 != 0o500 {
		return fmt.Errorf("pinned binary must be a regular non-symlink executable at its exact path")
	}
	digest, err := (OSImageInspector{}).SHA256(path)
	if err != nil {
		return fmt.Errorf("hash pinned binary: %w", err)
	}
	if digest != wantSHA256 {
		return fmt.Errorf("pinned binary SHA-256 mismatch")
	}
	return nil
}
