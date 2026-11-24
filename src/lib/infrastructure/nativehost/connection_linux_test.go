//go:build linux

package nativehost

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConnectionReferenceAndKeyAdmission(t *testing.T) {
	directory := t.TempDir()
	key := filepath.Join(directory, "key")
	config := filepath.Join(directory, "connection.json")
	if err := os.WriteFile(key, []byte(strings.Repeat("a", 64)), 0600); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(Connection{Schema: ConnectionSchema, Socket: "/tmp/host.sock", KeyFile: key})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadConnection(config); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readKey(key); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(key, 0644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readKey(key); err == nil {
		t.Fatal("public key file accepted")
	}
	link := filepath.Join(directory, "link")
	if err := os.Symlink(config, link); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadConnection(link); err == nil {
		t.Fatal("symlink config accepted")
	}
	if _, err := ReadConnection("relative.json"); err == nil {
		t.Fatal("relative config accepted")
	}
}
