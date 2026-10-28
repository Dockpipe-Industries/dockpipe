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

func cmdDependencyChange(operation string, args []string) error {
	flags := flag.NewFlagSet("dependency "+operation, flag.ContinueOnError)
	requestPath := flags.String("request", "", "exact local provider change request JSON")
	workdir := flags.String("workdir", ".", "existing resolver discovery context")
	dryRun := flags.Bool("dry-run", false, "validate and review without changing installed bytes")
	approve := flags.String("approve", "", "exact operation approval ID printed by dry-run")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	statusOnly := operation == "activation-status"
	if flags.NArg() != 0 || *requestPath == "" || (*dryRun && *approve != "") || (!statusOnly && !*dryRun && *approve == "") || (statusOnly && (*dryRun || *approve != "")) {
		return errors.New("use --request <file> with either --dry-run or --approve <exact-operation-id>")
	}
	request, err := lifecycleio.ReadChangeRequest(*requestPath)
	if err != nil {
		return err
	}
	action := "apply"
	if statusOnly {
		action = "activation-status"
	}
	if operation == "activate" {
		action = "activate"
	}
	if operation == "recover" {
		action = "recover"
	}
	if !statusOnly && !*dryRun && *approve != request.ApprovalID(action) {
		return errors.New("approval does not match this exact request and operation")
	}
	if *dryRun {
		if action == "activate" {
			action = "activation-plan"
		} else if action == "recover" {
			action = "recover-plan"
		} else {
			action = "plan"
		}
	}
	directory, err := filepath.Abs(*workdir)
	if err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	return infrastructure.RunOperationWithOptions(os.Stderr, "dependency."+operation, "Change local dependency", map[string]string{"request": request.ApprovalID(action), "resolver": request.Resolver},
		infrastructure.OperationOptions{Spinner: false}, func() error {
			provider := lifecycleio.WorkflowProvider{Executable: executable, Diagnostics: os.Stderr}
			result, err := provider.Change(ctx, directory, request, action, *approve)
			if err != nil {
				return err
			}
			response := struct {
				Result           dependency.ChangeResult `json:"result"`
				ApplyApproval    string                  `json:"apply_approval"`
				ActivateApproval string                  `json:"activate_approval"`
				RecoveryApproval string                  `json:"recovery_approval"`
			}{result, request.ApprovalID("apply"), request.ApprovalID("activate"), request.ApprovalID("recover")}
			if err := json.NewEncoder(os.Stdout).Encode(response); err != nil {
				return err
			}
			if result.Status == "recovery-required" || result.Status == "manual-required" || result.Status == "activation-recovered" {
				return fmt.Errorf("dependency change requires attention: %s", result.Message)
			}
			return nil
		})
}
