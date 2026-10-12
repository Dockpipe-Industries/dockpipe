package identitymaterial

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
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

	"dockpipe.vm/tools/internal/provisioning"
)

const Schema = "dockpipe.vm.identity-material.v1"
const MaxLifetime = 24 * time.Hour

var fileNames = []string{"controller.pub", "controller.key", "guest.pub", "guest.key", "material.json"}
var idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{2,127}$`)

type Descriptor struct {
	Schema                    string `json:"schema"`
	RunID                     string `json:"run_id"`
	CohortID                  string `json:"cohort_id"`
	BootstrapNonce            string `json:"bootstrap_nonce"`
	ControllerPublicKeySHA256 string `json:"controller_public_key_sha256"`
	GuestPublicKeySHA256      string `json:"guest_public_key_sha256"`
	CreatedAtUnix             int64  `json:"created_at_unix"`
	ExpiresAtUnix             int64  `json:"expires_at_unix"`
}

// Prepare generates fresh identities in memory before exclusively creating an
// owner-only, durable staging bundle. The bundle is outside the checkout and
// every live VM root so review and live authorization may happen in a later
// process without weakening the key pins.
func Prepare(root, checkoutRoot, runID, cohortID string, liveRoots provisioning.Roots, now time.Time) (Descriptor, error) {
	var out Descriptor
	root = filepath.Clean(root)
	if !filepath.IsAbs(root) || !idPattern.MatchString(runID) || !idPattern.MatchString(cohortID) {
		return out, fmt.Errorf("identity-material root, run ID, and cohort ID must be exact and absolute")
	}
	if err := validateLocation(root, checkoutRoot, liveRoots); err != nil {
		return out, err
	}
	parent, err := os.Lstat(filepath.Dir(root))
	if err != nil || !parent.IsDir() || parent.Mode()&os.ModeSymlink != 0 || parent.Mode().Perm()&0o077 != 0 || !owned(parent) {
		return out, fmt.Errorf("identity-material parent must be an existing current-user-owned private directory")
	}
	keys, err := provisioning.GenerateKeyMaterial()
	if err != nil {
		return out, err
	}
	nonceBytes := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, nonceBytes); err != nil {
		return out, err
	}
	out = Descriptor{Schema: Schema, RunID: runID, CohortID: cohortID, BootstrapNonce: hex.EncodeToString(nonceBytes), ControllerPublicKeySHA256: digest(keys.ControllerPublic), GuestPublicKeySHA256: digest(keys.GuestPublic), CreatedAtUnix: now.Unix(), ExpiresAtUnix: now.Add(MaxLifetime).Unix()}
	if err := os.Mkdir(root, 0o700); err != nil {
		return Descriptor{}, fmt.Errorf("exclusively create identity-material root: %w", err)
	}
	complete := false
	defer func() {
		if !complete {
			for _, name := range fileNames {
				_ = os.Remove(filepath.Join(root, name))
			}
			_ = os.Remove(root)
		}
	}()
	manifest, err := json.Marshal(out)
	if err != nil {
		return Descriptor{}, err
	}
	files := []struct {
		name string
		data []byte
	}{
		{"controller.pub", keys.ControllerPublic}, {"controller.key", keys.ControllerPrivate},
		{"guest.pub", keys.GuestPublic}, {"guest.key", keys.GuestPrivate}, {"material.json", manifest},
	}
	for _, file := range files {
		if err := writeExclusive(filepath.Join(root, file.name), file.data); err != nil {
			return Descriptor{}, err
		}
	}
	if err := syncDir(root); err != nil {
		return Descriptor{}, err
	}
	if err := syncDir(filepath.Dir(root)); err != nil {
		return Descriptor{}, err
	}
	complete = true
	return out, nil
}

func Load(root, checkoutRoot string, c provisioning.Contract, now time.Time) (Descriptor, provisioning.KeyMaterial, error) {
	var descriptor Descriptor
	var keys provisioning.KeyMaterial
	root = filepath.Clean(root)
	if err := validateLocation(root, checkoutRoot, c.Roots); err != nil {
		return descriptor, keys, err
	}
	info, err := os.Lstat(root)
	if err != nil || !filepath.IsAbs(root) || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0o700 || !owned(info) {
		return descriptor, keys, fmt.Errorf("identity-material root must be an exact owner-only non-symlink directory")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != len(fileNames) {
		return descriptor, keys, fmt.Errorf("identity-material inventory is not exact")
	}
	want := map[string]bool{}
	for _, name := range fileNames {
		want[name] = true
	}
	contents := map[string][]byte{}
	for _, entry := range entries {
		if !want[entry.Name()] {
			return descriptor, keys, fmt.Errorf("unexpected identity-material file %q", entry.Name())
		}
		path := filepath.Join(root, entry.Name())
		fileInfo, err := os.Lstat(path)
		if err != nil || !fileInfo.Mode().IsRegular() || fileInfo.Mode()&os.ModeSymlink != 0 || fileInfo.Mode().Perm() != 0o600 || !owned(fileInfo) {
			return descriptor, keys, fmt.Errorf("identity-material file %q is not owner-only and regular", entry.Name())
		}
		contents[entry.Name()], err = os.ReadFile(path)
		if err != nil {
			return descriptor, keys, err
		}
	}
	dec := json.NewDecoder(bytes.NewReader(contents["material.json"]))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&descriptor); err != nil {
		return descriptor, keys, fmt.Errorf("decode identity material: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return descriptor, keys, fmt.Errorf("identity material contains trailing JSON")
	}
	keys = provisioning.KeyMaterial{ControllerPublic: contents["controller.pub"], ControllerPrivate: contents["controller.key"], GuestPublic: contents["guest.pub"], GuestPrivate: contents["guest.key"]}
	if descriptor.Schema != Schema || descriptor.RunID != c.RunID || descriptor.CohortID != c.CohortID || descriptor.BootstrapNonce != c.BootstrapNonce || descriptor.ControllerPublicKeySHA256 != c.Artifacts.ControllerPublicKeySHA256 || descriptor.GuestPublicKeySHA256 != c.Artifacts.GuestPublicKeySHA256 || descriptor.ExpiresAtUnix-descriptor.CreatedAtUnix != int64(MaxLifetime/time.Second) || now.Unix() < descriptor.CreatedAtUnix-30 || now.Unix() >= descriptor.ExpiresAtUnix || len(keys.ControllerPublic) != ed25519.PublicKeySize || len(keys.ControllerPrivate) != ed25519.PrivateKeySize || len(keys.GuestPublic) != ed25519.PublicKeySize || len(keys.GuestPrivate) != ed25519.PrivateKeySize || !bytes.Equal(keys.ControllerPrivate[32:], keys.ControllerPublic) || !bytes.Equal(keys.GuestPrivate[32:], keys.GuestPublic) || digest(keys.ControllerPublic) != descriptor.ControllerPublicKeySHA256 || digest(keys.GuestPublic) != descriptor.GuestPublicKeySHA256 {
		return Descriptor{}, provisioning.KeyMaterial{}, fmt.Errorf("identity material does not match the exact authorized contract")
	}
	return descriptor, keys, nil
}

// Consume removes only the validated staging inventory after the same keys
// have been durably reserved at destination.
func Consume(root string, descriptor Descriptor, destination provisioning.ReservedIdentity) error {
	controller, err := os.ReadFile(destination.ControllerPublicKey)
	if err != nil || digest(controller) != descriptor.ControllerPublicKeySHA256 {
		return fmt.Errorf("reserved controller identity is not durable and exact")
	}
	guest, err := os.ReadFile(destination.GuestPublicKey)
	if err != nil || digest(guest) != descriptor.GuestPublicKeySHA256 {
		return fmt.Errorf("reserved guest identity is not durable and exact")
	}
	controllerPrivate, err := os.ReadFile(destination.ControllerPrivateKey)
	if err != nil || len(controllerPrivate) != ed25519.PrivateKeySize || !bytes.Equal(controllerPrivate[32:], controller) {
		return fmt.Errorf("reserved controller private identity is not durable and exact")
	}
	guestPrivate, err := os.ReadFile(destination.GuestPrivateKey)
	if err != nil || len(guestPrivate) != ed25519.PrivateKeySize || !bytes.Equal(guestPrivate[32:], guest) {
		return fmt.Errorf("reserved guest private identity is not durable and exact")
	}
	nonce, err := os.ReadFile(destination.BootstrapNoncePath)
	if err != nil || string(nonce) != descriptor.BootstrapNonce {
		return fmt.Errorf("reserved bootstrap nonce is not durable and exact")
	}
	for _, name := range fileNames {
		if err := os.Remove(filepath.Join(root, name)); err != nil {
			return fmt.Errorf("consume identity-material file %s: %w", name, err)
		}
	}
	if err := syncDir(root); err != nil {
		return err
	}
	if err := os.Remove(root); err != nil {
		return err
	}
	return syncDir(filepath.Dir(root))
}

func writeExclusive(path string, data []byte) error {
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
	return err
}

func syncDir(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}
func digest(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func owned(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return !ok || int(stat.Uid) == os.Geteuid()
}
func within(path, root string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	return err == nil && rel != ".." && !filepath.IsAbs(rel) && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func validateLocation(root, checkoutRoot string, liveRoots provisioning.Roots) error {
	for _, forbidden := range []string{checkoutRoot, liveRoots.Instances, liveRoots.Evidence, liveRoots.Config, liveRoots.Runtime} {
		if forbidden != "" && (within(root, forbidden) || within(forbidden, root)) {
			return fmt.Errorf("identity-material staging must not overlap checkout or live VM roots")
		}
	}
	return nil
}
