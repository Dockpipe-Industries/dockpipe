package provisioning

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"dockpipe.vm/tools/internal/manifest"
)

type RenderMaterial struct {
	Keys             KeyMaterial
	ControllerBinary []byte
	GuestAgentBinary []byte
}

type RenderedFile struct {
	Name    string `json:"name"`
	Mode    uint32 `json:"mode"`
	SHA256  string `json:"sha256"`
	Content []byte `json:"-"`
}

type AgentConfig struct {
	Schema                    string `json:"schema"`
	ControllerPublicKeyPath   string `json:"controller_public_key_path"`
	ControllerPublicKeySHA256 string `json:"controller_public_key_sha256"`
	GuestPrivateKeyPath       string `json:"guest_private_key_path"`
	GuestPublicKeySHA256      string `json:"guest_public_key_sha256"`
	ControllerBinarySHA256    string `json:"controller_binary_sha256"`
	GuestAgentBinarySHA256    string `json:"guest_agent_binary_sha256"`
	MachineUUID               string `json:"machine_uuid"`
	DiskSerial                string `json:"disk_serial"`
	RunID                     string `json:"run_id"`
	Scenario                  string `json:"scenario"`
	DurabilityBoundary        string `json:"durability_boundary"`
}

var reviewedAssetSHA256 = map[string]string{
	"nocloud/meta-data":               "e1e3a5697cd018d2465093e7011a60dfa17de51ade7a702252d887fada833ef7",
	"nocloud/network-config":          "639b6f419a9ac49312b218e12395dc7e7d623d96202c3315a92dcd19d6fa02ba",
	"nocloud/user-data":               "0415a78cb017cba0a05d45a5592a83a1bd33cc088123b64794219ccd4158184f",
	"systemd/dockpipe-agent.service":  "34bc05b718928c3d042210767b98f527ab9ce77271c4472e90e4481326dcb339",
	"systemd/dockpipe-agent.sysusers": "918c4529043c930ec81256f8c72c915f460283ce33a1d75712bf44aecfa1e5c9",
	"systemd/dockpipe-agent.tmpfiles": "fd07f8893e38df78e8c6b1cd2745bafc9f3a3634bb7685e004c8b0d8ff5b7a91",
}

// ValidateReviewedAssets binds planning to the exact package-owned NoCloud and
// systemd source bytes compiled into this controller version.
func ValidateReviewedAssets(root string) error {
	_, err := loadReviewedAssets(root)
	return err
}

