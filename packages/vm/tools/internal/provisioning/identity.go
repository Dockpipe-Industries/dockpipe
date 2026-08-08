package provisioning

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

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
	record := struct {
		RunID          string `json:"run_id"`
		CohortID       string `json:"cohort_id"`
		MachineUUID    string `json:"machine_uuid"`
		DiskSerial     string `json:"disk_serial"`
		FilesystemUUID string `json:"filesystem_uuid"`
		BootstrapNonce string `json:"bootstrap_nonce"`
	}{c.RunID, c.CohortID, c.MachineUUID, c.DiskSerial, c.FilesystemUUID, c.BootstrapNonce}
	b, err := json.Marshal(record)
	if err != nil {
		return ReservedIdentity{}, err
	}
	if err := writeExclusive(filepath.Join(root, "identity.json"), b, 0o600); err != nil {
		return ReservedIdentity{}, err
	}
	failed = false
	return out, nil
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
