package application

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"dockpipe/src/lib/domain/dependency"
	"dockpipe/src/lib/infrastructure"
	lifecycleio "dockpipe/src/lib/infrastructure/dependencylifecycle"
)

const dependencyUsage = `dockpipe dependency: host inventory, review plans and approved local changes
  inventory --resolver <name> [--machine <id>] [--workdir <path>] [--timeout-seconds <5..120>]
  assess --report <inventory.json>
  plan --report <inventory.json> --dry-run [--policy manual|notify|security-only|compatible|latest] [--pin <exact-version>]
  update --request <change.json> [--workdir <path>] (--dry-run | --approve <apply-id>)
  recover --request <change.json> [--workdir <path>] (--dry-run | --approve <recovery-id>)
Update/recover execute the selected trusted provider's local change workflow.
  activate --request <activation.json> [--workdir <path>] (--dry-run | --approve <activate-id>)
  activation-status --request <activation.json> [--workdir <path>]
Activation uses broker-owned supervision, exact maintenance approval and existing OS authentication.
No remote updates or unattended policy execution are implemented.
Inventory runs trusted resolver workflow code with ordinary workflow permissions;
the inventory protocol is not an OS sandbox. Assess/plan only read supplied evidence.
`

func cmdDependency(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Print(dependencyUsage)
		return nil
	}
	operation := args[0]
	if operation == "update" || operation == "recover" || operation == "activate" || operation == "activation-status" {
		return cmdDependencyChange(operation, args[1:])
	}
	if operation != "inventory" && operation != "assess" && operation != "plan" {
		return errors.New(dependencyUsage)
	}
	flags := flag.NewFlagSet("dependency "+operation, flag.ContinueOnError)
	resolver := flags.String("resolver", "", "explicit installed or declared resolver")
	machine := flags.String("machine", "local", "target identity for this local observation")
	workdir := flags.String("workdir", ".", "project used for existing resolver discovery")
	timeout := flags.Int("timeout-seconds", 60, "bounded inspection deadline")
	report := flags.String("report", "", "inventory evidence file")
	policy := flags.String("policy", "manual", "policy recorded in the non-executable plan")
	pin := flags.String("pin", "", "additional exact constraint; never overrides existing locks")
	dryRun := flags.Bool("dry-run", false, "emit a non-executable review plan")
	if err := flags.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 || *timeout < 5 || *timeout > 120 {
		return errors.New("dependency command requires named flags and a timeout from 5 to 120 seconds")
	}
	var invalidFlag string
	flags.Visit(func(value *flag.Flag) {
		inventoryFlag := value.Name == "resolver" || value.Name == "machine" || value.Name == "workdir" || value.Name == "timeout-seconds"
		planFlag := value.Name == "policy" || value.Name == "pin" || value.Name == "dry-run"
		if (operation == "inventory" && !inventoryFlag) || (operation != "inventory" && inventoryFlag) || (operation == "assess" && planFlag) {
			invalidFlag = value.Name
		}
	})
	if invalidFlag != "" {
		return fmt.Errorf("--%s is not supported by dependency %s", invalidFlag, operation)
	}
	if operation == "plan" && !*dryRun {
		return errors.New("dependency plan requires --dry-run; execution is not implemented")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, time.Duration(*timeout)*time.Second)
	defer cancel()
	return infrastructure.RunOperationWithOptions(os.Stderr, "dependency."+operation, "Inspect dependency lifecycle", nil,
		infrastructure.OperationOptions{Spinner: false}, func() error {
			if operation == "inventory" {
				executable, err := os.Executable()
				if err != nil {
					return err
				}
				directory, err := filepath.Abs(*workdir)
				if err != nil {
					return err
				}
				provider := lifecycleio.WorkflowProvider{Executable: executable, Diagnostics: os.Stderr}
				inventory, err := provider.Inspect(ctx, directory, *resolver, *machine)
				if err != nil {
					return err
				}
				return json.NewEncoder(os.Stdout).Encode(inventory)
			}
			if *report == "" {
				return errors.New("--report is required")
			}
			inventory, err := lifecycleio.ReadInventory(*report)
			if err != nil {
				return err
			}
			if operation == "assess" {
				assessment, err := dependency.AssessInventory(inventory, time.Now())
				if err != nil {
					return err
				}
				return json.NewEncoder(os.Stdout).Encode(assessment)
			}
			plan, err := dependency.BuildDryRunPlan(inventory, dependency.UpdatePolicy(*policy), *pin, time.Now())
			if err != nil {
				return err
			}
			return json.NewEncoder(os.Stdout).Encode(plan)
		})
}
