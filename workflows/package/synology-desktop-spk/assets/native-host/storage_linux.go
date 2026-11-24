//go:build linux

package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"dockpipe/workflows/internal/synology/endpointtls"
	"dockpipe/workflows/internal/synology/localaccount"
)

const packageRoot = "/var/packages/dockpipe-desktop"
const endpointRoot = packageRoot + "/etc/native-host"
const clientRoot = packageRoot + "/var/native-client"
const workerSocket = endpointRoot + "/native-worker.sock"
const clientSocket = clientRoot + "/client.sock"
const maximumStoredBytes = 4096

func randomToken() (string, error) {
	var value [32]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", errors.New("random_material_unavailable")
	}
	return hex.EncodeToString(value[:]), nil
}

func readPrivate(path string) ([]byte, error) {
	return readPrivateBounded(path, maximumStoredBytes)
}

func readPrivateBounded(path string, maximumBytes int64) ([]byte, error) {
	descriptor, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_CLOEXEC|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(descriptor), "native host settings")
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || info.Size() <= 0 || info.Size() > maximumBytes {
		return nil, errors.New("private_settings_invalid")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != os.Geteuid() {
		return nil, errors.New("private_settings_owner_invalid")
	}
	data, err := io.ReadAll(io.LimitReader(file, maximumBytes+1))
	if err != nil || int64(len(data)) > maximumBytes {
		return nil, errors.New("private_settings_exceed_budget")
	}
	return data, nil
}

func publishPrivate(path string, data []byte) error {
	if len(data) > maximumStoredBytes {
		return errors.New("settings_exceed_budget")
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(data); err != nil {
		return err
	}
	return file.Sync()
}

func initializeStorage() error {
	if os.Geteuid() == 0 {
		return errors.New("ordinary_package_identity_required")
	}
	if err := normalizeStreamerCertificate(packageRoot + "/var/ssl/cert.pem"); err != nil {
		return err
	}
	if err := endpointtls.Directory(endpointRoot, 0755, true); err != nil {
		return err
	}
	if err := endpointtls.Initialize(endpointRoot); err != nil {
		return err
	}
	if err := endpointtls.Directory(clientRoot, 0700, true); err != nil {
		return err
	}
	keyPath := filepath.Join(clientRoot, "client.key")
	key, err := readPrivate(keyPath)
	if errors.Is(err, os.ErrNotExist) {
		token, tokenError := randomToken()
		if tokenError != nil {
			return tokenError
		}
		if err := publishPrivate(keyPath, []byte(token)); err != nil {
			return err
		}
		key = []byte(token)
	} else if err != nil {
		return err
	}
	decoded, err := hex.DecodeString(string(key))
	if err != nil || len(decoded) != 32 || hex.EncodeToString(decoded) != string(key) {
		return errors.New("client_key_invalid")
	}
	reference := struct {
		Schema  string `json:"schema"`
		Socket  string `json:"socket"`
		KeyFile string `json:"key_file"`
	}{"dockpipe.native_host_connection.v1", "/config/native-client/client.sock", "/config/native-client/client.key"}
	data, _ := json.Marshal(reference)
	path := filepath.Join(clientRoot, "connection.json")
	existing, err := readPrivate(path)
	if errors.Is(err, os.ErrNotExist) {
		return publishPrivate(path, data)
	}
	if err != nil || string(existing) != string(data) {
		return errors.New("client_reference_invalid")
	}
	return nil
}

type ownerBinding struct {
	Username string `json:"username"`
	UID      uint32 `json:"uid"`
	GID      uint32 `json:"gid"`
}

type ownerStore struct{ path string }

func (store ownerStore) read() (*ownerBinding, error) {
	data, err := readPrivate(store.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, errors.New("owner_settings_unavailable")
	}
	var binding ownerBinding
	if json.Unmarshal(data, &binding) != nil || binding.Username == "" || binding.Username == "root" || binding.UID < 1024 || binding.GID == 0 {
		return nil, errors.New("owner_settings_invalid")
	}
	return &binding, nil
}

func sameOwner(binding *ownerBinding, account localaccount.Account) bool {
	return binding == nil || (binding.Username == account.Username && binding.UID == account.UID && binding.GID == account.GID)
}

func (store ownerStore) bind(account localaccount.Account) error {
	existing, err := store.read()
	if err != nil || !sameOwner(existing, account) {
		return errors.New("different_operator_configured")
	}
	if existing != nil {
		return nil
	}
	data, err := json.Marshal(ownerBinding{account.Username, account.UID, account.GID})
	if err != nil || publishPrivate(store.path, data) != nil {
		return errors.New("owner_binding_unavailable")
	}
	return nil
}
