package executor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"time"

	"dockpipe.vm/tools/internal/manifest"
	"dockpipe.vm/tools/internal/provisioning"
)

const (
	Gate3PlanSchema              = "dockpipe.vm.gate3-plan.v1"
	Gate3ReconstitutedPlanSchema = "dockpipe.vm.gate3-plan.v2"
	Gate3AuthorizationSchema     = "dockpipe.vm.gate3-authorization.v1"
	Gate3ResultSchema            = "dockpipe.vm.gate3-result.v1"
	Gate3TrialsPerBoundary       = 3
	Gate3BootTimeoutSeconds      = 240
	Gate3ActionTimeoutSeconds    = 60
	Gate3PowerTimeoutSeconds     = 30
)

var Gate3Boundaries = []string{
	"after-stage-before-commit",
	"inside-commit-hook-before-phase1",
	"after-commit-before-reload",
	"after-validation-before-ack",
}

type Gate3Plan struct {
	Schema               string                     `json:"schema"`
	ReconstitutionSHA256 string                     `json:"reconstitution_sha256,omitempty"`
	ExecutionSHA256      string                     `json:"execution_sha256"`
	ContractSHA256       string                     `json:"contract_sha256"`
	ProvisioningSHA256   string                     `json:"provisioning_sha256"`
	PlanSHA256           string                     `json:"plan_sha256"`
	RunID                string                     `json:"run_id"`
	CohortID             string                     `json:"cohort_id"`
	MachineUUID          string                     `json:"machine_uuid"`
	DiskSerial           string                     `json:"disk_serial"`
	Scenario             string                     `json:"scenario"`
	HarnessSHA256        string                     `json:"harness_sha256"`
	Boundaries           []string                   `json:"boundaries"`
	TrialsPerBoundary    int                        `json:"trials_per_boundary"`
	Launch               provisioning.PinnedCommand `json:"launch"`
	QMP                  string                     `json:"qmp"`
	AgentSocket          string                     `json:"agent_socket"`
	ConsoleSocket        string                     `json:"console_socket"`
	EvidenceRoot         string                     `json:"evidence_root"`
	BootTimeoutSeconds   int                        `json:"boot_timeout_seconds"`
	ActionTimeoutSeconds int                        `json:"action_timeout_seconds"`
	PowerTimeoutSeconds  int                        `json:"power_timeout_seconds"`
	Execute              bool                       `json:"execute"`
}

type Gate3Authorization struct {
	Schema          string `json:"schema"`
	Approved        bool   `json:"approved"`
	ExecutionSHA256 string `json:"execution_sha256"`
	PlanSHA256      string `json:"plan_sha256"`
	RunID           string `json:"run_id"`
	CohortID        string `json:"cohort_id"`
	TokenSHA256     string `json:"token_sha256"`
	ExpiresAtUnix   int64  `json:"expires_at_unix"`
}

type Gate3Trial struct {
	Boundary string `json:"boundary"`
	Attempt  int    `json:"attempt"`
	TrialID  string `json:"trial_id"`
}

type Gate3Result struct {
	Schema             string `json:"schema"`
	ExecutionSHA256    string `json:"execution_sha256"`
	PlanSHA256         string `json:"plan_sha256"`
	CompletedTrials    int    `json:"completed_trials"`
	HardPowerEvents    int    `json:"hard_power_events"`
	RecoveryResults    int    `json:"recovery_results"`
	ControlledShutdown bool   `json:"controlled_shutdown"`
	Preserved          bool   `json:"preserved"`
	CleanupRun         bool   `json:"cleanup_run"`
}

type Gate3Runner interface {
	Boot(context.Context, int) error
	Recover(context.Context, Gate3Trial) error
	Checkpoint(context.Context, Gate3Trial) error
	HardPower(context.Context, Gate3Trial) error
	ControlledShutdown(context.Context) error
	Preserve(context.Context) error
}

