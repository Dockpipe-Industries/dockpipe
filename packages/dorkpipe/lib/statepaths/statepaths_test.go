package statepaths

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
	"testing"
)

func TestProviderPoolScratchDirUsesPackageState(t *testing.T) {
	root := t.TempDir()
	got, err := ProviderPoolScratchDir(root)
	if err != nil {
		t.Fatal(err)
	}
	wantSuffix := filepath.Join("bin", ".dockpipe", "packages", "dorkpipe", "provider-pools", "scratch")
	if !strings.HasSuffix(filepath.Clean(got), wantSuffix) {
		t.Fatalf("scratch dir = %q, want suffix %q", got, wantSuffix)
	}
}

func TestProviderPoolSessionAdaptersDirUsesPackageState(t *testing.T) {
	root := t.TempDir()
	got, err := ProviderPoolSessionAdaptersDir(root)
	if err != nil {
		t.Fatal(err)
	}
	wantSuffix := filepath.Join("bin", ".dockpipe", "packages", "dorkpipe", "provider-pools", "session-adapters")
	if !strings.HasSuffix(filepath.Clean(got), wantSuffix) {
		t.Fatalf("session adapters dir = %q, want suffix %q", got, wantSuffix)
	}
}

func TestProviderPoolAppServerDirUsesPackageState(t *testing.T) {
	root := t.TempDir()
	got, err := ProviderPoolAppServerDir(root)
	if err != nil {
		t.Fatal(err)
	}
	wantSuffix := filepath.Join("bin", ".dockpipe", "packages", "dorkpipe", "provider-pools", "app-server")
	if !strings.HasSuffix(filepath.Clean(got), wantSuffix) {
		t.Fatalf("app server dir = %q, want suffix %q", got, wantSuffix)
	}
}

func TestProviderPoolAppServerAggregatePathUsesHashedPackageStateName(t *testing.T) {
	root := t.TempDir()
	sessionID := "pipeon-session-sensitive"
	first, err := ProviderPoolAppServerAggregatePath(root, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ProviderPoolAppServerAggregatePath(root, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(sessionID))
	wantSuffix := filepath.Join("bin", ".dockpipe", "packages", "dorkpipe", "provider-pools", "app-server", "aggregates", hex.EncodeToString(digest[:])+".json")
	if first != second || !strings.HasSuffix(filepath.Clean(first), wantSuffix) {
		t.Fatalf("aggregate path = %q, want deterministic suffix %q", first, wantSuffix)
	}
	if strings.Contains(first, sessionID) {
		t.Fatalf("aggregate path leaks raw session identity: %q", first)
	}
}

func TestProviderPoolAppServerAggregatePathRejectsInvalidSessionIdentity(t *testing.T) {
	invalidUTF8 := string([]byte{0xff})
	for _, sessionID := range []string{"", " session", "session ", "session id", "session\nidentity", invalidUTF8, strings.Repeat("s", 257)} {
		if path, err := ProviderPoolAppServerAggregatePath(t.TempDir(), sessionID); err == nil || path != "" {
			t.Fatalf("unsafe session identity %q produced path %q with error %v", sessionID, path, err)
		}
	}
}
