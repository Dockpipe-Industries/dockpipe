package provisioning

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
	"syscall"
)

var toolchainBundleVersionPattern = regexp.MustCompile(`^11\.0\.3-linux-amd64\.[1-9][0-9]*$`)

const (
	ToolchainSchema       = "dockpipe.vm.toolchain.v1"
	ToolchainBundleID     = "dockpipe-vm-qemu-linux-amd64"
	ToolchainQEMUVersion  = "11.0.3"
	ToolchainSourceURL    = "https://download.qemu.org/qemu-11.0.3.tar.xz"
	ToolchainSignatureURL = "https://download.qemu.org/qemu-11.0.3.tar.xz.sig"
	ToolchainSigner       = "CEACC9E15534EBABB82D3FA03353C9CEF108B584"
	ToolQEMUSystem        = "qemu-system-x86_64"
	ToolQEMUImage         = "qemu-img"
	ToolchainManifestName = "toolchain.json"
)

type ToolchainReference struct {
	Root           string `json:"root"`
	Manifest       string `json:"manifest"`
	ManifestSHA256 string `json:"manifest_sha256"`
}

type ToolchainManifest struct {
	Schema            string          `json:"schema"`
	BundleID          string          `json:"bundle_id"`
	BundleVersion     string          `json:"bundle_version"`
	OS                string          `json:"os"`
	Architecture      string          `json:"architecture"`
	QEMUVersion       string          `json:"qemu_version"`
	Source            ToolchainSource `json:"source"`
	BuildRecipeSHA256 string          `json:"build_recipe_sha256"`
	Tools             []ToolPin       `json:"tools"`
	RuntimeFiles      []FilePin       `json:"runtime_files"`
}

type ToolchainSource struct {
	URL               string `json:"url"`
	SignatureURL      string `json:"signature_url"`
	ArchiveSHA256     string `json:"archive_sha256"`
	SignerFingerprint string `json:"signer_fingerprint"`
}

type ToolPin struct {
	ID           string `json:"id"`
	RelativePath string `json:"relative_path"`
	SHA256       string `json:"sha256"`
	Version      string `json:"version"`
	Mode         uint32 `json:"mode"`
}

type FilePin struct {
	RelativePath string `json:"relative_path"`
	SHA256       string `json:"sha256"`
	Mode         uint32 `json:"mode"`
}

func LoadToolchain(ref ToolchainReference) (ToolchainManifest, error) {
	var out ToolchainManifest
	if !filepath.IsAbs(ref.Root) || !filepath.IsAbs(ref.Manifest) || filepath.Clean(ref.Manifest) != filepath.Join(filepath.Clean(ref.Root), ToolchainManifestName) {
		return out, fmt.Errorf("toolchain root and manifest must be absolute and exact")
	}
	info, err := os.Lstat(ref.Manifest)
	if err != nil {
		return out, fmt.Errorf("inspect toolchain manifest: %w", err)
	}
	if err := validateOwnedRegular(info, 0o400, false); err != nil {
		return out, fmt.Errorf("toolchain manifest: %w", err)
	}
	b, err := os.ReadFile(ref.Manifest)
	if err != nil {
		return out, err
	}
	sum := sha256.Sum256(b)
	if hex.EncodeToString(sum[:]) != ref.ManifestSHA256 {
		return out, fmt.Errorf("toolchain manifest SHA-256 mismatch")
	}
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&out); err != nil {
		return out, fmt.Errorf("decode toolchain manifest: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return out, fmt.Errorf("toolchain manifest contains trailing JSON")
	}
	return out, nil
}

