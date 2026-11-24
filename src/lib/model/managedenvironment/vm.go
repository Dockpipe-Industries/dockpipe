package managedenvironment

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

type VM struct {
	OS           string `yaml:"os" json:"os"`
	CPUs         uint   `yaml:"cpus" json:"cpus"`
	MemoryGiB    uint   `yaml:"memory_gib" json:"memory_gib"`
	DiskGiB      uint   `yaml:"disk_gib" json:"disk_gib"`
	InstallerISO string `yaml:"installer_iso,omitempty" json:"installer_iso,omitempty"`
	Resolver     string `yaml:"resolver,omitempty" json:"resolver,omitempty"`
}

func VMPresets() []VM {
	return []VM{
		{OS: "ubuntu", CPUs: 2, MemoryGiB: 4, DiskGiB: 64},
		{OS: "debian", CPUs: 2, MemoryGiB: 2, DiskGiB: 32},
		{OS: "fedora", CPUs: 4, MemoryGiB: 8, DiskGiB: 64},
		{OS: "other-linux", CPUs: 2, MemoryGiB: 4, DiskGiB: 32},
		{OS: "windows11", CPUs: 4, MemoryGiB: 8, DiskGiB: 80},
	}
}

func (settings VM) Validate() error {
	if !slices.ContainsFunc(VMPresets(), func(preset VM) bool { return preset.OS == settings.OS }) {
		return fmt.Errorf("unknown VM operating system %q; see dockpipe environment presets --kind vm", settings.OS)
	}
	if settings.CPUs < 1 || settings.CPUs > 256 || settings.MemoryGiB < 1 || settings.MemoryGiB > 1024 ||
		settings.DiskGiB < 1 || settings.DiskGiB > 16384 {
		return errors.New("choose 1–256 CPUs, 1–1024 GiB of RAM, and 1–16384 GiB of disk")
	}
	if settings.OS == "windows11" && (settings.CPUs < 2 || settings.MemoryGiB < 4 || settings.DiskGiB < 64) {
		return errors.New("at least 2 CPUs, 4 GiB of RAM, and 64 GiB of disk are required for Windows 11")
	}
	if settings.InstallerISO != "" && (!strings.EqualFold(filepath.Ext(settings.InstallerISO), ".iso") ||
		len(settings.InstallerISO) > 4096 || strings.ContainsAny(settings.InstallerISO, "\x00\r\n\t")) {
		return errors.New("choose a valid installer ISO path")
	}
	if settings.Resolver != "" && !ValidName(settings.Resolver) {
		return errors.New("invalid VM resolver name")
	}
	return nil
}
