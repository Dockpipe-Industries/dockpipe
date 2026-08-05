package keymaterial

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
)

type Pair struct {
	PublicPath  string
	PrivatePath string
}

// Generate creates one new Ed25519 identity and refuses to replace an
// existing per-instance key. Private material is owner-only.
func Generate(root, name string) (Pair, error) {
	var pair Pair
	if !filepath.IsAbs(root) || name == "" {
		return pair, fmt.Errorf("key root must be absolute and name non-empty")
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return pair, err
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return pair, err
	}
	pair = Pair{PublicPath: filepath.Join(root, name+".pub"), PrivatePath: filepath.Join(root, name+".key")}
	privateFile, err := os.OpenFile(pair.PrivatePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return Pair{}, fmt.Errorf("create private key: %w", err)
	}
	if _, err := privateFile.Write(priv); err != nil {
		privateFile.Close()
		return Pair{}, err
	}
	if err := privateFile.Sync(); err != nil {
		privateFile.Close()
		return Pair{}, err
	}
	if err := privateFile.Close(); err != nil {
		return Pair{}, err
	}
	if err := os.WriteFile(pair.PublicPath, pub, 0o644); err != nil {
		return Pair{}, err
	}
	return pair, nil
}