func (m ToolchainManifest) Validate(ref ToolchainReference, checkoutRoot string, generated Roots) error {
	if m.Schema != ToolchainSchema || m.BundleID != ToolchainBundleID || m.OS != "linux" || m.Architecture != "amd64" || m.QEMUVersion != ToolchainQEMUVersion || !toolchainBundleVersionPattern.MatchString(m.BundleVersion) {
		return fmt.Errorf("toolchain must be the exact versioned Linux/amd64 QEMU %s bundle", ToolchainQEMUVersion)
	}
	if m.Source.URL != ToolchainSourceURL || m.Source.SignatureURL != ToolchainSignatureURL || m.Source.SignerFingerprint != ToolchainSigner || !shaPattern.MatchString(m.Source.ArchiveSHA256) || !shaPattern.MatchString(m.BuildRecipeSHA256) {
		return fmt.Errorf("toolchain source archive, signature, signer, and build recipe must be hash-pinned")
	}
	root := filepath.Clean(ref.Root)
	if containsPathSegment(root, ".dockpipe") || containsPathSegment(root, ".dorkpipe") || checkoutRoot != "" && pathWithin(root, checkoutRoot) {
		return fmt.Errorf("task-owned toolchain root must be outside checkout and generated stores")
	}
	for _, forbidden := range []string{generated.Instances, generated.Evidence, generated.Config, generated.Runtime} {
		if forbidden != "" && (pathWithin(root, forbidden) || pathWithin(forbidden, root)) {
			return fmt.Errorf("toolchain root must not overlap VM instance, evidence, configuration, or runtime roots")
		}
	}
	rootInfo, err := os.Lstat(root)
	if err != nil {
		return fmt.Errorf("inspect toolchain root: %w", err)
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() || rootInfo.Mode().Perm() != 0o500 || !ownedByCurrentUser(rootInfo) {
		return fmt.Errorf("toolchain root must be an immutable owner-only non-symlink directory")
	}
	if len(m.Tools) != 2 || len(m.RuntimeFiles) == 0 {
		return fmt.Errorf("toolchain must contain exactly two tools and an explicit non-empty runtime closure")
	}
	wantIDs := []string{ToolQEMUImage, ToolQEMUSystem}
	gotIDs := make([]string, 0, len(m.Tools))
	listed := map[string]struct{}{ToolchainManifestName: {}}
	for _, tool := range m.Tools {
		if err := validateBundlePath(tool.RelativePath); err != nil || !shaPattern.MatchString(tool.SHA256) || tool.Mode != 0o500 {
			return fmt.Errorf("invalid hash-pinned executable %q", tool.ID)
		}
		wantVersion := "qemu-img version " + ToolchainQEMUVersion
		wantPath := "bin/" + ToolQEMUImage
		if tool.ID == ToolQEMUSystem {
			wantVersion = "QEMU emulator version " + ToolchainQEMUVersion
			wantPath = "bin/" + ToolQEMUSystem
		}
		if tool.Version != wantVersion || tool.RelativePath != wantPath {
			return fmt.Errorf("tool %q path or version output differs from the reviewed tuple", tool.ID)
		}
		gotIDs = append(gotIDs, tool.ID)
		if _, duplicate := listed[tool.RelativePath]; duplicate {
			return fmt.Errorf("duplicate toolchain path %q", tool.RelativePath)
		}
		listed[tool.RelativePath] = struct{}{}
		if err := validatePinnedBundleFile(root, tool.RelativePath, tool.SHA256, os.FileMode(tool.Mode), true); err != nil {
			return err
		}
	}
	slices.Sort(gotIDs)
	if !slices.Equal(gotIDs, wantIDs) {
		return fmt.Errorf("toolchain executable set must be exactly qemu-img and qemu-system-x86_64")
	}
	for _, file := range m.RuntimeFiles {
		if err := validateBundlePath(file.RelativePath); err != nil || !shaPattern.MatchString(file.SHA256) || file.Mode != 0o400 && file.Mode != 0o500 {
			return fmt.Errorf("invalid runtime-closure file %q", file.RelativePath)
		}
		if _, duplicate := listed[file.RelativePath]; duplicate {
			return fmt.Errorf("duplicate toolchain path %q", file.RelativePath)
		}
		listed[file.RelativePath] = struct{}{}
		if err := validatePinnedBundleFile(root, file.RelativePath, file.SHA256, os.FileMode(file.Mode), file.Mode == 0o500); err != nil {
			return err
		}
	}
	return validateExactBundleInventory(root, listed)
}

func (m ToolchainManifest) Tool(id string) (ToolPin, error) {
	for _, tool := range m.Tools {
		if tool.ID == id {
			return tool, nil
		}
	}
	return ToolPin{}, fmt.Errorf("required tool %q is absent", id)
}

func validatePinnedBundleFile(root, relative, want string, mode os.FileMode, executable bool) error {
	path := filepath.Join(root, filepath.FromSlash(relative))
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect toolchain file %s: %w", relative, err)
	}
	if err := validateOwnedRegular(info, mode, executable); err != nil {
		return fmt.Errorf("toolchain file %s: %w", relative, err)
	}
	digest, err := (OSImageInspector{}).SHA256(path)
	if err != nil || digest != want {
		return fmt.Errorf("toolchain file %s SHA-256 mismatch", relative)
	}
	return nil
}

func validateOwnedRegular(info os.FileInfo, mode os.FileMode, executable bool) error {
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Mode().Perm() != mode || executable && info.Mode().Perm()&0o100 == 0 || !ownedByCurrentUser(info) {
		return fmt.Errorf("must be an exact-mode current-user-owned regular non-symlink file")
	}
	return nil
}

func ownedByCurrentUser(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return !ok || int(stat.Uid) == os.Geteuid()
}

func validateBundlePath(relative string) error {
	if relative == "" || filepath.IsAbs(relative) || filepath.Clean(relative) != filepath.FromSlash(relative) || relative == "." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || strings.ContainsAny(relative, "\r\n") {
		return fmt.Errorf("bundle path must be a clean relative path")
	}
	return nil
}

func validateExactBundleInventory(root string, listed map[string]struct{}) error {
	seen := map[string]struct{}{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("toolchain inventory contains symlink %s", path)
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			if info.Mode().Perm() != 0o500 || !ownedByCurrentUser(info) {
				return fmt.Errorf("toolchain directory %s is not immutable and owner-only", rel)
			}
			return nil
		}
		if _, ok := listed[rel]; !ok {
			return fmt.Errorf("unmanifested toolchain file %s", rel)
		}
		seen[rel] = struct{}{}
		return nil
	})
	if err != nil {
		return err
	}
	if len(seen) != len(listed) {
		return fmt.Errorf("toolchain inventory is incomplete")
	}
	return nil
}
