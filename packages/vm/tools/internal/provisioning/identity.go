package provisioning

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

type ReservedIdentityRecord struct {
	RunID          string `json:"run_id"`
	CohortID       string `json:"cohort_id"`
	MachineUUID    string `json:"machine_uuid"`
	DiskSerial     string `json:"disk_serial"`
	FilesystemUUID string `json:"filesystem_uuid"`
	BootstrapNonce string `json:"bootstrap_nonce"`
}

// ReservedPublicIdentity is the non-secret subset needed to authenticate
// durable, historical qualification evidence. Private-key bytes are never
// opened by LoadReservedPublicIdentity.
type ReservedPublicIdentity struct {
	ControllerPublic ed25519.PublicKey
	GuestPublic      ed25519.PublicKey
	Record           ReservedIdentityRecord
}

// LoadReservedPublicIdentity reads only the public keys and identity record
// from an exact reserved identity directory. It inspects private-key metadata
// to reject an incomplete or widened bundle but never reads private-key bytes.
func LoadReservedPublicIdentity(root string) (ReservedPublicIdentity, error) {
	var out ReservedPublicIdentity
	if !filepath.IsAbs(root) || filepath.Base(root) != "identity" {
		return out, fmt.Errorf("reserved identity root is invalid")
	}
	info, err := os.Lstat(root)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() || info.Mode().Perm() != 0o700 || !ownedByCurrentUser(info) {
		return out, fmt.Errorf("reserved identity root must remain private")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return out, fmt.Errorf("inspect reserved identity inventory: %w", err)
	}
	want := map[string]int64{
		"bootstrap-nonce": 64,
		"controller.key":  ed25519.PrivateKeySize,
		"controller.pub":  ed25519.PublicKeySize,
		"guest.key":       ed25519.PrivateKeySize,
		"guest.pub":       ed25519.PublicKeySize,
		"identity.json":   -1,
	}
	if len(entries) != len(want) {
		return out, fmt.Errorf("reserved identity inventory changed")
	}
	for _, entry := range entries {
		size, ok := want[entry.Name()]
		if !ok {
			return out, fmt.Errorf("reserved identity inventory changed")
		}
		fileInfo, err := os.Lstat(filepath.Join(root, entry.Name()))
		if err != nil || fileInfo.Mode()&os.ModeSymlink != 0 || !fileInfo.Mode().IsRegular() || fileInfo.Mode().Perm() != 0o600 || !ownedByCurrentUser(fileInfo) || (size >= 0 && fileInfo.Size() != size) {
			return out, fmt.Errorf("reserved identity file %s changed", entry.Name())
		}
	}
	read := func(name string) ([]byte, error) {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			return nil, fmt.Errorf("read reserved identity file %s: %w", name, err)
		}
		return data, nil
	}
	controllerPublic, err := read("controller.pub")
	if err != nil {
		return out, err
	}
	guestPublic, err := read("guest.pub")
	if err != nil {
		return out, err
	}
	nonce, err := read("bootstrap-nonce")
	if err != nil {
		return out, err
	}
	recordJSON, err := read("identity.json")
	if err != nil {
		return out, err
	}
	decoder := json.NewDecoder(bytes.NewReader(recordJSON))
	decoder.DisallowUnknownFields()
	var record ReservedIdentityRecord
	if decoder.Decode(&record) != nil {
		return out, fmt.Errorf("reserved identity record changed")
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF || !idPattern.MatchString(record.RunID) || !idPattern.MatchString(record.CohortID) || !uuidPattern.MatchString(record.MachineUUID) || !serialPattern.MatchString(record.DiskSerial) || !uuidPattern.MatchString(record.FilesystemUUID) || !noncePattern.MatchString(record.BootstrapNonce) || string(nonce) != record.BootstrapNonce {
		return out, fmt.Errorf("reserved identity record changed")
	}
	out = ReservedPublicIdentity{ControllerPublic: ed25519.PublicKey(controllerPublic), GuestPublic: ed25519.PublicKey(guestPublic), Record: record}
	return out, nil
}

