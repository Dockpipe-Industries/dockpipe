package manifest

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func sample(t *testing.T) Manifest {
	t.Helper()
	m, err := Load(filepath.Join("..", "..", "..", "manifests", "linux-qualification.json"))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func shortRuntimeRoot(t *testing.T, name string) string {
	t.Helper()
	root := t.TempDir()
	volumeRoot := filepath.VolumeName(root) + string(filepath.Separator)
	return filepath.Join(volumeRoot, name)
}

func TestQualificationManifestAndExactCommands(t *testing.T) {
	m := sample(t)
	mkfs, err := MkfsCommand(m, "/dev/disk/by-id/virtio-"+m.DataDisk.Serial)
	if err != nil {
		t.Fatal(err)
	}
	wantMkfs := []string{"mkfs.ext4", "-F", "-L", "dockpipe-qual", "-U", m.Filesystem.UUID, "-E", "lazy_itable_init=0,lazy_journal_init=0", "/dev/disk/by-id/virtio-" + m.DataDisk.Serial}
	if !slices.Equal(mkfs, wantMkfs) {
		t.Fatalf("mkfs tuple changed: %q", mkfs)
	}
	mount, err := MountCommand(m, "/dev/disk/by-uuid/"+m.Filesystem.UUID)
	if err != nil || !slices.Equal(mount, []string{"mount", "-t", "ext4", "-o", strings.Join(RequiredMountOptions, ","), "/dev/disk/by-uuid/" + m.Filesystem.UUID, QualificationMount}) {
		t.Fatalf("mount tuple changed: %q, %v", mount, err)
	}
}

func TestManifestRejectsUnsafeQualificationVariants(t *testing.T) {
	tests := map[string]func(*Manifest){
		"tcg":                func(m *Manifest) { m.Machine.Acceleration = "tcg" },
		"host as guest":      func(m *Manifest) { m.HostMachineUUID = m.MachineUUID },
		"boot ID input":      func(m *Manifest) { m.BootIDSource = "33333333-3333-4333-8333-333333333333" },
		"network":            func(m *Manifest) { m.Isolation.Network = true },
		"ssh":                func(m *Manifest) { m.Isolation.SSH = true },
		"share":              func(m *Manifest) { m.Isolation.Shares = []string{"/host"} },
		"physical disk":      func(m *Manifest) { m.Isolation.PhysicalDisks = []string{"/dev/sda"} },
		"passthrough":        func(m *Manifest) { m.Isolation.Passthrough = []string{"0000:01:00.0"} },
		"extra disk":         func(m *Manifest) { m.Isolation.ExtraDisks = []string{"third.raw"} },
		"arbitrary exec":     func(m *Manifest) { m.Isolation.ArbitraryExec = true },
		"wrong data size":    func(m *Manifest) { m.DataDisk.Bytes-- },
		"long OS serial":     func(m *Manifest) { m.OSDisk.Serial = strings.Repeat("o", VirtioBlockSerialMaxBytes+1) },
		"long data serial":   func(m *Manifest) { m.DataDisk.Serial = strings.Repeat("d", VirtioBlockSerialMaxBytes+1) },
		"discard":            func(m *Manifest) { m.QEMU.Discard = true },
		"snapshot":           func(m *Manifest) { m.QEMU.Snapshot = true },
		"unexpected mount":   func(m *Manifest) { m.Filesystem.MountOptions = append(m.Filesystem.MountOptions, "discard") },
		"unexpected feature": func(m *Manifest) { m.Filesystem.Features = append(m.Filesystem.Features, "encrypt") },
		"qemu 6.2":           func(m *Manifest) { m.QEMU.Version = "QEMU emulator version 6.2.0" },
		"live execution":     func(m *Manifest) { m.LiveExecutionApproved = true },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			m := sample(t)
			m.Filesystem.MountOptions = slices.Clone(m.Filesystem.MountOptions)
			m.Filesystem.Features = slices.Clone(m.Filesystem.Features)
			mutate(&m)
			if err := m.Validate(); err == nil {
				t.Fatal("expected unsafe manifest rejection")
			}
		})
	}
}

