package application

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/model/managedenvironment"
)

const environmentUsage = `Usage:
  dockpipe environment generate --name NAME --kind vm|container --workflow NAME [--package NAME] [--storage-profile NAME] [--file PATH | --personal]
  dockpipe environment create   --name NAME --kind vm|container --workflow NAME [--package NAME] [--storage-profile NAME] [--file PATH] [--workdir DIR] [-- WORKFLOW_ARGS...]
  dockpipe environment generate --name NAME --kind container (--image IMAGE | --preset PRESET) [CONTAINER_OPTIONS] [--file PATH | --personal]
  dockpipe environment create   --name NAME --kind container (--image IMAGE | --preset PRESET) [CONTAINER_OPTIONS] [--file PATH]
  dockpipe environment generate --name NAME --kind vm --preset PRESET [VM_OPTIONS] [--file PATH | --personal]
  dockpipe environment create   --name NAME --kind vm --preset PRESET --installer-iso PATH [VM_OPTIONS] [--file PATH]
  dockpipe environment presets [--kind vm|container]
  dockpipe environment apply    (--file PATH | --name NAME) [--workdir DIR] [-- WORKFLOW_ARGS...]
  dockpipe environment show     (--file PATH | --name NAME)
  dockpipe environment status   (--file PATH | --name NAME)
  dockpipe environment definitions [--observe] [--file PATH]...

Container options:
  --startup image|keep-running  --network bridge|none  --security standard|restricted
  --restart no|unless-stopped   --user image|non-root  --read-only
  --host-port PORT --container-port PORT             --data-path /CONTAINER/PATH

VM options:
  --os OS                     Use the operating system’s defaults in place of --preset.
  --cpus COUNT --memory-gib GIB --disk-gib GIB
  --installer-iso PATH        Required for a new disk on apply.
  --vm-resolver NAME          Select an installed VM resolver if several are available.
`

