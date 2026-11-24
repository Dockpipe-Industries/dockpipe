package application

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/model/managedenvironment"
)

func printEnvironmentPresets(args []string) error {
	kind := "container"
	if len(args) != 0 {
		if len(args) != 2 || args[0] != "--kind" {
			return errors.New("usage: dockpipe environment presets [--kind vm|container]")
		}
		kind = args[1]
	}
	switch kind {
	case "container":
		for _, preset := range managedenvironment.ContainerPresets() {
			fmt.Printf("%-8s %-20s %s\n", preset.Name, preset.Image, preset.Startup)
		}
	case "vm":
		for _, preset := range managedenvironment.VMPresets() {
			fmt.Printf("%-12s %d CPUs   %d GiB RAM   %d GiB disk\n", preset.OS, preset.CPUs, preset.MemoryGiB, preset.DiskGiB)
		}
	default:
		return errors.New("preset kind must be vm or container")
	}
	return nil
}

func resolveVMPreset(flags *flag.FlagSet, presetName string, settings managedenvironment.VM) (managedenvironment.VM, error) {
	if presetName != "" && settings.OS != "" {
		return managedenvironment.VM{}, errors.New("choose either --os or --preset for a VM")
	}
	if presetName == "" {
		presetName = settings.OS
	}
	presets := managedenvironment.VMPresets()
	index := slices.IndexFunc(presets, func(preset managedenvironment.VM) bool {
		return preset.OS == presetName
	})
	if index < 0 {
		return managedenvironment.VM{}, fmt.Errorf("unknown VM preset %q; see dockpipe environment presets --kind vm", presetName)
	}

	preset := presets[index]
	preset.Resolver = settings.Resolver
	preset.InstallerISO = settings.InstallerISO
	flags.Visit(func(current *flag.Flag) {
		switch current.Name {
		case "cpus":
			preset.CPUs = settings.CPUs
		case "memory-gib":
			preset.MemoryGiB = settings.MemoryGiB
		case "disk-gib":
			preset.DiskGiB = settings.DiskGiB
		}
	})
	return preset, nil
}

func vmEnvironmentResolver(workdir, requested string) (string, error) {
	if requested != "" {
		if _, err := infrastructure.ResolveResolverFilePath(workdir, requested); err != nil {
			return "", err
		}
		return requested, nil
	}

	projectRoot, err := domain.FindProjectRootWithDockpipeConfig(workdir)
	if err != nil {
		return "", err
	}
	var matches []string
	for _, resolver := range catalogResolverDetails(workdir, listCatalogResolvers(projectRoot, workdir)) {
		if resolver.Surface != nil && resolver.Surface.ID == "dockpipe.vm" {
			matches = append(matches, resolver.Name)
		}
	}
	switch len(matches) {
	case 0:
		return "", errors.New("install a VM resolver before starting this environment")
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("multiple VM resolvers are installed (%s); choose vm.resolver in the definition", strings.Join(matches, ", "))
	}
}

func applyVMEnvironment(path, workdir string, definition managedenvironment.Definition, baseEnviron []string) error {
	if strings.TrimSpace(workdir) == "" {
		var err error
		workdir, err = os.Getwd()
		if err != nil {
			return err
		}
	}
	settings := *definition.VM
	iso := settings.InstallerISO
	if iso != "" {
		if !filepath.IsAbs(iso) {
			iso = filepath.Join(filepath.Dir(path), iso)
		}
		var err error
		iso, err = filepath.Abs(iso)
		if err != nil {
			return err
		}
		info, err := os.Stat(iso)
		if err != nil {
			return fmt.Errorf("read VM installer ISO: %w", err)
		}
		if !info.Mode().IsRegular() {
			return errors.New("VM installer ISO must be a regular file")
		}
	}
	resolver, err := vmEnvironmentResolver(workdir, settings.Resolver)
	if err != nil {
		return err
	}
	instance, stateWorkdir, err := environmentInstanceForDefinition(path)
	if err != nil {
		return err
	}
	disk := filepath.Join(stateWorkdir, "disk.qcow2")
	info, err := os.Lstat(disk)
	switch {
	case err == nil && !info.Mode().IsRegular():
		return errors.New("the managed VM disk must be a regular file")
	case errors.Is(err, os.ErrNotExist) && iso == "":
		return errors.New("select an installer ISO in vm.installer_iso to create the first disk")
	case err != nil && !errors.Is(err, os.ErrNotExist):
		return err
	}
	environment, err := prepareEnvironmentStorage(nil, baseEnviron)
	if err != nil {
		return err
	}
	environment = append(environment, environmentInstanceKey+"="+instance.ID,
		environmentStateWorkdirKey+"="+stateWorkdir)
	environment = append(environment, vmEnvironmentVariables(disk, iso, settings)...)
	return Run([]string{"--runtime", "vm", "--resolver", resolver, "--workdir", workdir, "--", "true"}, environment)
}

func vmEnvironmentVariables(disk, iso string, settings managedenvironment.VM) []string {
	boot := "image"
	if iso != "" {
		boot = "installer-iso"
	}
	security := "off"
	if settings.OS == "windows11" {
		security = "required"
	}
	return []string{
		"DOCKPIPE_VM_BOOT_SOURCE=" + boot,
		"DOCKPIPE_VM_DISK=" + disk,
		"DOCKPIPE_VM_INSTALL_DISK=" + disk,
		"DOCKPIPE_VM_CDROM=" + iso,
		"DOCKPIPE_VM_DISK_SIZE=" + strconv.FormatUint(uint64(settings.DiskGiB), 10) + "G",
		"DOCKPIPE_VM_CPUS=" + strconv.FormatUint(uint64(settings.CPUs), 10),
		"DOCKPIPE_VM_MEMORY=" + strconv.FormatUint(uint64(settings.MemoryGiB), 10) + "G",
		"DOCKPIPE_VM_PERSISTENCE=persistent",
		"DOCKPIPE_VM_INTERACTIVE=true",
		"DOCKPIPE_VM_INTERACTIVE_SSH=false",
		"DOCKPIPE_VM_AGENT=false",
		"DOCKPIPE_VM_SYNC_HOST_PATH=",
		"DOCKPIPE_VM_SYNC_GUEST_PATH=",
		"DOCKPIPE_VM_MOUNTS=",
		"DOCKPIPE_VM_GUEST_PREPARATION=provided-only",
		"DOCKPIPE_VM_WINDOWS_INSTALL_MODE=manual",
		"DOCKPIPE_VM_TPM=" + security,
		"DOCKPIPE_VM_SECURE_BOOT=" + security,
	}
}
