//go:build linux

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

const hostFolderSchema = "dockpipe.synology_host_folder.v1"
const hostFoldersSchema = "dockpipe.synology_host_folders.v2"
const maximumHostFolders = 16
const maximumHostFolderBytes = 64 << 10
const maximumHostFolderNameBytes = 128
const maximumComposeBytes = 64 << 10

// Mount-table diagnostics have a 4 MiB aggregate budget. Exhaustion rejects the
// selection rather than silently missing an existing nested mount.
const maximumMountInfoBytes = 4 << 20

var hostFolderVolume = regexp.MustCompile(`^volume[1-9][0-9]*$`)

type hostFolder struct {
	Schema string `json:"schema,omitempty"` // Read only for the legacy v1 manifest.
	Source string `json:"source,omitempty"`
	Name   string `json:"name,omitempty"`
	Access string `json:"access,omitempty"`
}

func validateHostFolder(folder hostFolder) error {
	if folder.Schema != hostFolderSchema {
		return errors.New("saved host folder settings are invalid")
	}
	if folder.Source == "" && folder.Access == "" {
		return nil
	}
	if folder.Access != "ro" && folder.Access != "rw" {
		return errors.New("choose read-only or read/write host folder access")
	}
	if len(folder.Source) > 4096 || filepath.Clean(folder.Source) != folder.Source || strings.ContainsAny(folder.Source, "\x00\r\n") {
		return errors.New("choose an absolute folder path inside a DSM shared folder")
	}
	parts := strings.Split(folder.Source, "/")
	if len(parts) < 3 || parts[0] != "" || !hostFolderVolume.MatchString(parts[1]) {
		return errors.New("choose a folder inside /volumeN/shared-folder; volume roots cannot be mounted")
	}
	for _, part := range parts[2:] {
		if part == "" || strings.HasPrefix(part, "@") || part == "." || part == ".." {
			return errors.New("system storage and invalid path components cannot be mounted")
		}
		for _, character := range part {
			if character < 32 || character == 127 {
				return errors.New("host folder paths cannot contain control characters")
			}
		}
	}
	return nil
}

// Walk with directory descriptors so no symlink in the selected path is followed.
// Permission checks use the actual package identity, including its DSM ACL access.
// No ownership or ACLs on the selected folder are changed.
func checkHostFolderAccess(path, access string) error {
	descriptor, err := unix.Open("/", unix.O_PATH|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return errors.New("cannot check host folder access")
	}
	defer func() { _ = unix.Close(descriptor) }()
	for _, part := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		next, err := unix.Openat(descriptor, part, unix.O_PATH|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		if err != nil {
			return errors.New("host folder must exist, contain no symbolic links, and be readable by the dockpipe-desktop system user")
		}
		if err := unix.Close(descriptor); err != nil {
			_ = unix.Close(next)
			return errors.New("cannot close host folder traversal descriptor")
		}
		descriptor = next
	}
	mode := uint32(unix.R_OK | unix.X_OK)
	if access == "rw" {
		mode |= unix.W_OK
	}
	// Real and effective package UIDs match. flags=0 uses the kernel ACL
	// check even on DSM kernels without faccessat2; AT_EACCESS can fall back
	// to a userspace mode-bit approximation on those kernels.
	if err := unix.Faccessat(descriptor, ".", mode, 0); err != nil {
		return errors.New("allow the requested folder permissions for the dockpipe-desktop system user in DSM Shared Folder settings")
	}
	return nil
}

