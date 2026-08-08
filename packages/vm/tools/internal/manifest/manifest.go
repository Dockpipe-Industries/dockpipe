package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

const (
	Schema               = "dockpipe.vm.qualification.v2"
	QualificationPurpose = "qualification"
	QualificationMount   = "/var/lib/dockpipe-qualification"
	KernelBootIDSource   = "/proc/sys/kernel/random/boot_id"
	DataDiskBytes        = int64(4 * 1024 * 1024 * 1024)
	// VirtioBlockSerialMaxBytes is the Linux-visible virtio-blk serial limit.
	VirtioBlockSerialMaxBytes = 20
	UbuntuImageURL       = "https://cloud-images.ubuntu.com/releases/noble/release-20260801/ubuntu-24.04-server-cloudimg-amd64.img"
	UbuntuImageSHA256    = "0533b0655c32e68b31d792ecd6ccfca95abdbc536c4446874fe0513bd4140ffe"
)

var (
	uuidPattern   = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	serialPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{7,19}$`)
	idPattern     = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{2,127}$`)
	shaPattern    = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

var RequiredMountOptions = []string{"rw", "noatime", "nodev", "nosuid", "noexec", "data=ordered"}
var RequiredExt4Features = []string{"has_journal", "ext_attr", "resize_inode", "dir_index", "filetype", "extent", "64bit", "flex_bg", "sparse_super", "large_file", "huge_file", "dir_nlink", "extra_isize", "metadata_csum"}

type Manifest struct {
	Schema                string     `json:"schema"`
	Purpose               string     `json:"purpose"`
	Disposable            bool       `json:"disposable"`
	LiveExecutionApproved bool       `json:"live_execution_approved"`
	RunID                 string     `json:"run_id"`
	Scenario              string     `json:"scenario"`
	DurabilityBoundary    string     `json:"durability_boundary"`
	MachineUUID           string     `json:"machine_uuid"`
	HostMachineUUID       string     `json:"host_machine_uuid"`
	BootIDSource          string     `json:"boot_id_source"`
	Image                 Image      `json:"image"`
	Machine               Machine    `json:"machine"`
	Isolation             Isolation  `json:"isolation"`
	OSDisk                Disk       `json:"os_disk"`
	DataDisk              Disk       `json:"data_disk"`
	Filesystem            Filesystem `json:"filesystem"`
	Evidence              Evidence   `json:"evidence"`
	QEMU                  QEMU       `json:"qemu"`
	Protocol              Protocol   `json:"protocol"`
}

type Image struct {
	URL      string `json:"url"`
	SHA256   string `json:"sha256"`
	Override string `json:"override,omitempty"`
}

type Machine struct {
	Acceleration string `json:"acceleration"`
	CPUModel     string `json:"cpu_model"`
	CPUs         int    `json:"cpus"`
	MemoryMiB    int    `json:"memory_mib"`
	Swap         bool   `json:"swap"`
}

type Isolation struct {
	Network       bool     `json:"network"`
	SSH           bool     `json:"ssh"`
	PhysicalDisks []string `json:"physical_disks"`
	Passthrough   []string `json:"passthrough"`
	Shares        []string `json:"shares"`
	ExtraDisks    []string `json:"extra_disks"`
	ArbitraryExec bool     `json:"arbitrary_exec"`
}

type Disk struct {
	Path                string `json:"path"`
	Format              string `json:"format"`
	Bytes               int64  `json:"bytes"`
	Sparse              bool   `json:"sparse"`
	Private             bool   `json:"private"`
	Serial              string `json:"serial"`
	NodeName            string `json:"node_name"`
	HostBackingIdentity string `json:"host_backing_identity"`
}

type Filesystem struct {
	Type             string   `json:"type"`
	WholeDevice      bool     `json:"whole_device"`
	UUID             string   `json:"uuid"`
	Label            string   `json:"label"`
	Mount            string   `json:"mount"`
	MountOptions     []string `json:"mount_options"`
	Features         []string `json:"features"`
	LazyITableInit   bool     `json:"lazy_itable_init"`
	LazyJournalInit  bool     `json:"lazy_journal_init"`
	Encrypted        bool     `json:"encrypted"`
	NestedFilesystem bool     `json:"nested_filesystem"`
	Overlay          bool     `json:"overlay"`
}