// LoadReservedKeyMaterial reopens only the exact durable identity created for
// one authorized instance. It rejects links, widened modes, ownership changes,
// key substitution, and identity-record drift.
func LoadReservedKeyMaterial(root string, c Contract) (KeyMaterial, error) {
	var material KeyMaterial
	if !filepath.IsAbs(root) || filepath.Base(root) != "identity" {
		return material, fmt.Errorf("reserved identity root is invalid")
	}
	info, err := os.Lstat(root)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() || info.Mode().Perm() != 0o700 {
		return material, fmt.Errorf("reserved identity root must remain private")
	}
	if stat, ok := info.Sys().(*syscall.Stat_t); ok && int(stat.Uid) != os.Geteuid() {
		return material, fmt.Errorf("reserved identity ownership changed")
	}
	read := func(name string, size int) ([]byte, error) {
		path := filepath.Join(root, name)
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 || info.Size() != int64(size) {
			return nil, fmt.Errorf("reserved identity file %s changed", name)
		}
		if stat, ok := info.Sys().(*syscall.Stat_t); ok && int(stat.Uid) != os.Geteuid() {
			return nil, fmt.Errorf("reserved identity file %s ownership changed", name)
		}
		return os.ReadFile(path)
	}
	controllerPublic, err := read("controller.pub", ed25519.PublicKeySize)
	if err != nil {
		return material, err
	}
	controllerPrivate, err := read("controller.key", ed25519.PrivateKeySize)
	if err != nil {
		return material, err
	}
	guestPublic, err := read("guest.pub", ed25519.PublicKeySize)
	if err != nil {
		return material, err
	}
	guestPrivate, err := read("guest.key", ed25519.PrivateKeySize)
	if err != nil {
		return material, err
	}
	material = KeyMaterial{ControllerPublic: controllerPublic, ControllerPrivate: controllerPrivate, GuestPublic: guestPublic, GuestPrivate: guestPrivate}
	if err := validateKeyMaterial(c, material); err != nil {
		return KeyMaterial{}, err
	}
	nonce, err := read("bootstrap-nonce", len(c.BootstrapNonce))
	if err != nil || string(nonce) != c.BootstrapNonce {
		return KeyMaterial{}, fmt.Errorf("reserved bootstrap nonce changed")
	}
	var record ReservedIdentityRecord
	recordPath := filepath.Join(root, "identity.json")
	recordInfo, err := os.Lstat(recordPath)
	if err != nil || recordInfo.Mode()&os.ModeSymlink != 0 || !recordInfo.Mode().IsRegular() || recordInfo.Mode().Perm() != 0o600 {
		return KeyMaterial{}, fmt.Errorf("reserved identity record changed")
	}
	if stat, ok := recordInfo.Sys().(*syscall.Stat_t); ok && int(stat.Uid) != os.Geteuid() {
		return KeyMaterial{}, fmt.Errorf("reserved identity record ownership changed")
	}
	recordJSON, err := os.ReadFile(recordPath)
	decoder := json.NewDecoder(bytes.NewReader(recordJSON))
	decoder.DisallowUnknownFields()
	if err != nil || decoder.Decode(&record) != nil {
		return KeyMaterial{}, fmt.Errorf("reserved identity record changed")
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF || record.RunID != c.RunID || record.CohortID != c.CohortID || record.MachineUUID != c.MachineUUID || record.DiskSerial != c.DiskSerial || record.FilesystemUUID != c.FilesystemUUID || record.BootstrapNonce != c.BootstrapNonce {
		return KeyMaterial{}, fmt.Errorf("reserved identity record changed")
	}
	return material, nil
}

type KeyMaterial struct {
	ControllerPublic  ed25519.PublicKey
	ControllerPrivate ed25519.PrivateKey
	GuestPublic       ed25519.PublicKey
	GuestPrivate      ed25519.PrivateKey
}

// GenerateKeyMaterial creates the fresh in-memory identities whose public
// hashes must be bound into the contract before its plan is authorized.
func GenerateKeyMaterial() (KeyMaterial, error) {
	controllerPublic, controllerPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return KeyMaterial{}, err
	}
	guestPublic, guestPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return KeyMaterial{}, err
	}
	return KeyMaterial{
		ControllerPublic: controllerPublic, ControllerPrivate: controllerPrivate,
		GuestPublic: guestPublic, GuestPrivate: guestPrivate,
	}, nil
}

type ReservedIdentity struct {
	Root                   string `json:"root"`
	ControllerPublicKey    string `json:"controller_public_key"`
	ControllerPrivateKey   string `json:"controller_private_key"`
	GuestPublicKey         string `json:"guest_public_key"`
	GuestPrivateKey        string `json:"guest_private_key"`
	ControllerPublicSHA256 string `json:"controller_public_sha256"`
	GuestPublicSHA256      string `json:"guest_public_sha256"`
	BootstrapNoncePath     string `json:"bootstrap_nonce_path"`
}

