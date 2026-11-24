package application

import (
	"os"
	"path/filepath"
	"testing"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/model/managedenvironment"
)

func TestVMEnvironmentPresetGeneratesEditableHardware(t *testing.T) {
	path := filepath.Join(t.TempDir(), "development.yaml")
	err := cmdEnvironment([]string{"generate", "--name", "development", "--kind", "vm", "--preset",
		"debian", "--cpus", "6", "--disk-gib", "96", "--installer-iso", "media/debian.iso", "--file", path}, nil)
	if err != nil {
		t.Fatal(err)
	}

	definition, err := readEnvironment(path)
	if err != nil {
		t.Fatal(err)
	}
	expected := managedenvironment.VM{OS: "debian", CPUs: 6, MemoryGiB: 2, DiskGiB: 96, InstallerISO: "media/debian.iso"}
	if definition.Provision != nil || definition.VM == nil || *definition.VM != expected {
		t.Fatalf("expected a standalone VM definition with overridden hardware, got %+v", definition.VM)
	}
}

func TestVMEnvironmentRejectsInvalidSettingsBeforeSaving(t *testing.T) {
	for _, settings := range [][]string{
		{"--preset", "unknown"}, {"--os", "ubuntu", "--cpus", "0"}, {"--os", "windows11", "--disk-gib", "32"},
		{"--preset", "ubuntu", "--workflow", "manual"}, {"--preset", "ubuntu", "--image", "alpine"},
		{"--preset", "ubuntu", "--installer-iso", "disk.qcow2"},
	} {
		path := filepath.Join(t.TempDir(), "invalid.yaml")
		args := append([]string{"generate", "--name", "development", "--kind", "vm", "--file", path}, settings...)
		if err := cmdEnvironment(args, nil); err == nil {
			t.Errorf("accepted invalid options %v", settings)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("invalid options wrote a definition: %v", err)
		}
	}
}

func TestVMEnvironmentPassesHardwareAndKeepsHostFoldersUnshared(t *testing.T) {
	settings := managedenvironment.VM{OS: "windows11", CPUs: 6, MemoryGiB: 12, DiskGiB: 100}
	variables := domain.EnvSliceToMap(vmEnvironmentVariables("/owned/disk.qcow2", "/media/Windows 11.iso", settings))
	for key, value := range map[string]string{
		"DOCKPIPE_VM_BOOT_SOURCE": "installer-iso", "DOCKPIPE_VM_CDROM": "/media/Windows 11.iso",
		"DOCKPIPE_VM_CPUS": "6", "DOCKPIPE_VM_MEMORY": "12G", "DOCKPIPE_VM_DISK_SIZE": "100G",
		"DOCKPIPE_VM_DISK": "/owned/disk.qcow2", "DOCKPIPE_VM_MOUNTS": "", "DOCKPIPE_VM_SYNC_HOST_PATH": "",
		"DOCKPIPE_VM_TPM": "required", "DOCKPIPE_VM_SECURE_BOOT": "required",
	} {
		if got, exists := variables[key]; !exists || got != value {
			t.Errorf("%s = %q, want %q", key, got, value)
		}
	}
}
