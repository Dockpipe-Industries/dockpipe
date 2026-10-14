package remotecmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	contract "dockpipe/src/lib/domain/remote"
	remoteio "dockpipe/src/lib/infrastructure/remote"
)

func TestMonitoringRequiresLocalWorkerAndExplicitChoice(t *testing.T) {
	root := filepath.Join(t.TempDir(), "remote")
	if err := remoteio.PrivateDirectory(root); err != nil {
		t.Fatal(err)
	}
	if configureMonitoring(root, true, true, false) == nil {
		t.Fatal("enabled monitoring without a paired worker")
	}
	worker := contract.WorkerConfig{Schema: contract.Version, Node: "fixture", Token: strings.Repeat("a", 64), Endpoint: "http://127.0.0.1:47831"}
	path := filepath.Join(root, "worker.json")
	if err := remoteio.WritePrivate(path, worker); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	if configureMonitoring(root, false, false, false) == nil || configureMonitoring(root, true, false, true) == nil {
		t.Fatal("accepted missing or conflicting monitoring permission")
	}
	if err := configureMonitoring(root, true, true, false); err != nil {
		t.Fatal(err)
	}
	var permission remoteio.MonitoringPermission
	if err := remoteio.ReadPrivate(filepath.Join(root, "monitoring.json"), &permission); err != nil || !permission.System || !permission.Docker {
		t.Fatal("permission not persisted", err)
	}
	if err := configureMonitoring(root, false, false, true); err != nil {
		t.Fatal(err)
	}
	if err := remoteio.ReadPrivate(filepath.Join(root, "monitoring.json"), &permission); err != nil || permission.System || permission.Docker {
		t.Fatal("permission not revoked", err)
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("monitoring changed delivery or pairing authority")
	}
}
