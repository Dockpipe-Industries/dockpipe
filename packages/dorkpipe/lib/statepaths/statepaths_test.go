package statepaths

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
	"testing"
)

func TestProviderPoolScratchDirUsesPackageRuntime(t *testing.T) {
	root := t.TempDir()
	got, err := ProviderPoolScratchDir(root)
	if err != nil {
		t.Fatal(err)
	}
	runtimeRoot, err := PackageRuntimeDir(root)
	if err != nil {
		t.Fatal(err)
	}
	wantSuffix := filepath.Join(runtimeRoot, "provider-pools", "scratch")
	if filepath.Clean(got) != filepath.Clean(wantSuffix) {
		t.Fatalf("scratch dir = %q, want %q", got, wantSuffix)
	}
}

func TestProviderPoolSessionAdaptersDirUsesPackageState(t *testing.T) {
	root := isolatedDurableStateWorkdir(t)
	got, err := ProviderPoolSessionAdaptersDir(root)
	if err != nil {
		t.Fatal(err)
	}
	wantSuffix := filepath.Join("provider-pools", "session-adapters")
	if !strings.HasSuffix(filepath.Clean(got), wantSuffix) || strings.HasPrefix(filepath.Clean(got), filepath.Clean(root)+string(filepath.Separator)) {
		t.Fatalf("session adapters dir = %q, want suffix %q", got, wantSuffix)
	}
}

func TestLearningPathsUseDurableAuthorityAndKeepDerivedExportsDisposable(t *testing.T) {
	root := isolatedDurableStateWorkdir(t)
	metrics, err := MetricsPath(root)
	if err != nil {
		t.Fatal(err)
	}
	training, err := TrainingMetricsPath(root)
	if err != nil {
		t.Fatal(err)
	}
	insights, err := InsightsPath(root)
	if err != nil {
		t.Fatal(err)
	}
	for name, path := range map[string]string{"metrics": metrics, "training": training, "insights": insights} {
		if strings.HasPrefix(filepath.Clean(path), filepath.Clean(root)+string(filepath.Separator)) {
			t.Fatalf("%s path remained inside checkout: %q", name, path)
		}
		if !strings.Contains(path, filepath.Join("learning")) {
			t.Fatalf("%s path does not use atomic learning authority: %q", name, path)
		}
	}
	derived, err := InsightsByCategoryDir(root)
	if err != nil {
		t.Fatal(err)
	}
	runtimeRoot, err := PackageRuntimeDir(root)
	if err != nil {
		t.Fatal(err)
	}
	wantDerived := filepath.Join(runtimeRoot, "analysis", "by-category")
	if filepath.Clean(derived) != filepath.Clean(wantDerived) {
		t.Fatalf("derived insights path = %q, want disposable path %q", derived, wantDerived)
	}
}

func TestDisposablePathsUseRuntimeAndIgnoreDurableCompatibilityEnv(t *testing.T) {
	root := t.TempDir()
	durableTrap := filepath.Join(t.TempDir(), "durable-package-state")
	t.Setenv("DOCKPIPE_PACKAGE_STATE_DIR", durableTrap)
	runtimeRoot, err := PackageRuntimeDir(root)
	if err != nil {
		t.Fatal(err)
	}
	paths := map[string]func() (string, error){
		"edit":          func() (string, error) { return EditArtifactsDir(root, "request") },
		"reasoning":     func() (string, error) { return ReasoningArtifactsDir(root, "request") },
		"run":           func() (string, error) { return RunPath(root) },
		"nodes":         func() (string, error) { return NodesDir(root) },
		"ci":            func() (string, error) { return PackageCIDir(root) },
		"self-analysis": func() (string, error) { return SelfAnalysisDir(root) },
		"leases":        func() (string, error) { return ProviderPoolLeasesDir(root) },
		"scratch":       func() (string, error) { return ProviderPoolScratchDir(root) },
		"handoff":       func() (string, error) { return CursorPromptPath(root) },
	}
	for name, resolve := range paths {
		path, err := resolve()
		if err != nil {
			t.Fatalf("%s path: %v", name, err)
		}
		if !strings.HasPrefix(filepath.Clean(path), filepath.Clean(runtimeRoot)+string(filepath.Separator)) {
			t.Fatalf("%s path = %q, want package runtime prefix %q", name, path, runtimeRoot)
		}
		if strings.HasPrefix(filepath.Clean(path), filepath.Clean(durableTrap)) {
			t.Fatalf("%s path fell back to durable compatibility state: %q", name, path)
		}
	}
}

func TestProviderPoolAppServerDirUsesPackageState(t *testing.T) {
	root := isolatedDurableStateWorkdir(t)
	got, err := ProviderPoolAppServerDir(root)
	if err != nil {
		t.Fatal(err)
	}
	wantSuffix := filepath.Join("provider-pools", "app-server")
	if !strings.HasSuffix(filepath.Clean(got), wantSuffix) || strings.HasPrefix(filepath.Clean(got), filepath.Clean(root)+string(filepath.Separator)) {
		t.Fatalf("app server dir = %q, want suffix %q", got, wantSuffix)
	}
}

func TestProviderPoolAppServerAggregatePathUsesHashedPackageStateName(t *testing.T) {
	root := isolatedDurableStateWorkdir(t)
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
	wantSuffix := filepath.Join("provider-pools", "app-server", "aggregates", hex.EncodeToString(digest[:])+".json")
	if first != second || !strings.HasSuffix(filepath.Clean(first), wantSuffix) {
		t.Fatalf("aggregate path = %q, want deterministic suffix %q", first, wantSuffix)
	}
	if strings.Contains(first, sessionID) {
		t.Fatalf("aggregate path leaks raw session identity: %q", first)
	}
}

func isolatedDurableStateWorkdir(t *testing.T) string {
	t.Helper()
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)
	t.Setenv("LOCALAPPDATA", stateHome)
	t.Setenv("HOME", stateHome)
	return t.TempDir()
}

func TestProviderPoolAppServerAggregatePathRejectsInvalidSessionIdentity(t *testing.T) {
	root := isolatedDurableStateWorkdir(t)
	invalidUTF8 := string([]byte{0xff})
	for _, sessionID := range []string{"", " session", "session ", "session id", "session\nidentity", invalidUTF8, strings.Repeat("s", 257)} {
		if path, err := ProviderPoolAppServerAggregatePath(root, sessionID); err == nil || path != "" {
			t.Fatalf("unsafe session identity %q produced path %q with error %v", sessionID, path, err)
		}
	}
}