func cmdEnvironment(args []string, baseEnviron []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Print(environmentUsage)
		return nil
	}

	command := args[0]
	if command == "definitions" {
		return cmdEnvironmentDefinitions(args[1:])
	}
	if command == "record-runtime" {
		return recordEnvironmentRuntime(args[1:], baseEnviron)
	}
	if command == "presets" {
		return printEnvironmentPresets(args[1:])
	}

	options, workflowArgs := args[1:], []string(nil)
	if separator := slices.Index(options, "--"); separator >= 0 {
		workflowArgs = options[separator+1:]
		options = options[:separator]
	}
	flags := flag.NewFlagSet("environment "+command, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.Usage = func() { fmt.Print(environmentUsage) }
	file := flags.String("file", "", "Environment definition path")
	name := flags.String("name", "", "Environment name")
	var definition managedenvironment.Definition
	var personal bool
	var printPath bool
	var workdir string
	var storageProfile string
	var workflow managedenvironment.Workflow
	var container managedenvironment.Container
	var vm managedenvironment.VM
	var presetName string
	var hostPort, containerPort uint
	switch command {
	case "generate", "create":
		definition.Schema = managedenvironment.Schema
		flags.StringVar(&definition.Kind, "kind", "", "Environment kind")
		flags.StringVar(&workflow.Workflow, "workflow", "", "Provisioning workflow")
		flags.StringVar(&workflow.Package, "package", "", "Optional workflow package")
		flags.StringVar(&storageProfile, "storage-profile", "", "Private host storage profile name")
		flags.StringVar(&container.Image, "image", "", "Docker image")
		flags.StringVar(&presetName, "preset", "", "Environment preset name")
		flags.StringVar(&container.Startup, "startup", "", "Container startup")
		flags.StringVar(&container.Network, "network", "", "Container networking")
		flags.StringVar(&container.Security, "security", "", "Container security profile")
		flags.StringVar(&container.Restart, "restart", "", "Container restart policy")
		flags.StringVar(&container.User, "user", "", "Container user")
		flags.BoolVar(&container.ReadOnly, "read-only", false, "Read-only container filesystem")
		flags.UintVar(&hostPort, "host-port", 0, "Port bound to this computer")
		flags.UintVar(&containerPort, "container-port", 0, "Port inside the container")
		flags.StringVar(&container.DataPath, "data-path", "", "Container directory for persistent storage")
		flags.StringVar(&vm.OS, "os", "", "VM operating system")
		flags.UintVar(&vm.CPUs, "cpus", 0, "VM logical CPUs")
		flags.UintVar(&vm.MemoryGiB, "memory-gib", 0, "VM memory in GiB")
		flags.UintVar(&vm.DiskGiB, "disk-gib", 0, "New VM disk size in GiB")
		flags.StringVar(&vm.InstallerISO, "installer-iso", "", "VM installer ISO")
		flags.StringVar(&vm.Resolver, "vm-resolver", "", "Installed VM resolver")
	case "apply", "show", "status":
	default:
		return fmt.Errorf("unknown environment command %q", command)
	}
	if command == "generate" {
		flags.BoolVar(&personal, "personal", false, "Save in the Dockpipe user scope")
		flags.BoolVar(&printPath, "print-path", false, "Print the absolute saved definition path")
	}
	if command == "create" || command == "apply" {
		flags.StringVar(&workdir, "workdir", "", "Provisioning workflow working directory")
	} else if len(workflowArgs) != 0 {
		return errors.New("workflow arguments require environment create or apply")
	}
	if err := flags.Parse(options); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected environment argument %q", flags.Arg(0))
	}

	path := *file
	switch command {
	case "generate", "create":
		definition.Name = *name
		if workflow != (managedenvironment.Workflow{}) {
			definition.Provision = &workflow
		}
		if definition.Kind == "vm" && (presetName != "" || vm != (managedenvironment.VM{})) {
			settings, err := resolveVMPreset(flags, presetName, vm)
			if err != nil {
				return err
			}
			definition.VM = &settings
		} else if presetName != "" {
			if container.Image != "" {
				return errors.New("choose either --image or --preset")
			}
			presets := managedenvironment.ContainerPresets()
			index := slices.IndexFunc(presets, func(preset managedenvironment.ContainerPreset) bool {
				return preset.Name == presetName
			})
			if index < 0 {
				return fmt.Errorf("unknown container preset %q; see dockpipe environment presets", presetName)
			}
			preset := presets[index]
			container.Image = preset.Image
			if container.Startup == "" {
				container.Startup = preset.Startup
			}
		}
		if hostPort > 65535 || containerPort > 65535 {
			return errors.New("container ports must be between 1 and 65535")
		}
		portSpecified := false
		flags.Visit(func(current *flag.Flag) {
			if current.Name == "host-port" || current.Name == "container-port" {
				portSpecified = true
			}
		})
		if portSpecified {
			container.Publish = &managedenvironment.LocalPort{Host: uint16(hostPort), Container: uint16(containerPort)}
		}
		if container != (managedenvironment.Container{}) {
			definition.Container = &container
		}
		if definition.Container != nil && len(workflowArgs) != 0 {
			return errors.New("container settings do not take workflow arguments")
		}
		if definition.VM != nil && len(workflowArgs) != 0 {
			return errors.New("VM settings do not take workflow arguments")
		}
		if definition.Kind != "vm" && vm != (managedenvironment.VM{}) {
			return errors.New("VM options require --kind vm")
		}
		if storageProfile != "" {
			definition.Storage = &managedenvironment.Storage{Profile: storageProfile}
		}
		if err := definition.Validate(); err != nil {
			return err
		}
		if personal && path != "" {
			return errors.New("choose either --personal or --file")
		}
		if path == "" && (personal || command == "create") {
			var err error
			path, err = personalEnvironmentPath(*name, true)
			if err != nil {
				return err
			}
		}
		if path == "" {
			if printPath {
				return errors.New("--print-path requires --personal or --file")
			}
			return writeEnvironment(os.Stdout, definition)
		}
		if err := saveNewEnvironment(path, definition); err != nil {
			return err
		}
		if command == "generate" {
			if printPath {
				absolute, err := filepath.Abs(path)
				if err != nil {
					return err
				}
				_, err = fmt.Fprintln(os.Stdout, absolute)
				return err
			}
			return nil
		}
	case "apply", "show", "status":
		if (path == "") == (*name == "") {
			return errors.New("choose exactly one of --file or --name")
		}
		if path == "" {
			var err error
			path, err = personalEnvironmentPath(*name, false)
			if err != nil {
				return err
			}
		}
		var err error
		definition, err = readEnvironment(path)
		if err != nil {
			return err
		}
		if *name != "" && *name != definition.Name {
			return errors.New("personal environment name differs from its saved definition")
		}
		if command == "show" {
			return writeEnvironment(os.Stdout, definition)
		}
		if command == "status" {
			return writeEnvironmentStatus(path, definition)
		}
	}

	if definition.Container != nil {
		if len(workflowArgs) != 0 {
			return errors.New("container settings do not take workflow arguments")
		}
		instance, _, err := environmentInstanceForDefinition(path)
		if err != nil {
			return err
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		containerID, err := infrastructure.ApplyDockerEnvironment(ctx, definition.Name, instance.ID, *definition.Container)
		if err != nil {
			return err
		}
		fmt.Printf("Started container %s\n", containerID)
		return nil
	}
	if strings.TrimSpace(workdir) == "" && *file != "" {
		workdir = filepath.Dir(path)
	}
	if definition.VM != nil {
		if len(workflowArgs) != 0 {
			return errors.New("VM settings do not take workflow arguments")
		}
		return applyVMEnvironment(path, workdir, definition, baseEnviron)
	}

	invocation := []string{"--workflow", definition.Provision.Workflow}
	if definition.Provision.Package != "" {
		invocation = append(invocation, "--package", definition.Provision.Package)
	}
	if workdir != "" {
		invocation = append(invocation, "--workdir", workdir)
	}
	if len(workflowArgs) != 0 {
		invocation = append(append(invocation, "--"), workflowArgs...)
	}
	environ, err := prepareEnvironmentStorage(definition.Storage, baseEnviron)
	if err != nil {
		return err
	}
	instance, stateWorkdir, err := environmentInstanceForDefinition(path)
	if err != nil {
		return err
	}

	environ = append(environ, environmentInstanceKey+"="+instance.ID,
		environmentStateWorkdirKey+"="+stateWorkdir)
	return Run(invocation, environ)
}
