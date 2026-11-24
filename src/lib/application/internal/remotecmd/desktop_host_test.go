package remotecmd

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	contract "dockpipe/src/lib/domain/remote"
	remoteio "dockpipe/src/lib/infrastructure/remote"
	"dockpipe/src/lib/infrastructure/remoteidentity"
)

func TestContainedDesktopWorkerRefusesBeforeReadiness(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux application containment boundary")
	}
	t.Setenv("FLATPAK_ID", "com.example.DesktopViewer")
	var output desktopBuffer
	err := streamDesktopWorker(context.Background(), t.TempDir(), io.NopCloser(bytes.NewReader(nil)), &output)
	if err == nil || !strings.Contains(err.Error(), "incoming desktop capture and control are unavailable") {
		t.Fatalf("contained worker admission = %v", err)
	}
	if output.Len() != 0 {
		t.Fatal("contained worker published readiness")
	}
}

func TestContainedDesktopGrantCanBeDisabledButNotEnabled(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux application containment boundary")
	}
	t.Setenv("FLATPAK_ID", "com.example.DesktopViewer")
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	identity, err := remoteidentity.New()
	if err != nil {
		t.Fatal(err)
	}
	broker, err := remoteidentity.New()
	if err != nil {
		t.Fatal(err)
	}
	config := contract.WorkerConfig{
		Schema: contract.Version, Node: "fixture", Endpoint: "https://example.invalid",
		Token: strings.Repeat("a", 64), MachineKey: identity.PublicKey,
		MachinePrivateKey: identity.PrivateKey, BrokerKey: broker.PublicKey,
	}
	if err := remoteio.WritePrivate(filepath.Join(root, "worker.json"), config); err != nil {
		t.Fatal(err)
	}
	permissions := remoteio.DesktopPermissions{Schema: contract.DesktopVersion, Provider: desktopTestIntent().Provider}
	if err := remoteio.SaveDesktopPermissions(root, permissions); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(root, "desktop.json"))
	if err != nil {
		t.Fatal(err)
	}
	permissions.View = true
	data, err := json.Marshal(permissions)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "request.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := configureDesktop(context.Background(), root, path); err == nil || !strings.Contains(err.Error(), "incoming desktop") {
		t.Fatalf("contained grant enable = %v", err)
	}
	after, err := os.ReadFile(filepath.Join(root, "desktop.json"))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("rejected contained grant changed owner permissions", err)
	}
	if err := os.WriteFile(path, before, 0600); err != nil {
		t.Fatal(err)
	}
	if err := configureDesktop(context.Background(), root, path); err != nil {
		t.Fatal("contained operator could not disable desktop hosting", err)
	}
}

func TestContainedDesktopPreparationRemainsAvailable(t *testing.T) {
	t.Setenv("FLATPAK_ID", "com.example.DesktopViewer")
	request := desktopTestIntent()
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "intent.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := prepareDesktop(context.Background(), path, &output); err != nil {
		t.Fatal(err)
	}
	var prepared desktopPrepared
	if err := json.Unmarshal(output.Bytes(), &prepared); err != nil {
		t.Fatal(err)
	}
	if prepared.Reference != request.Reference() || prepared.Mode != request.Mode {
		t.Fatal("contained viewer preparation changed its authorized session")
	}
}
