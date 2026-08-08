package manifest

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// Linux sockaddr_un.sun_path contains 108 bytes. Filesystem socket paths must
// leave one byte for the terminating NUL; QEMU's QMP and chardev sockets are
// pathname sockets rather than abstract sockets.
const LinuxUnixSocketPathMaxBytes = 107

// QEMUPlan is inert: it records the exact reviewed argv and never starts a process.
type QEMUPlan struct {
	Binary string   `json:"binary"`
	Args   []string `json:"args"`
}

// PlanProvisioningQEMU binds the validated VM tuple to the fresh XDG instance
// disks and an immutable read-only NoCloud seed. It remains an inert argv plan.
func PlanProvisioningQEMU(m Manifest, runtimeDir, osDisk, dataDisk, seed, consoleSocket string) (QEMUPlan, error) {
	for label, path := range map[string]string{"OS disk": osDisk, "data disk": dataDisk, "NoCloud seed": seed} {
		if !filepath.IsAbs(path) || strings.ContainsAny(path, ",\r\n") {
			return QEMUPlan{}, fmt.Errorf("%s path must be absolute and QEMU-safe", label)
		}
	}
	if osDisk == dataDisk || osDisk == seed || dataDisk == seed {
		return QEMUPlan{}, fmt.Errorf("OS, data, and NoCloud seed paths must be distinct")
	}
	if err := ValidateLinuxUnixSocketPath("first-boot console", consoleSocket); err != nil {
		return QEMUPlan{}, err
	}
	m.OSDisk.Path = osDisk
	m.DataDisk.Path = dataDisk
	plan, err := PlanQEMU(m, runtimeDir)
	if err != nil {
		return QEMUPlan{}, err
	}
	plan.Args = append(plan.Args,
		"-chardev", "socket,id=dockpipe-first-boot-console,path="+consoleSocket+",server=off,reconnect-ms=0",
		"-serial", "chardev:dockpipe-first-boot-console",
		"-device", "virtio-scsi-pci,id=qual-seed-scsi",
		"-blockdev", "driver=file,node-name=qual-seed-file,filename="+seed+",read-only=on,auto-read-only=off,discard=ignore",
		"-blockdev", "driver=raw,node-name=qual-seed,file=qual-seed-file,read-only=on,discard=ignore",
		"-device", "scsi-cd,drive=qual-seed,bus=qual-seed-scsi.0",
	)
	return plan, nil
}

func ValidateLinuxUnixSocketPath(label, path string) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || strings.ContainsAny(path, ",\r\n") {
		return fmt.Errorf("%s Unix socket path must be absolute, clean, and QEMU-safe", label)
	}
	if len([]byte(path)) > LinuxUnixSocketPathMaxBytes {
		return fmt.Errorf("%s Unix socket path is %d bytes; Linux pathname sockets permit at most %d", label, len([]byte(path)), LinuxUnixSocketPathMaxBytes)
	}
	return nil
}

func (p QEMUPlan) CanonicalJSON() (string, error) {
	b, err := json.Marshal(p)
	return string(b), err
}

func PlanQEMU(m Manifest, runtimeDir string) (QEMUPlan, error) {
	if err := m.Validate(); err != nil {
		return QEMUPlan{}, err
	}
	if !filepath.IsAbs(runtimeDir) || strings.ContainsAny(runtimeDir, ",\r\n") {
		return QEMUPlan{}, fmt.Errorf("runtime directory must be absolute")
	}
	qmpSocket := filepath.Join(runtimeDir, m.RunID+".qmp")
	agentSocket := filepath.Join(runtimeDir, m.RunID+".agent")
	for _, socket := range []struct {
		name string
		path string
	}{
		{"QMP", qmpSocket},
		{"agent", agentSocket},
	} {
		if err := ValidateLinuxUnixSocketPath(socket.name, socket.path); err != nil {
			return QEMUPlan{}, err
		}
	}
	aio := m.QEMU.AIO
	fileOptions := func(node, path string) string {
		return strings.Join([]string{"driver=file", "node-name=" + node + "-file", "filename=" + path, "cache.direct=on", "cache.no-flush=off", "aio=" + aio, "discard=ignore"}, ",")
	}
	rawOptions := func(node, fileNode string) string {
		return strings.Join([]string{"driver=raw", "node-name=" + node, "file=" + fileNode, "discard=ignore"}, ",")
	}
	args := []string{
		"-name", "dockpipe-qualification-" + m.RunID,
		"-machine", "q35,accel=kvm",
		"-cpu", "host",
		"-smp", "2",
		"-m", "4096",
		"-uuid", m.MachineUUID,
		"-nodefaults", "-no-reboot", "-nographic", "-nic", "none",
		"-qmp", "unix:" + qmpSocket + ",server=on,wait=off",
		"-chardev", "socket,id=dockpipe-agent,path=" + agentSocket + ",server=on,wait=off",
		"-device", "virtio-serial-pci",
		"-device", "virtserialport,chardev=dockpipe-agent,name=org.dockpipe.agent.1",
		"-blockdev", fileOptions(m.OSDisk.NodeName, m.OSDisk.Path),
		"-blockdev", "driver=qcow2,node-name=" + m.OSDisk.NodeName + ",file=" + m.OSDisk.NodeName + "-file,discard=ignore",
		"-device", "virtio-blk-pci,drive=" + m.OSDisk.NodeName + ",serial=" + m.OSDisk.Serial + ",config-wce=on,rerror=stop,werror=stop",
		"-blockdev", fileOptions(m.DataDisk.NodeName, m.DataDisk.Path),
		"-blockdev", rawOptions(m.DataDisk.NodeName, m.DataDisk.NodeName+"-file"),
		"-device", "virtio-blk-pci,drive=" + m.DataDisk.NodeName + ",serial=" + m.DataDisk.Serial + ",config-wce=on,rerror=stop,werror=stop",
	}
	return QEMUPlan{Binary: m.QEMU.BinaryPath, Args: args}, nil
}

func MkfsCommand(m Manifest, deviceByID string) ([]string, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}
	if !strings.HasPrefix(deviceByID, "/dev/disk/by-id/") {
		return nil, fmt.Errorf("mkfs target must be a stable /dev/disk/by-id path")
	}
	return []string{"mkfs.ext4", "-F", "-L", m.Filesystem.Label, "-U", m.Filesystem.UUID, "-E", "lazy_itable_init=0,lazy_journal_init=0", deviceByID}, nil
}

func MountCommand(m Manifest, deviceByUUID string) ([]string, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}
	if deviceByUUID != "/dev/disk/by-uuid/"+m.Filesystem.UUID {
		return nil, fmt.Errorf("mount target does not match manifest filesystem UUID")
	}
	return []string{"mount", "-t", "ext4", "-o", strings.Join(RequiredMountOptions, ","), deviceByUUID, QualificationMount}, nil
}