func BuildGate3Plan(execution Contract, contract provisioning.Contract, qualification manifest.Manifest) (Gate3Plan, error) {
	var plan Gate3Plan
	if err := execution.Validate(); err != nil || execution.Schema != Schema {
		return plan, fmt.Errorf("Gate 3 requires the current qualified executor contract")
	}
	contractSHA, err := contract.Digest()
	if err != nil || contractSHA != execution.ContractSHA256 || contract.RunID != execution.RunID || contract.CohortID != execution.CohortID {
		return plan, fmt.Errorf("Gate 3 provisioning contract does not match the executor")
	}
	if err := qualification.Validate(); err != nil || qualification.RunID != execution.RunID || qualification.MachineUUID != contract.MachineUUID || qualification.DataDisk.Serial != contract.DiskSerial || qualification.Protocol.HarnessSHA256 != contract.Artifacts.HarnessBinarySHA256 {
		return plan, fmt.Errorf("Gate 3 qualification manifest or harness pin does not match")
	}
	if execution.ProvisioningRoots == nil || execution.FirstBootObservation == nil {
		return plan, fmt.Errorf("Gate 3 requires current provisioning roots and console transport")
	}
	evidenceRoot := filepath.Join(execution.ProvisioningRoots.Evidence, execution.RunID, execution.CohortID, "gate3")
	plan = Gate3Plan{
		Schema: Gate3PlanSchema, ExecutionSHA256: execution.ExecutionSHA256, ContractSHA256: execution.ContractSHA256,
		ProvisioningSHA256: execution.PlanSHA256, RunID: execution.RunID, CohortID: execution.CohortID,
		MachineUUID: qualification.MachineUUID, DiskSerial: qualification.DataDisk.Serial, Scenario: qualification.Scenario,
		HarnessSHA256: contract.Artifacts.HarnessBinarySHA256, Boundaries: slices.Clone(Gate3Boundaries), TrialsPerBoundary: Gate3TrialsPerBoundary,
		Launch: execution.Launch.Command, QMP: execution.Launch.QMP, AgentSocket: execution.Launch.AgentSocket,
		ConsoleSocket: execution.FirstBootObservation.SocketPath, EvidenceRoot: evidenceRoot,
		BootTimeoutSeconds: Gate3BootTimeoutSeconds, ActionTimeoutSeconds: Gate3ActionTimeoutSeconds, PowerTimeoutSeconds: Gate3PowerTimeoutSeconds,
	}
	plan.PlanSHA256, err = plan.Digest()
	if err != nil {
		return Gate3Plan{}, err
	}
	return plan, plan.Validate(execution)
}

// BuildGate3PlanFromReconstitution creates an inert plan from authenticated
// historical proof. The resulting v2 plan is intentionally not executable.
func BuildGate3PlanFromReconstitution(execution Contract, reconstitution Gate3Reconstitution, executorFileSHA256 string) (Gate3Plan, error) {
	var plan Gate3Plan
	if err := execution.Validate(); err != nil || execution.Schema != Schema {
		return plan, fmt.Errorf("Gate 3 requires the current qualified executor contract")
	}
	if err := reconstitution.Validate(execution, executorFileSHA256); err != nil {
		return plan, err
	}
	if execution.ProvisioningRoots == nil || execution.FirstBootObservation == nil {
		return plan, fmt.Errorf("Gate 3 requires current provisioning roots and console transport")
	}
	plan = Gate3Plan{
		Schema: Gate3ReconstitutedPlanSchema, ReconstitutionSHA256: reconstitution.ReconstitutionSHA256,
		ExecutionSHA256: execution.ExecutionSHA256, ContractSHA256: execution.ContractSHA256, ProvisioningSHA256: execution.PlanSHA256,
		RunID: execution.RunID, CohortID: execution.CohortID, MachineUUID: reconstitution.MachineUUID, DiskSerial: reconstitution.DiskSerial, Scenario: reconstitution.Scenario,
		HarnessSHA256: reconstitution.HarnessSHA256, Boundaries: slices.Clone(Gate3Boundaries), TrialsPerBoundary: Gate3TrialsPerBoundary,
		Launch: execution.Launch.Command, QMP: execution.Launch.QMP, AgentSocket: execution.Launch.AgentSocket, ConsoleSocket: execution.FirstBootObservation.SocketPath,
		EvidenceRoot:       filepath.Join(execution.ProvisioningRoots.Evidence, execution.RunID, execution.CohortID, "gate3"),
		BootTimeoutSeconds: Gate3BootTimeoutSeconds, ActionTimeoutSeconds: Gate3ActionTimeoutSeconds, PowerTimeoutSeconds: Gate3PowerTimeoutSeconds,
	}
	var err error
	plan.PlanSHA256, err = plan.Digest()
	if err != nil {
		return Gate3Plan{}, err
	}
	return plan, plan.Validate(execution)
}

func (p Gate3Plan) Digest() (string, error) {
	copy := p
	copy.PlanSHA256 = ""
	copy.Execute = false
	b, err := json.Marshal(copy)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(b)
	return hex.EncodeToString(digest[:]), nil
}

