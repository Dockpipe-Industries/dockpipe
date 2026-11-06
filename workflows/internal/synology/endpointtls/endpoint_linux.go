//go:build linux

// Package endpointtls owns per-install pinned local HTTPS endpoints.
package endpointtls

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

const privateEndpointDirectory = "endpoint-private"
const publicEndpointDirectory = "endpoint-public"
const endpointKeyName = "server-key.pem"
const endpointCertificateName = "ca.pem"
const maximumEndpointFileBytes = 16 << 10

func Directory(path string, mode os.FileMode, create bool) error {
	if create {
		err := os.Mkdir(path, mode)
		if err != nil && !errors.Is(err, os.ErrExist) {
			return errors.New("cannot create endpoint directory")
		}
		if err == nil && os.Chmod(path, mode) != nil {
			return errors.New("cannot establish endpoint directory permissions")
		}
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != mode {
		return errors.New("endpoint directory permissions are invalid")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != os.Geteuid() {
		return errors.New("endpoint directory ownership is invalid")
	}
	return nil
}

func Paths(root string, create bool) (string, string, error) {
	// DSM's fixed FHS var alias is permitted; neither runtime leaf may be a link.
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil || !filepath.IsAbs(resolved) {
		return "", "", errors.New("endpoint var directory unavailable")
	}
	if err := Directory(resolved, 0755, false); err != nil {
		return "", "", err
	}
	private := filepath.Join(resolved, privateEndpointDirectory)
	public := filepath.Join(resolved, publicEndpointDirectory)
	if err := Directory(private, 0700, create); err != nil {
		return "", "", err
	}
	if err := Directory(public, 0755, create); err != nil {
		return "", "", err
	}
	return filepath.Join(private, endpointKeyName), filepath.Join(public, endpointCertificateName), nil
}

func File(path string, mode os.FileMode, owner int) ([]byte, error) {
	descriptor, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_CLOEXEC|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, errors.New("endpoint material unavailable")
	}
	file := os.NewFile(uintptr(descriptor), "endpoint material")
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != mode || info.Size() <= 0 || info.Size() > maximumEndpointFileBytes {
		return nil, errors.New("endpoint material permissions or size are invalid")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != owner {
		return nil, errors.New("endpoint material ownership is invalid")
	}
	data, err := io.ReadAll(io.LimitReader(file, maximumEndpointFileBytes+1))
	if err != nil || len(data) > maximumEndpointFileBytes {
		return nil, errors.New("endpoint material exceeds read budget")
	}
	return data, nil
}

func Load(root string) (*tls.Config, error) {
	keyPath, certificatePath, err := Paths(root, false)
	if err != nil {
		return nil, err
	}
	key, err := File(keyPath, 0600, os.Geteuid())
	if err != nil {
		return nil, err
	}
	certificate, err := File(certificatePath, 0644, os.Geteuid())
	if err != nil {
		return nil, err
	}
	pair, err := tls.X509KeyPair(certificate, key)
	if err != nil || len(pair.Certificate) != 1 {
		return nil, errors.New("endpoint certificate and key are invalid")
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil || !leaf.IsCA || leaf.CheckSignatureFrom(leaf) != nil || leaf.VerifyHostname("localhost") != nil || leaf.VerifyHostname("127.0.0.1") != nil {
		return nil, errors.New("endpoint certificate identity is invalid")
	}
	now := time.Now()
	if now.Before(leaf.NotBefore) || now.After(leaf.NotAfter) {
		return nil, errors.New("endpoint certificate is outside its validity interval")
	}
	pair.Leaf = leaf
	return &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{pair}}, nil
}

func publishEndpointFile(path string, mode os.FileMode, data []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return errors.New("endpoint material already exists or cannot be created")
	}
	defer file.Close()
	if file.Chmod(mode) != nil {
		return errors.New("cannot establish endpoint material permissions")
	}
	if _, err := file.Write(data); err != nil {
		return errors.New("cannot write complete endpoint material")
	}
	if file.Sync() != nil {
		return errors.New("cannot persist endpoint material")
	}
	return nil
}

func Initialize(root string) error {
	keyPath, certificatePath, err := Paths(root, true)
	if err != nil {
		return err
	}
	_, keyError := os.Lstat(keyPath)
	_, certificateError := os.Lstat(certificatePath)
	if keyError == nil || certificateError == nil {
		// Preserve an established pin across upgrades; never replace partial,
		// malformed or unexpectedly permissioned material automatically.
		_, err := Load(root)
		return err
	}
	if !errors.Is(keyError, os.ErrNotExist) || !errors.Is(certificateError, os.ErrNotExist) {
		return errors.New("cannot admit endpoint material destinations")
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return errors.New("cannot generate endpoint key")
	}
	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, serialLimit)
	if err != nil {
		return errors.New("cannot generate endpoint certificate serial")
	}
	if serial.Sign() == 0 {
		serial.SetInt64(1)
	}
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: serial, Subject: pkix.Name{CommonName: "Dockpipe qualification local endpoint"},
		NotBefore: now.Add(-5 * time.Minute), NotAfter: now.AddDate(10, 0, 0),
		DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
		IsCA: true, BasicConstraintsValid: true,
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	certificate, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return errors.New("cannot generate endpoint certificate")
	}
	encodedKey, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return errors.New("cannot encode endpoint private key")
	}
	if err := publishEndpointFile(keyPath, 0600, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encodedKey})); err != nil {
		return err
	}
	if err := publishEndpointFile(certificatePath, 0644, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate})); err != nil {
		return err
	}
	_, err = Load(root)
	return err
}
