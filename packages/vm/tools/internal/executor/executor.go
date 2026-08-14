package executor

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Runner is deliberately typed. It offers no command, shell, environment,
// network, SSH, passthrough, share, physical-disk, or fallback surface.
// Tests use fake implementations; the package-owned Linux adapter implements
// only these sealed operations.
type Runner interface {
	CreateOSClone(context.Context, OSCloneRequest) error
	CreateSparseRawDisk(context.Context, SparseRawDiskRequest) error
	CreateNoCloudSeed(context.Context, NoCloudSeedRequest) error
	LaunchQEMU(context.Context, LaunchRequest) error
	VerifyGuest(context.Context, GuestVerificationRequest) error
	ControlledShutdown(context.Context, ShutdownRequest) error
	PreserveFailure(context.Context, PreservationRequest) error
	Cleanup(context.Context, CleanupRequest) error
}

type Result struct {
	ExecutionSHA256 string   `json:"execution_sha256"`
	Completed       []string `json:"completed"`
	Preserved       bool     `json:"preserved"`
	CleanupRun      bool     `json:"cleanup_run"`
}

type step struct {
	name    string
	timeout int
	run     func(context.Context) error
}

func Execute(ctx context.Context, contract Contract, runner Runner) (Result, error) {
	result := Result{ExecutionSHA256: contract.ExecutionSHA256}
	if runner == nil {
		return result, fmt.Errorf("typed executor runner is required")
	}
	if err := contract.Validate(); err != nil {
		return result, err
	}
	if contract.Schema != Schema || contract.FirstBootObservation == nil {
		return result, fmt.Errorf("qualification execution requires the current first-boot observation contract")
	}
	steps := []step{
		{"create-private-os-clone", contract.OSClone.Command.TimeoutSeconds, func(call context.Context) error { return runner.CreateOSClone(call, contract.OSClone) }},
		{"create-private-data-disk", PreservationDeadline, func(call context.Context) error { return runner.CreateSparseRawDisk(call, contract.DataDisk) }},
		{"create-nocloud-seed", PreservationDeadline, func(call context.Context) error { return runner.CreateNoCloudSeed(call, contract.NoCloud) }},
		{"launch-qemu", contract.Launch.Command.TimeoutSeconds, func(call context.Context) error { return runner.LaunchQEMU(call, contract.Launch) }},
		{"verify-guest", contract.Guest.TimeoutSeconds, func(call context.Context) error { return runner.VerifyGuest(call, contract.Guest) }},
		{"controlled-shutdown", contract.Shutdown.TimeoutSeconds, func(call context.Context) error { return runner.ControlledShutdown(call, contract.Shutdown) }},
	}
	for _, current := range steps {
		call, cancel := context.WithTimeout(ctx, time.Duration(current.timeout)*time.Second)
		err := current.run(call)
		cancel()
		if err != nil {
			preserveCtx, preserveCancel := context.WithTimeout(context.WithoutCancel(ctx), time.Duration(contract.Preservation.TimeoutSeconds)*time.Second)
			preserveErr := runner.PreserveFailure(preserveCtx, contract.Preservation)
			preserveCancel()
			result.Preserved = preserveErr == nil
			if preserveErr != nil {
				return result, fmt.Errorf("%s failed: %w; preserve complete failure: %v", current.name, err, preserveErr)
			}
			return result, fmt.Errorf("%s failed and the complete instance was preserved: %w", current.name, err)
		}
		result.Completed = append(result.Completed, current.name)
	}
	return result, nil
}

func ExecuteCleanup(ctx context.Context, contract Contract, authorization CleanupAuthorization, now time.Time, runner Runner) (Result, error) {
	result := Result{ExecutionSHA256: contract.ExecutionSHA256}
	if runner == nil {
		return result, fmt.Errorf("typed executor runner is required")
	}
	if err := contract.Validate(); err != nil {
		return result, err
	}
	if err := authorization.Validate(contract, now); err != nil {
		return result, err
	}
	call, cancel := context.WithTimeout(ctx, time.Duration(PreservationDeadline)*time.Second)
	defer cancel()
	if err := runner.Cleanup(call, contract.Cleanup); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return result, fmt.Errorf("exact cleanup exceeded its bounded deadline: %w", err)
		}
		return result, fmt.Errorf("exact cleanup failed without fallback: %w", err)
	}
	result.CleanupRun = true
	result.Completed = []string{"cleanup"}
	return result, nil
}