type QEMU struct {
	BinaryPath          string `json:"binary_path"`
	BinarySHA256        string `json:"binary_sha256"`
	Version             string `json:"version"`
	ConfigurationSHA256 string `json:"configuration_sha256"`
	Cache               string `json:"cache"`
	AIO                 string `json:"aio"`
	ReviewedNativeAIO   bool   `json:"reviewed_native_aio"`
	GuestWriteCache     bool   `json:"guest_write_cache"`
	Discard             bool   `json:"discard"`
	DetectZeroes        bool   `json:"detect_zeroes"`
	Snapshot            bool   `json:"snapshot"`
	ReadError           string `json:"read_error"`
	WriteError          string `json:"write_error"`
}

type Evidence struct {
	MkfsVersion        string `json:"mkfs_version"`
	HostKernelRelease  string `json:"host_kernel_release"`
	GuestKernelRelease string `json:"guest_kernel_release"`
	MountID            uint64 `json:"mount_id"`
}

type Protocol struct {
	ControllerPublicKeySHA256 string   `json:"controller_public_key_sha256"`
	GuestPublicKeySHA256      string   `json:"guest_public_key_sha256"`
	ControllerBinarySHA256    string   `json:"controller_binary_sha256"`
	GuestAgentBinarySHA256    string   `json:"guest_agent_binary_sha256"`
	HarnessSHA256             string   `json:"harness_sha256"`
	Capabilities              []string `json:"capabilities"`
}

func Load(path string) (Manifest, error) {
	var out Manifest
	b, err := os.ReadFile(path)
	if err != nil {
		return out, err
	}
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&out); err != nil {
		return out, fmt.Errorf("decode qualification manifest: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return out, fmt.Errorf("qualification manifest contains trailing JSON")
	}
	return out, out.Validate()
}

func (m Manifest) Validate() error {
	if m.Schema != Schema || m.Purpose != QualificationPurpose || !m.Disposable {
		return fmt.Errorf("qualification manifest must use %s, purpose qualification, and disposable=true", Schema)
	}
	if m.LiveExecutionApproved {
		return fmt.Errorf("live qualification execution is unavailable in this foundation slice")
	}
	for label, value := range map[string]string{"run_id": m.RunID, "scenario": m.Scenario, "durability_boundary": m.DurabilityBoundary} {
		if !idPattern.MatchString(value) {
			return fmt.Errorf("%s is not a stable identifier", label)
		}
	}
	if !uuidPattern.MatchString(m.MachineUUID) || !uuidPattern.MatchString(m.HostMachineUUID) || m.MachineUUID == m.HostMachineUUID || m.BootIDSource != KernelBootIDSource {
		return fmt.Errorf("machine and host identities must be exact, distinct UUIDs and boot ID must come from %s", KernelBootIDSource)
	}
	if err := validateImage(m.Image); err != nil {
		return err
	}
	if m.Machine.Acceleration != "kvm" || m.Machine.CPUModel != "host" || m.Machine.CPUs != 2 || m.Machine.MemoryMiB != 4096 || m.Machine.Swap {
		return fmt.Errorf("qualification machine tuple must be KVM, host CPU, 2 CPUs, 4096 MiB, and no swap")
	}
	if m.Isolation.Network || m.Isolation.SSH || m.Isolation.ArbitraryExec || len(m.Isolation.PhysicalDisks)+len(m.Isolation.Passthrough)+len(m.Isolation.Shares)+len(m.Isolation.ExtraDisks) != 0 {
		return fmt.Errorf("qualification isolation prohibits network, SSH, exec, physical disks, passthrough, shares, and extra disks")
	}
	if err := validateDisks(m.OSDisk, m.DataDisk); err != nil {
		return err
	}
	if err := validateFilesystem(m.Filesystem); err != nil {
		return err
	}
	if strings.TrimSpace(m.Evidence.MkfsVersion) == "" || strings.TrimSpace(m.Evidence.HostKernelRelease) == "" || strings.TrimSpace(m.Evidence.GuestKernelRelease) == "" || m.Evidence.MountID == 0 {
		return fmt.Errorf("mkfs version, host and guest kernels, and mount ID must be recorded")
	}
	if err := validateQEMU(m.QEMU); err != nil {
		return err
	}
	if err := validateProtocol(m.Protocol); err != nil {
		return err
	}
	return nil
}

func validateImage(image Image) error {
	if image.Override == "" {
		if image.URL != UbuntuImageURL || image.SHA256 != UbuntuImageSHA256 {
			return fmt.Errorf("default image must match the pinned Ubuntu release URL and checksum")
		}
		return nil
	}
	if !filepath.IsAbs(image.Override) || !shaPattern.MatchString(image.SHA256) {
		return fmt.Errorf("image override requires an absolute path and explicit SHA-256")
	}
	return nil
}