// ReserveIdentity creates a brand-new instance identity directory containing
// the exact fresh keys pinned before authorization. The root, keys, and bootstrap nonce
// all use exclusive creation and are never replaced.
func ReserveIdentity(root string, c Contract, material KeyMaterial) (ReservedIdentity, error) {
	var out ReservedIdentity
	if err := validateContractIdentities(c); err != nil {
		return out, err
	}
	if err := validateKeyMaterial(c, material); err != nil {
		return out, err
	}
	if !filepath.IsAbs(root) {
		return out, fmt.Errorf("identity root must be absolute")
	}
	if err := os.Mkdir(root, 0o700); err != nil {
		return out, fmt.Errorf("exclusively create identity root: %w", err)
	}
	failed := true
	defer func() {
		if failed {
			_ = os.Remove(filepath.Join(root, "identity.json"))
			_ = os.Remove(filepath.Join(root, "bootstrap-nonce"))
			_ = os.Remove(filepath.Join(root, "controller.pub"))
			_ = os.Remove(filepath.Join(root, "controller.key"))
			_ = os.Remove(filepath.Join(root, "guest.pub"))
			_ = os.Remove(filepath.Join(root, "guest.key"))
			_ = os.Remove(root)
		}
	}()
	out = ReservedIdentity{
		Root:                root,
		ControllerPublicKey: filepath.Join(root, "controller.pub"), ControllerPrivateKey: filepath.Join(root, "controller.key"),
		GuestPublicKey: filepath.Join(root, "guest.pub"), GuestPrivateKey: filepath.Join(root, "guest.key"), BootstrapNoncePath: filepath.Join(root, "bootstrap-nonce"),
	}
	out.ControllerPublicSHA256 = hash(material.ControllerPublic)
	out.GuestPublicSHA256 = hash(material.GuestPublic)
	for _, file := range []struct {
		path string
		data []byte
		mode os.FileMode
	}{
		{out.ControllerPublicKey, material.ControllerPublic, 0o600}, {out.ControllerPrivateKey, material.ControllerPrivate, 0o600},
		{out.GuestPublicKey, material.GuestPublic, 0o600}, {out.GuestPrivateKey, material.GuestPrivate, 0o600},
		{out.BootstrapNoncePath, []byte(c.BootstrapNonce), 0o600},
	} {
		if err := writeExclusive(file.path, file.data, file.mode); err != nil {
			return ReservedIdentity{}, err
		}
	}
	record := ReservedIdentityRecord{c.RunID, c.CohortID, c.MachineUUID, c.DiskSerial, c.FilesystemUUID, c.BootstrapNonce}
	b, err := json.Marshal(record)
	if err != nil {
		return ReservedIdentity{}, err
	}
	if err := writeExclusive(filepath.Join(root, "identity.json"), b, 0o600); err != nil {
		return ReservedIdentity{}, err
	}
	if err := syncIdentityDir(root); err != nil {
		return ReservedIdentity{}, err
	}
	if err := syncIdentityDir(filepath.Dir(root)); err != nil {
		return ReservedIdentity{}, err
	}
	failed = false
	return out, nil
}

func syncIdentityDir(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}

func validateKeyMaterial(c Contract, material KeyMaterial) error {
	if len(material.ControllerPublic) != ed25519.PublicKeySize || len(material.ControllerPrivate) != ed25519.PrivateKeySize || len(material.GuestPublic) != ed25519.PublicKeySize || len(material.GuestPrivate) != ed25519.PrivateKeySize {
		return fmt.Errorf("exact Ed25519 controller and guest keypairs are required")
	}
	if !bytes.Equal(material.ControllerPrivate[32:], material.ControllerPublic) || !bytes.Equal(material.GuestPrivate[32:], material.GuestPublic) {
		return fmt.Errorf("Ed25519 private and public identities do not match")
	}
	if hash(material.ControllerPublic) != c.Artifacts.ControllerPublicKeySHA256 || hash(material.GuestPublic) != c.Artifacts.GuestPublicKeySHA256 {
		return fmt.Errorf("fresh Ed25519 identities do not match the authorized public-key pins")
	}
	return nil
}

func writeExclusive(path string, data []byte, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return fmt.Errorf("exclusively create %s: %w", filepath.Base(path), err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