func loadReviewedAssets(root string) (map[string][]byte, error) {
	if !filepath.IsAbs(root) {
		return nil, fmt.Errorf("reviewed assets root must be absolute")
	}
	assets := make(map[string][]byte, len(reviewedAssetSHA256))
	for relative, want := range reviewedAssetSHA256 {
		path := filepath.Join(root, filepath.FromSlash(relative))
		info, err := os.Lstat(path)
		if err != nil {
			return nil, fmt.Errorf("inspect reviewed asset %s: %w", relative, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return nil, fmt.Errorf("reviewed asset %s must be a regular non-symlink file", relative)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read reviewed asset %s: %w", relative, err)
		}
		if hash(data) != want {
			return nil, fmt.Errorf("reviewed asset %s SHA-256 mismatch", relative)
		}
		assets[relative] = data
	}
	return assets, nil
}

func RenderNoCloud(c Contract, m manifest.Manifest, material RenderMaterial) ([]RenderedFile, error) {
	if err := validateContractIdentities(c); err != nil {
		return nil, err
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	if c.MachineUUID != m.MachineUUID || c.RunID != m.RunID || c.DiskSerial != m.DataDisk.Serial || c.FilesystemUUID != m.Filesystem.UUID {
		return nil, fmt.Errorf("render identities must exactly match the qualification manifest")
	}
	if err := validateMaterial(c, material); err != nil {
		return nil, err
	}
	reviewed, err := loadReviewedAssets(c.Artifacts.AssetsRoot)
	if err != nil {
		return nil, err
	}
	meta := reviewed["nocloud/meta-data"]
	network := reviewed["nocloud/network-config"]
	userTemplate := reviewed["nocloud/user-data"]
	assets := []struct {
		relative string
		target   string
		owner    string
		mode     string
		data     []byte
	}{
		{"", "/usr/libexec/dockpipe-guest-agent", "root:root", "0755", material.GuestAgentBinary},
		{"systemd/dockpipe-agent.service", "/etc/systemd/system/dockpipe-agent.service", "root:root", "0644", nil},
		{"systemd/dockpipe-agent.sysusers", "/usr/lib/sysusers.d/dockpipe-agent.conf", "root:root", "0644", nil},
		{"systemd/dockpipe-agent.tmpfiles", "/usr/lib/tmpfiles.d/dockpipe-agent.conf", "root:root", "0644", nil},
		{"", "/etc/dockpipe-agent/controller.pub", "dockpipe-agent:dockpipe-agent", "0400", material.Keys.ControllerPublic},
		{"", "/etc/dockpipe-agent/guest.key", "dockpipe-agent:dockpipe-agent", "0400", material.Keys.GuestPrivate},
	}
	for i := range assets {
		if assets[i].relative != "" {
			assets[i].data = reviewed[assets[i].relative]
		}
	}
	serviceText := string(assets[1].data)
	for _, required := range []string{"User=dockpipe-agent", "PrivateNetwork=yes", "NoNewPrivileges=yes", "ProtectSystem=strict", "CapabilityBoundingSet=", "AmbientCapabilities=", "RestrictAddressFamilies=AF_UNIX", "--serve-virtio-serial=/dev/virtio-ports/org.dockpipe.agent.1"} {
		if !strings.Contains(serviceText, required) {
			return nil, fmt.Errorf("reviewed systemd sandbox is missing %q", required)
		}
	}
	config := AgentConfig{
		Schema: "dockpipe.vm.guest-agent-config.v1", ControllerPublicKeyPath: "/etc/dockpipe-agent/controller.pub",
		ControllerPublicKeySHA256: c.Artifacts.ControllerPublicKeySHA256, GuestPrivateKeyPath: "/etc/dockpipe-agent/guest.key",
		GuestPublicKeySHA256: c.Artifacts.GuestPublicKeySHA256, ControllerBinarySHA256: c.Artifacts.ControllerBinarySHA256, GuestAgentBinarySHA256: c.Artifacts.GuestAgentBinarySHA256,
		MachineUUID: c.MachineUUID, DiskSerial: c.DiskSerial, RunID: c.RunID, Scenario: m.Scenario, DurabilityBoundary: m.DurabilityBoundary,
	}
	configJSON, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}
	assets = append(assets, struct {
		relative, target, owner, mode string
		data                          []byte
	}{"", "/etc/dockpipe-agent/config.json", "dockpipe-agent:dockpipe-agent", "0400", configJSON})
	var writeFiles strings.Builder
	for _, asset := range assets {
		fmt.Fprintf(&writeFiles, "  - path: %s\n    owner: %s\n    permissions: \"%s\"\n    encoding: b64\n    content: %s\n", asset.target, asset.owner, asset.mode, base64.StdEncoding.EncodeToString(asset.data))
	}
	replacements := map[string]string{
		"@@RUN_ID@@": c.RunID, "@@COHORT_ID@@": c.CohortID, "@@DISK_SERIAL@@": c.DiskSerial,
		"@@FILESYSTEM_UUID@@": c.FilesystemUUID, "@@WRITE_FILES@@": strings.TrimSuffix(writeFiles.String(), "\n"),
	}
	meta = replaceExact(meta, replacements)
	userData := replaceExact(userTemplate, replacements)
	joined := string(userData)
	for _, forbidden := range []string{"ssh_authorized_keys: [\"", "package_update: true", "package_upgrade: true", "packages:\n  -", "exec/v1"} {
		if strings.Contains(joined, forbidden) {
			return nil, fmt.Errorf("rendered NoCloud content contains prohibited surface %q", forbidden)
		}
	}
	if strings.Contains(joined, "@@") || strings.Contains(string(meta), "@@") {
		return nil, fmt.Errorf("unresolved NoCloud template marker")
	}
	files := []RenderedFile{{Name: "meta-data", Mode: 0o600, Content: meta}, {Name: "network-config", Mode: 0o600, Content: network}, {Name: "user-data", Mode: 0o600, Content: userData}}
	for i := range files {
		sum := sha256.Sum256(files[i].Content)
		files[i].SHA256 = hex.EncodeToString(sum[:])
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	return files, nil
}

func validateMaterial(c Contract, material RenderMaterial) error {
	if err := validateKeyMaterial(c, material.Keys); err != nil {
		return err
	}
	if hash(material.ControllerBinary) != c.Artifacts.ControllerBinarySHA256 || hash(material.GuestAgentBinary) != c.Artifacts.GuestAgentBinarySHA256 {
		return fmt.Errorf("binary hash pin mismatch")
	}
	return nil
}

func hash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func replaceExact(data []byte, replacements map[string]string) []byte {
	out := string(data)
	keys := make([]string, 0, len(replacements))
	for key := range replacements {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		out = strings.ReplaceAll(out, key, replacements[key])
	}
	return []byte(out)
}
