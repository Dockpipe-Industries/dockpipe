package manifest

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// QEMUPlan is inert: it records the exact reviewed argv and never starts a process.
type QEMUPlan struct {
	Binary string   `json:"binary"`
	Args   []string `json:"args"`
}

// PlanProvisioningQEMU binds the validated VM tuple to the fresh XDG instance
// disks and an immutable read-only NoCloud seed. It remains an inert argv plan.
func PlanProvisioningQEMU(m Manifest, runtimeDir, osDisk, dataDisk, seed string) (QEMUPlan, error) {
	for label, path := range map[string]string{"OS disk": osDisk, "data disk": dataDisk, "NoCloud seed": seed} {
		if !filepath.IsAbs(path) || strings.ContainsAny(path, ",\r\n") {
			return QEMUPlan{}, fmt.Errorf("%s path must be absolute and QEMU-safe", label)
		}
	}
	if osDisk == dataDisk || osDisk == seed || dataDisk == seed {
		return QEMUPlan{}, fmt.Errorf("OS, data, and NoCloud seed paths must be distinct")
	}
	m.OSDisk.Path = osDisk
	m.DataDisk.Path = dataDisk
	plan, err := PlanQEMU(m, runtimeDir)
	if err != nil {
		return QEMUPlan{}, err
	}
	plan.Args = append(plan.Args,
		"-device", "virtio-scsi-pci,id=qual-seed-scsi",
		"-blockdev", "driver=file,node-name=qual-seed-file,filename="+seed+",read-only=on,auto-read-only=off,discard=ignore",
		"-blockdev", "driver=raw,node-name=qual-seed,file=qual-seed-file,read-only=on,discard=ignore",
		"-device", "scsi-cd,drive=qual-seed,bus=qual-seed-scsi.0",
	)
	return plan, nil
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
		"-qmp", "unix:" + filepath.Join(runtimeDir, m.RunID+".qmp") + ",server=on,wait=off",
		"-chardev", "socket,id=dockpipe-agent,path=" + filepath.Join(runtimeDir, m.RunID+".agent") + ",server=on,wait=off",
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