func (p Gate3Plan) Validate(execution Contract) error {
	digest, err := p.Digest()
	legacyShape := p.Schema == Gate3PlanSchema && p.ReconstitutionSHA256 == ""
	reconstitutedShape := p.Schema == Gate3ReconstitutedPlanSchema && isSHA256(p.ReconstitutionSHA256)
	if err != nil || (!legacyShape && !reconstitutedShape) || p.PlanSHA256 != digest || p.Execute || p.ExecutionSHA256 != execution.ExecutionSHA256 || p.ContractSHA256 != execution.ContractSHA256 || p.ProvisioningSHA256 != execution.PlanSHA256 || p.RunID != execution.RunID || p.CohortID != execution.CohortID || !isSHA256(p.HarnessSHA256) {
		return fmt.Errorf("Gate 3 plan identity or digest is invalid")
	}
	if !slices.Equal(p.Boundaries, Gate3Boundaries) || p.TrialsPerBoundary != Gate3TrialsPerBoundary || p.BootTimeoutSeconds != Gate3BootTimeoutSeconds || p.ActionTimeoutSeconds != Gate3ActionTimeoutSeconds || p.PowerTimeoutSeconds != Gate3PowerTimeoutSeconds {
		return fmt.Errorf("Gate 3 boundaries, counts, or timeouts changed")
	}
	if !reflect.DeepEqual(p.Launch, execution.Launch.Command) || p.QMP != execution.Launch.QMP || p.AgentSocket != execution.Launch.AgentSocket || execution.FirstBootObservation == nil || p.ConsoleSocket != execution.FirstBootObservation.SocketPath {
		return fmt.Errorf("Gate 3 launch tuple changed")
	}
	wantEvidence := filepath.Join(filepath.Dir(execution.Guest.Evidence), "gate3")
	if p.EvidenceRoot != wantEvidence || !filepath.IsAbs(p.EvidenceRoot) || filepath.Clean(p.EvidenceRoot) != p.EvidenceRoot {
		return fmt.Errorf("Gate 3 evidence root changed")
	}
	return nil
}

func LoadGate3Plan(path string) (Gate3Plan, error) {
	var plan Gate3Plan
	if err := decodeGate3File(path, &plan); err != nil {
		return plan, err
	}
	return plan, nil
}

func LoadGate3Authorization(path string) (Gate3Authorization, error) {
	var authorization Gate3Authorization
	if err := decodeGate3File(path, &authorization); err != nil {
		return authorization, err
	}
	return authorization, nil
}

func (a Gate3Authorization) Validate(plan Gate3Plan, execution Contract, now time.Time) error {
	if plan.Schema != Gate3PlanSchema || plan.ReconstitutionSHA256 != "" {
		return fmt.Errorf("reconstituted Gate 3 plans cannot be authorized")
	}
	if a.Schema != Gate3AuthorizationSchema || !a.Approved || a.ExecutionSHA256 != execution.ExecutionSHA256 || a.PlanSHA256 != plan.PlanSHA256 || a.RunID != plan.RunID || a.CohortID != plan.CohortID || !isSHA256(a.TokenSHA256) {
		return fmt.Errorf("Gate 3 authorization does not match the exact plan")
	}
	if a.ExpiresAtUnix <= now.Unix() || a.ExpiresAtUnix > now.Add(15*time.Minute).Unix() {
		return fmt.Errorf("Gate 3 authorization must be fresh and expire within 15 minutes")
	}
	return nil
}