func validateDisks(osDisk, dataDisk Disk) error {
	if !filepath.IsAbs(osDisk.Path) || !filepath.IsAbs(dataDisk.Path) || filepath.Clean(osDisk.Path) == filepath.Clean(dataDisk.Path) || strings.ContainsAny(osDisk.Path+dataDisk.Path, ",\r\n") {
		return fmt.Errorf("qualification requires distinct absolute OS and data disk paths")
	}
	if !osDisk.Private || osDisk.Format != "qcow2" || osDisk.Bytes <= 0 || !serialPattern.MatchString(osDisk.Serial) || osDisk.NodeName == "" || osDisk.HostBackingIdentity == "" {
		return fmt.Errorf("OS disk must be a private persistent qcow2 clone with recorded identity")
	}
	if !dataDisk.Private || !dataDisk.Sparse || dataDisk.Format != "raw" || dataDisk.Bytes != DataDiskBytes || !serialPattern.MatchString(dataDisk.Serial) || dataDisk.NodeName == "" || dataDisk.HostBackingIdentity == "" {
		return fmt.Errorf("data disk must be a private 4 GiB sparse raw disk with stable node, serial, and host identity")
	}
	if osDisk.NodeName == dataDisk.NodeName || osDisk.HostBackingIdentity == dataDisk.HostBackingIdentity {
		return fmt.Errorf("OS and data disk identities must be distinct")
	}
	return nil
}

func validateFilesystem(fs Filesystem) error {
	if fs.Type != "ext4" || !fs.WholeDevice || !uuidPattern.MatchString(fs.UUID) || fs.Label != "dockpipe-qual" || fs.Mount != QualificationMount {
		return fmt.Errorf("qualification filesystem must be whole-device ext4 with stable UUID, label, and mount")
	}
	if !slices.Equal(fs.MountOptions, RequiredMountOptions) || !slices.Equal(fs.Features, RequiredExt4Features) {
		return fmt.Errorf("qualification filesystem options or features differ from the reviewed tuple")
	}
	if fs.LazyITableInit || fs.LazyJournalInit || fs.Encrypted || fs.NestedFilesystem || fs.Overlay {
		return fmt.Errorf("lazy initialization, encryption, nesting, and overlays are prohibited")
	}
	for _, forbidden := range []string{"discard", "dax", "nobarrier", "remount", "bind"} {
		if slices.Contains(fs.MountOptions, forbidden) {
			return fmt.Errorf("forbidden mount option %q", forbidden)
		}
	}
	return nil
}

func validateQEMU(q QEMU) error {
	if !filepath.IsAbs(q.BinaryPath) || !shaPattern.MatchString(q.BinarySHA256) || !shaPattern.MatchString(q.ConfigurationSHA256) || strings.TrimSpace(q.Version) == "" {
		return fmt.Errorf("QEMU binary path, version, binary hash, and configuration hash are required")
	}
	if strings.Contains(q.Version, "6.2") {
		return fmt.Errorf("QEMU 6.2 is not qualified by this contract")
	}
	if q.Cache != "none" || q.GuestWriteCache != true || q.Discard || q.DetectZeroes || q.Snapshot || q.ReadError != "stop" || q.WriteError != "stop" {
		return fmt.Errorf("QEMU block behavior differs from the reviewed no-fallback tuple")
	}
	if q.AIO != "threads" && !(q.AIO == "native" && q.ReviewedNativeAIO) {
		return fmt.Errorf("AIO must be threads or an explicitly reviewed native tuple")
	}
	return nil
}

func validateProtocol(p Protocol) error {
	for _, sum := range []string{p.ControllerPublicKeySHA256, p.GuestPublicKeySHA256, p.ControllerBinarySHA256, p.GuestAgentBinarySHA256, p.HarnessSHA256} {
		if !shaPattern.MatchString(sum) {
			return fmt.Errorf("protocol keys and harness require SHA-256 pins")
		}
	}
	want := []string{"identity/v1", "health/v1", "checkpoint/v1", "recovery/v1", "launch-hash-pinned/v1"}
	if !slices.Equal(p.Capabilities, want) {
		return fmt.Errorf("qualification capability set must match the reviewed versioned list")
	}
	return nil
}

func ConfigurationSHA256(m Manifest) (string, error) {
	m.QEMU.ConfigurationSHA256 = strings.Repeat("0", 64)
	b, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