func rejectNestedHostMounts(source string, table []byte) error {
	if len(table) == 0 || len(table) > maximumMountInfoBytes {
		return errors.New("host mount table is unavailable or exceeds its size limit")
	}
	unescape := strings.NewReplacer(`\040`, " ", `\011`, "\t", `\012`, "\n", `\134`, `\`)
	for _, line := range bytes.Split(table, []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		fields := strings.Fields(string(line))
		if len(fields) < 10 {
			return errors.New("host mount table is invalid")
		}
		mount := unescape.Replace(fields[4])
		if !filepath.IsAbs(mount) || filepath.Clean(mount) != mount {
			return errors.New("host mount table contains an invalid path")
		}
		if strings.HasPrefix(mount, source+"/") {
			return errors.New("choose a host folder without nested mounts; older DSM kernels cannot guarantee read-only access to them")
		}
	}
	return nil
}

func checkHostFolderMounts(source string) error {
	file, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return errors.New("cannot inspect existing host mounts")
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maximumMountInfoBytes+1))
	if err != nil {
		return errors.New("cannot inspect existing host mounts")
	}
	return rejectNestedHostMounts(source, data)
}

type hostFolders struct {
	Schema  string       `json:"schema"`
	Folders []hostFolder `json:"folders"`
}

func emptyHostFolders() hostFolders {
	return hostFolders{Schema: hostFoldersSchema, Folders: []hostFolder{}}
}

func decodeHostFolderJSON(data []byte, output any) error {
	if len(data) > maximumHostFolderBytes {
		return errors.New("host folder settings exceed the 64 KiB limit")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(output) != nil || decoder.Decode(new(any)) != io.EOF {
		return errors.New("host folder settings are invalid")
	}
	return nil
}

func normalizeHostFolders(configuration hostFolders) (hostFolders, error) {
	if configuration.Schema != hostFoldersSchema || len(configuration.Folders) > maximumHostFolders {
		return hostFolders{}, errors.New("host folder settings are invalid or exceed 16 folders")
	}
	names := make(map[string]bool, len(configuration.Folders))
	normalized := emptyHostFolders()
	for _, folder := range configuration.Folders {
		if folder.Schema != "" || folder.Source == "" {
			return hostFolders{}, errors.New("each host folder needs an existing NAS path")
		}
		legacy := folder
		legacy.Schema = hostFolderSchema
		if err := validateHostFolder(legacy); err != nil {
			return hostFolders{}, err
		}
		if folder.Name == "" {
			folder.Name = filepath.Base(folder.Source)
		}
		if folder.Name == "." || folder.Name == ".." || len(folder.Name) > maximumHostFolderNameBytes || strings.ContainsAny(folder.Name, "/\\") {
			return hostFolders{}, errors.New("use a folder name of at most 128 UTF-8 bytes, without slashes, dot or dot-dot")
		}
		for _, character := range folder.Name {
			if character < 32 || character == 127 {
				return hostFolders{}, errors.New("host folder names cannot contain control characters")
			}
		}
		if names[folder.Name] {
			return hostFolders{}, errors.New("host folder names must be unique; choose an override for duplicate folder names")
		}
		names[folder.Name] = true
		normalized.Folders = append(normalized.Folders, folder)
	}
	encoded, err := json.Marshal(normalized)
	if err != nil || len(encoded) > maximumHostFolderBytes {
		return hostFolders{}, errors.New("host folder settings exceed the 64 KiB limit")
	}
	return normalized, nil
}

func readHostFolders(path string) (hostFolders, error) {
	data, err := readPrivateBounded(path, maximumHostFolderBytes)
	if errors.Is(err, os.ErrNotExist) {
		return emptyHostFolders(), nil
	}
	if err != nil {
		return hostFolders{}, errors.New("saved host folder settings are unavailable")
	}
	var header struct {
		Schema string `json:"schema"`
	}
	if json.Unmarshal(data, &header) != nil {
		return hostFolders{}, errors.New("saved host folder settings are invalid")
	}
	if header.Schema == hostFolderSchema {
		var legacy hostFolder
		if err := decodeHostFolderJSON(data, &legacy); err != nil {
			return hostFolders{}, err
		}
		if err := validateHostFolder(legacy); err != nil {
			return hostFolders{}, err
		}
		configuration := emptyHostFolders()
		if legacy.Source != "" {
			// Keep the existing /host/source location for r10 upgrades.
			configuration.Folders = append(configuration.Folders, hostFolder{Source: legacy.Source, Name: "source", Access: legacy.Access})
		}
		return normalizeHostFolders(configuration)
	}
	var configuration hostFolders
	if err := decodeHostFolderJSON(data, &configuration); err != nil {
		return hostFolders{}, err
	}
	return normalizeHostFolders(configuration)
}

func selectHostFolders(saved hostFolders, action, entries string) (hostFolders, error) {
	switch action {
	case "", "keep":
		return normalizeHostFolders(saved)
	case "remove":
		return emptyHostFolders(), nil
	case "set":
		configuration := emptyHostFolders()
		if err := decodeHostFolderJSON([]byte(entries), &configuration.Folders); err != nil {
			return hostFolders{}, err
		}
		if len(configuration.Folders) == 0 {
			return hostFolders{}, errors.New("add at least one host folder or choose Remove all")
		}
		return normalizeHostFolders(configuration)
	default:
		return hostFolders{}, errors.New("choose whether to keep, edit or remove host folders")
	}
}

func renderHostFolderCompose(base []byte, configuration hostFolders) ([]byte, error) {
	configuration, err := normalizeHostFolders(configuration)
	if err != nil || len(base) == 0 || len(base) > maximumComposeBytes {
		return nil, errors.New("container configuration is invalid")
	}
	anchor := []byte("      - /var/packages/dockpipe-desktop/var:/config\n")
	if bytes.Count(base, anchor) != 1 || bytes.Contains(base, []byte("/host/")) {
		return nil, errors.New("container configuration has an unexpected workspace mapping")
	}
	replacement := append([]byte{}, anchor...)
	for _, folder := range configuration.Folders {
		mapping := struct {
			Type     string `json:"type"`
			Source   string `json:"source"`
			Target   string `json:"target"`
			ReadOnly bool   `json:"read_only"`
			Bind     struct {
				CreateHostPath bool `json:"create_host_path"`
			} `json:"bind"`
		}{Type: "bind", Source: folder.Source, Target: "/host/" + folder.Name, ReadOnly: folder.Access == "ro"}
		encoded, err := json.Marshal(mapping)
		if err != nil {
			return nil, errors.New("host folder configuration is invalid")
		}
		// Compose expands dollar signs even inside quoted YAML values.
		encoded = bytes.ReplaceAll(encoded, []byte("$"), []byte("$$"))
		replacement = append(replacement, []byte("      - ")...)
		replacement = append(replacement, encoded...)
		replacement = append(replacement, '\n')
	}
	return bytes.Replace(base, anchor, replacement, 1), nil
}

func readComposeBase(path string) ([]byte, error) {
	descriptor, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_CLOEXEC|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, errors.New("installer container template is unavailable")
	}
	file := os.NewFile(uintptr(descriptor), "container template")
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maximumComposeBytes {
		return nil, errors.New("installer container template is invalid")
	}
	data, err := io.ReadAll(io.LimitReader(file, maximumComposeBytes+1))
	if err != nil || len(data) > maximumComposeBytes {
		return nil, errors.New("installer container template exceeds its size limit")
	}
	return data, nil
}

func replaceFolderFile(path string, data []byte, mode os.FileMode) error {
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("host folder configuration must be a regular file")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return errors.New("host folder configuration is unavailable")
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".host-folder-*")
	if err != nil {
		return errors.New("cannot prepare host folder configuration")
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if file.Chmod(mode) != nil {
		return errors.New("cannot protect host folder configuration")
	}
	if _, err := file.Write(data); err != nil || file.Sync() != nil {
		return errors.New("cannot write host folder configuration")
	}
	if file.Close() != nil || os.Rename(file.Name(), path) != nil {
		return errors.New("cannot publish host folder configuration")
	}
	return nil
}

func prepareHostFolder(stage, savedPath, action, entries string) error {
	// Only installer staging is changed. postinst persists the desired mapping
	// after DSM's Docker Project resource acquisition stage.
	desktop := filepath.Join(stage, "desktop")
	info, err := os.Lstat(desktop)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("installer staging directory is unavailable")
	}
	if err := checkHostFolderDestination(savedPath); err != nil {
		return err
	}
	saved := emptyHostFolders()
	if action == "" || action == "keep" {
		saved, err = readHostFolders(savedPath)
		if err != nil {
			return err
		}
	}
	configuration, err := selectHostFolders(saved, action, entries)
	if err != nil {
		return err
	}
	for index, folder := range configuration.Folders {
		if err := checkHostFolderMounts(folder.Source); err != nil {
			return fmt.Errorf("folder %d %q at /host/%s: %w", index+1, folder.Source, folder.Name, err)
		}
		if err := checkHostFolderAccess(folder.Source, folder.Access); err != nil {
			return fmt.Errorf("folder %d %q at /host/%s (%s): %w", index+1, folder.Source, folder.Name, folder.Access, err)
		}
	}
	base, err := readComposeBase(filepath.Join(desktop, "compose.base.yml"))
	if err != nil {
		return err
	}
	compose, err := renderHostFolderCompose(base, configuration)
	if err != nil {
		return err
	}
	data, err := json.Marshal(configuration)
	if err != nil || len(data) > maximumHostFolderBytes {
		return errors.New("host folder configuration exceeds its size limit")
	}
	if err := replaceFolderFile(filepath.Join(desktop, "host-folder.json"), data, 0600); err != nil {
		return err
	}
	return replaceFolderFile(filepath.Join(desktop, "compose.yml"), compose, 0644)
}

func checkHostFolderDestination(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		return errors.New("saved host folder settings must be a package-private regular file")
	}
	metadata, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(metadata.Uid) != os.Geteuid() {
		return errors.New("saved host folder settings must belong to the package account")
	}
	return nil
}

func saveHostFolder(target, savedPath string) error {
	if err := checkHostFolderDestination(savedPath); err != nil {
		return err
	}
	staged := filepath.Join(target, "desktop/host-folder.json")
	if _, err := os.Lstat(staged); err != nil {
		return errors.New("installer did not prepare host folder configuration")
	}
	configuration, err := readHostFolders(staged)
	if err != nil {
		return err
	}
	data, err := json.Marshal(configuration)
	if err != nil {
		return err
	}
	return replaceFolderFile(savedPath, data, 0600)
}

func validInstallerPayload(root string) bool {
	for _, directory := range []string{root, filepath.Join(root, "bin"), filepath.Join(root, "desktop")} {
		info, err := os.Lstat(directory)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return false
		}
	}
	helper, err := os.Lstat(filepath.Join(root, "bin/dockpipe-dsm-host"))
	if err != nil || !helper.Mode().IsRegular() || helper.Mode().Perm()&0111 == 0 {
		return false
	}
	base, err := os.Lstat(filepath.Join(root, "desktop/compose.base.yml"))
	return err == nil && base.Mode().IsRegular() && base.Size() > 0 && base.Size() <= maximumComposeBytes
}

func installerPayloadRoot(temporary, executable string) (string, error) {
	if !filepath.IsAbs(temporary) || filepath.Clean(temporary) == "/" || !filepath.IsAbs(executable) {
		return "", errors.New("DSM installer staging and helper paths must be absolute")
	}
	temporary = filepath.Clean(temporary)
	executable = filepath.Clean(executable)
	root := filepath.Dir(filepath.Dir(executable))
	if executable != filepath.Join(root, "bin/dockpipe-dsm-host") {
		return "", errors.New("installer helper is not inside the candidate payload bin directory")
	}
	selected := ""
	for _, candidate := range []string{temporary, filepath.Join(temporary, "package")} {
		if !validInstallerPayload(candidate) {
			continue
		}
		if selected != "" {
			return "", errors.New("DSM installer staging contains ambiguous candidate payloads")
		}
		selected = candidate
	}
	if selected == "" {
		return "", errors.New("executed installer helper does not match the supported DSM staging payload")
	}
	selectedRoot, selectedRootError := os.Stat(selected)
	executedRoot, executedRootError := os.Stat(root)
	selectedHelper, selectedHelperError := os.Lstat(filepath.Join(selected, "bin/dockpipe-dsm-host"))
	executedHelper, executedHelperError := os.Lstat(executable)
	if selectedRootError != nil || executedRootError != nil || selectedHelperError != nil || executedHelperError != nil ||
		!selectedHelper.Mode().IsRegular() || !executedHelper.Mode().IsRegular() ||
		!os.SameFile(selectedRoot, executedRoot) || !os.SameFile(selectedHelper, executedHelper) {
		return "", errors.New("executed installer helper does not match the supported DSM staging payload")
	}
	return selected, nil
}

func hostFolderRole(role string) error {
	configuration := os.Getenv("SYNOPKG_PKGETC")
	if volume := os.Getenv("SYNOPKG_PKGDEST_VOL"); volume != "" && volume != "/" {
		configuration = filepath.Join(volume, "@appconf/dockpipe-desktop")
	}
	if configuration == "" {
		configuration = packageRoot + "/etc"
	}
	saved := filepath.Join(configuration, "host-folder.json")
	if role == "folder-save" {
		target := os.Getenv("SYNOPKG_PKGDEST")
		if target == "" {
			return errors.New("installer target directory is unavailable")
		}
		return saveHostFolder(target, saved)
	}
	temporary := os.Getenv("SYNOPKG_PKGINST_TEMP_DIR")
	executable, err := os.Executable()
	if err != nil {
		return errors.New("cannot identify the executed installer helper")
	}
	payload, err := installerPayloadRoot(temporary, executable)
	if err != nil {
		return err
	}
	return prepareHostFolder(payload, saved,
		os.Getenv("wizard_host_folder_action"), os.Getenv("wizard_host_folders"))
}