func TestQEMUPlanHasExactIsolationAndBlockPolicy(t *testing.T) {
	m := sample(t)
	plan, err := PlanQEMU(m, shortRuntimeRoot(t, "dp-vm-manifest"))
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(plan.Args, " ")
	for _, required := range []string{"q35,accel=kvm", "-cpu host", "-smp 2", "-m 4096", "-nic none", "node-name=qual-data", "serial=dockpipe-data-000001", "cache.direct=on", "cache.no-flush=off", "aio=threads", "discard=ignore", "config-wce=on", "rerror=stop", "werror=stop"} {
		if !strings.Contains(joined, required) {
			t.Fatalf("QEMU plan missing %q: %s", required, joined)
		}
	}
	for _, forbidden := range []string{"accel=tcg", "-net ", "hostfwd", "virtio-9p", "snapshot=on", "discard=unmap", "detect-zeroes=unmap"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("QEMU plan contains forbidden %q: %s", forbidden, joined)
		}
	}
}

func TestQEMUPlanEnforcesLinuxUnixSocketPathLimit(t *testing.T) {
	m := sample(t)
	agentName := m.RunID + ".agent"
	runtimeLength := LinuxUnixSocketPathMaxBytes - 1 - len(agentName)
	volumeRoot := filepath.VolumeName(t.TempDir()) + string(filepath.Separator)
	runtime := filepath.Join(volumeRoot, strings.Repeat("r", runtimeLength-len(volumeRoot)))
	agentSocket := filepath.Join(runtime, agentName)
	if got := len([]byte(agentSocket)); got != LinuxUnixSocketPathMaxBytes {
		t.Fatalf("test did not construct the exact Linux socket boundary: got %d", got)
	}
	if _, err := PlanQEMU(m, runtime); err != nil {
		t.Fatalf("exact %d-byte Unix socket path was rejected: %v", LinuxUnixSocketPathMaxBytes, err)
	}
	if _, err := PlanQEMU(m, runtime+"r"); err == nil || !strings.Contains(err.Error(), "agent Unix socket path is 108 bytes") {
		t.Fatalf("expected deterministic overlength agent-socket rejection, got %v", err)
	}
}

func TestProvisioningQEMUPlanBindsFreshDisksAndReadOnlySeed(t *testing.T) {
	m := sample(t)
	root := t.TempDir()
	runtimeRoot := shortRuntimeRoot(t, "dp-vm-provisioning")
	plan, err := PlanProvisioningQEMU(m, runtimeRoot, filepath.Join(root, "os.qcow2"), filepath.Join(root, "data.raw"), filepath.Join(root, "seed.iso"), filepath.Join(runtimeRoot, "console.sock"))
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(plan.Args, " ")
	for _, required := range []string{filepath.Join(root, "os.qcow2"), filepath.Join(root, "data.raw"), filepath.Join(root, "seed.iso"), "read-only=on", "scsi-cd", "-nic none", "id=dockpipe-first-boot-console", "server=off", "reconnect-ms=0", "-serial chardev:dockpipe-first-boot-console"} {
		if !strings.Contains(joined, required) {
			t.Fatalf("provisioning QEMU plan missing %q: %s", required, joined)
		}
	}
}

func TestCleanupIsExactAndPreservesCompletedCohorts(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "run-001", "cohort-001")
	layout := TrialLayout{RunID: "run-001", Cohort: "cohort-001", MachineUUID: "55555555-5555-4555-8555-555555555555", InstanceRoot: root, OSDisk: filepath.Join(base, "os.qcow2"), DataDisk: filepath.Join(base, "data.raw"), AttemptRoot: filepath.Join(base, "attempt-001"), BoundaryRoot: filepath.Join(base, "boundary-after-fsync")}
	resources := []string{layout.OSDisk, layout.DataDisk, layout.AttemptRoot, layout.BoundaryRoot}
	if _, err := layout.CleanupPlan("wrong-run", resources); err == nil {
		t.Fatal("expected cleanup identity mismatch")
	}
	if _, err := layout.CleanupPlan(layout.RunID, resources[:3]); err == nil {
		t.Fatal("expected incomplete enumeration rejection")
	}
	if _, err := layout.CleanupPlan(layout.RunID, resources); err != nil {
		t.Fatal(err)
	}
	layout.Completed = true
	if _, err := layout.CleanupPlan(layout.RunID, resources); err == nil {
		t.Fatal("expected completed cohort preservation")
	}
	layout.Completed = false
	layout.Failed = true
	if _, err := layout.CleanupPlan(layout.RunID, resources); err == nil {
		t.Fatal("expected failed cohort preservation")
	}
}