func ExecuteGate3(ctx context.Context, plan Gate3Plan, execution Contract, authorization Gate3Authorization, now time.Time, runner Gate3Runner) (Gate3Result, error) {
	result := Gate3Result{Schema: Gate3ResultSchema, ExecutionSHA256: execution.ExecutionSHA256, PlanSHA256: plan.PlanSHA256}
	if plan.Schema != Gate3PlanSchema || plan.ReconstitutionSHA256 != "" {
		return result, fmt.Errorf("reconstituted Gate 3 plans are inert and cannot execute")
	}
	if runner == nil {
		return result, fmt.Errorf("typed Gate 3 runner is required")
	}
	if err := plan.Validate(execution); err != nil {
		return result, err
	}
	if err := authorization.Validate(plan, execution, now); err != nil {
		return result, err
	}
	trials := Gate3Trials()
	var previous *Gate3Trial
	for index := range trials {
		if err := gate3Call(ctx, plan.BootTimeoutSeconds, func(call context.Context) error { return runner.Boot(call, index+1) }); err != nil {
			return preserveGate3(ctx, runner, result, fmt.Errorf("Gate 3 boot %d: %w", index+1, err))
		}
		if previous != nil {
			if err := gate3Call(ctx, plan.ActionTimeoutSeconds, func(call context.Context) error { return runner.Recover(call, *previous) }); err != nil {
				return preserveGate3(ctx, runner, result, fmt.Errorf("Gate 3 recovery %s: %w", previous.TrialID, err))
			}
			result.RecoveryResults++
		}
		trial := trials[index]
		if err := gate3Call(ctx, plan.ActionTimeoutSeconds, func(call context.Context) error { return runner.Checkpoint(call, trial) }); err != nil {
			return preserveGate3(ctx, runner, result, fmt.Errorf("Gate 3 checkpoint %s: %w", trial.TrialID, err))
		}
		if err := gate3Call(ctx, plan.PowerTimeoutSeconds, func(call context.Context) error { return runner.HardPower(call, trial) }); err != nil {
			return preserveGate3(ctx, runner, result, fmt.Errorf("Gate 3 hard power %s: %w", trial.TrialID, err))
		}
		result.HardPowerEvents++
		result.CompletedTrials++
		previous = &trial
	}
	if err := gate3Call(ctx, plan.BootTimeoutSeconds, func(call context.Context) error { return runner.Boot(call, len(trials)+1) }); err != nil {
		return preserveGate3(ctx, runner, result, fmt.Errorf("Gate 3 final recovery boot: %w", err))
	}
	if previous == nil {
		return result, fmt.Errorf("Gate 3 trial set is empty")
	}
	if err := gate3Call(ctx, plan.ActionTimeoutSeconds, func(call context.Context) error { return runner.Recover(call, *previous) }); err != nil {
		return preserveGate3(ctx, runner, result, fmt.Errorf("Gate 3 final recovery %s: %w", previous.TrialID, err))
	}
	result.RecoveryResults++
	if err := gate3Call(ctx, execution.Shutdown.TimeoutSeconds, runner.ControlledShutdown); err != nil {
		return preserveGate3(ctx, runner, result, fmt.Errorf("Gate 3 controlled shutdown: %w", err))
	}
	result.ControlledShutdown = true
	return result, nil
}

func StoreGate3Result(plan Gate3Plan, result Gate3Result) error {
	if plan.Schema != Gate3PlanSchema || plan.ReconstitutionSHA256 != "" {
		return fmt.Errorf("reconstituted Gate 3 plans cannot store execution results")
	}
	if result.Schema != Gate3ResultSchema || result.ExecutionSHA256 != plan.ExecutionSHA256 || result.PlanSHA256 != plan.PlanSHA256 || result.CompletedTrials != len(Gate3Trials()) || result.HardPowerEvents != len(Gate3Trials()) || result.RecoveryResults != len(Gate3Trials()) || !result.ControlledShutdown || result.Preserved || result.CleanupRun {
		return fmt.Errorf("Gate 3 result is incomplete")
	}
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return durableExclusive(filepath.Join(plan.EvidenceRoot, "result.json"), data, 0o600)
}

func Gate3Trials() []Gate3Trial {
	trials := make([]Gate3Trial, 0, len(Gate3Boundaries)*Gate3TrialsPerBoundary)
	for _, boundary := range Gate3Boundaries {
		for attempt := 1; attempt <= Gate3TrialsPerBoundary; attempt++ {
			trials = append(trials, Gate3Trial{Boundary: boundary, Attempt: attempt, TrialID: fmt.Sprintf("%s-%d", boundary, attempt)})
		}
	}
	return trials
}

func gate3Call(ctx context.Context, seconds int, run func(context.Context) error) error {
	call, cancel := context.WithTimeout(ctx, time.Duration(seconds)*time.Second)
	defer cancel()
	return run(call)
}

func preserveGate3(ctx context.Context, runner Gate3Runner, result Gate3Result, cause error) (Gate3Result, error) {
	preserve, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	if err := runner.Preserve(preserve); err != nil {
		return result, fmt.Errorf("%w; preserve complete Gate 3 instance: %v", cause, err)
	}
	result.Preserved = true
	return result, fmt.Errorf("%w; complete Gate 3 instance preserved", cause)
}

func decodeGate3File(path string, target any) error {
	return decodeOwnerOnly(path, target)
}
