package application

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/infrastructure"
)

// cmdInternalState is an intentionally hidden bridge for bundled/package-owned host scripts. It
// is not a public package-state, get, scope, SDK, or configuration surface.
func cmdInternalState(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("unknown internal state operation")
	}
	if args[0] == "package-runtime" {
		return cmdInternalPackageRuntime(args[1:])
	}
	if args[0] == "private-directory" {
		return cmdInternalPrivateDirectory(args[1:])
	}
	if args[0] != "prepare-durable-cohort" {
		return fmt.Errorf("unknown internal state operation")
	}
	parsed, err := parseInternalDurableCohortArgs(args[1:])
	if err != nil {
		return err
	}
	status, err := infrastructure.PrepareDurableCohortImport(parsed.workdir, infrastructure.DurableCohortImportSpec{
		OwnerID:     parsed.owner,
		Cohort:      parsed.cohort,
		InstanceID:  parsed.instance,
		RunID:       parsed.runID,
		LegacyRoot:  parsed.legacyRoot,
		Mappings:    parsed.mappings,
		IgnorePaths: parsed.ignorePaths,
	})
	if err != nil {
		return err
	}
	for _, value := range []string{status.DurableDir, status.RuntimeDir} {
		if strings.ContainsAny(value, "\r\n\t") {
			return fmt.Errorf("internal state path contains unsupported control characters")
		}
	}
	fmt.Printf("%s\t%s\t%t\t%t\n", status.DurableDir, status.RuntimeDir, status.ImportedLegacy, status.LegacyDiverged)
	return nil
}

func cmdInternalPrivateDirectory(args []string) error {
	root := ""
	suffix := ""
	for index := 0; index < len(args); index++ {
		name := args[index]
		if index+1 >= len(args) {
			return fmt.Errorf("%s requires a value", name)
		}
		value := args[index+1]
		index++
		switch name {
		case "--root":
			if root != "" {
				return fmt.Errorf("internal private directory accepts one root")
			}
			root = value
		case "--path":
			if suffix != "" {
				return fmt.Errorf("internal private directory accepts one path")
			}
			suffix = value
		default:
			return fmt.Errorf("unknown internal private directory option %q", name)
		}
	}
	if root == "" || suffix == "" {
		return fmt.Errorf("internal private directory requires root and path")
	}
	destination, err := infrastructure.PreparePrivateStateSubdirectory(root, suffix)
	if err != nil {
		return err
	}
	if strings.ContainsAny(destination, "\r\n\t") {
		return fmt.Errorf("internal private directory path contains unsupported control characters")
	}
	fmt.Println(destination)
	return nil
}

func cmdInternalPackageRuntime(args []string) error {
	workdir := ""
	owner := ""
	suffix := ""
	ensurePrivate := false
	for index := 0; index < len(args); index++ {
		name := args[index]
		if name == "--ensure-private" {
			ensurePrivate = true
			continue
		}
		if index+1 >= len(args) {
			return fmt.Errorf("%s requires a value", name)
		}
		value := args[index+1]
		index++
		switch name {
		case "--workdir":
			workdir = value
		case "--owner":
			owner = value
		case "--path":
			if suffix != "" {
				return fmt.Errorf("internal package runtime accepts one path suffix")
			}
			suffix = value
		default:
			return fmt.Errorf("unknown internal package runtime option %q", name)
		}
	}
	if workdir == "" {
		workdir = strings.TrimSpace(os.Getenv("DOCKPIPE_WORKDIR"))
	}
	if workdir == "" {
		var err error
		workdir, err = os.Getwd()
		if err != nil {
			return err
		}
	}
	workdir, err := filepath.Abs(filepath.Clean(workdir))
	if err != nil {
		return err
	}
	var root string
	if ensurePrivate {
		root, err = infrastructure.PreparePackageRuntimeDir(workdir, owner)
	} else {
		root, err = infrastructure.PackageRuntimeDir(workdir, owner)
	}
	if err != nil {
		return err
	}
	if suffix != "" {
		root, err = infrastructure.JoinStatePath(root, suffix)
		if err != nil {
			return err
		}
	}
	if strings.ContainsAny(root, "\r\n\t") {
		return fmt.Errorf("internal package runtime path contains unsupported control characters")
	}
	fmt.Println(root)
	return nil
}

type internalDurableCohortArgs struct {
	workdir     string
	owner       string
	cohort      string
	instance    string
	runID       string
	legacyRoot  string
	mappings    []infrastructure.DurableImportMapping
	ignorePaths []string
}

func parseInternalDurableCohortArgs(args []string) (internalDurableCohortArgs, error) {
	parsed := internalDurableCohortArgs{}
	for index := 0; index < len(args); index++ {
		name := args[index]
		if index+1 >= len(args) {
			return parsed, fmt.Errorf("%s requires a value", name)
		}
		value := args[index+1]
		index++
		switch name {
		case "--workdir":
			parsed.workdir = value
		case "--owner":
			parsed.owner = value
		case "--cohort":
			parsed.cohort = value
		case "--instance":
			parsed.instance = value
		case "--run":
			parsed.runID = value
		case "--legacy-root":
			parsed.legacyRoot = value
		case "--file", "--tree":
			source, destination, ok := strings.Cut(value, "=")
			if !ok || source == "" || destination == "" {
				return parsed, fmt.Errorf("%s requires source=destination", name)
			}
			parsed.mappings = append(parsed.mappings, infrastructure.DurableImportMapping{Source: source, Destination: destination, Tree: name == "--tree"})
		case "--ignore":
			parsed.ignorePaths = append(parsed.ignorePaths, value)
		default:
			return parsed, fmt.Errorf("unknown internal state option %q", name)
		}
	}
	if parsed.workdir == "" {
		parsed.workdir = strings.TrimSpace(os.Getenv("DOCKPIPE_WORKDIR"))
	}
	if parsed.workdir == "" {
		var err error
		parsed.workdir, err = os.Getwd()
		if err != nil {
			return parsed, err
		}
	}
	workdir, err := filepath.Abs(filepath.Clean(parsed.workdir))
	if err != nil {
		return parsed, err
	}
	parsed.workdir = workdir
	if parsed.owner == "" || parsed.cohort == "" || parsed.instance == "" || parsed.runID == "" || parsed.legacyRoot == "" {
		return parsed, fmt.Errorf("internal durable cohort requires owner, cohort, instance, run, and legacy root")
	}
	return parsed, nil
}
