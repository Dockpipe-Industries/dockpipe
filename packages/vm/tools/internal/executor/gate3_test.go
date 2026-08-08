package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"dockpipe.vm/tools/internal/protocol"
)

type fakeGate3Runner struct {
	calls  []string
	failAt string
}

func (f *fakeGate3Runner) call(value string) error {
	f.calls = append(f.calls, value)
	if value == f.failAt {
		return fmt.Errorf("injected Gate 3 failure")
	}
	return nil
}

func (f *fakeGate3Runner) Boot(_ context.Context, boot int) error {
	return f.call(fmt.Sprintf("boot:%d", boot))
}
func (f *fakeGate3Runner) Recover(_ context.Context, trial Gate3Trial) error {
	return f.call("recover:" + trial.TrialID)
}
func (f *fakeGate3Runner) Checkpoint(_ context.Context, trial Gate3Trial) error {
	return f.call("checkpoint:" + trial.TrialID)
}
func (f *fakeGate3Runner) HardPower(_ context.Context, trial Gate3Trial) error {
	return f.call("hard-power:" + trial.TrialID)
}
func (f *fakeGate3Runner) ControlledShutdown(context.Context) error {
	return f.call("controlled-shutdown")
}
func (f *fakeGate3Runner) Preserve(context.Context) error { return f.call("preserve") }

func TestGate3ExecutesTwelveExactTrialsAndThirteenBoots(t *testing.T) {
	execution := executorFixture(t)
	plan := gate3TestPlan(t, execution)
	now := time.Unix(1_900_000_000, 0)
	authorization := Gate3Authorization{Schema: Gate3AuthorizationSchema, Approved: true, ExecutionSHA256: execution.ExecutionSHA256, PlanSHA256: plan.PlanSHA256, RunID: plan.RunID, CohortID: plan.CohortID, TokenSHA256: strings.Repeat("a", 64), ExpiresAtUnix: now.Add(10 * time.Minute).Unix()}
	runner := &fakeGate3Runner{}
	result, err := ExecuteGate3(context.Background(), plan, execution, authorization, now, runner)
	if err != nil {
		t.Fatal(err)
	}
	trials := Gate3Trials()
	if len(trials) != 12 || result.CompletedTrials != 12 || result.HardPowerEvents != 12 || result.RecoveryResults != 12 || !result.ControlledShutdown || result.Preserved || result.CleanupRun {
		t.Fatalf("Gate 3 result changed: trials=%d result=%+v", len(trials), result)
	}
	want := make([]string, 0, 50)
	for index, trial := range trials {
		want = append(want, fmt.Sprintf("boot:%d", index+1))
		if index > 0 {
			want = append(want, "recover:"+trials[index-1].TrialID)
		}
		want = append(want, "checkpoint:"+trial.TrialID, "hard-power:"+trial.TrialID)
	}
	want = append(want, "boot:13", "recover:"+trials[11].TrialID, "controlled-shutdown")
	if !slices.Equal(runner.calls, want) {
		t.Fatalf("Gate 3 closed order changed:\n got=%v\nwant=%v", runner.calls, want)
	}
}

func TestGate3FailurePreservesAndNeverContinuesOrCleans(t *testing.T) {
	execution := executorFixture(t)
	plan := gate3TestPlan(t, execution)
	now := time.Unix(1_900_000_000, 0)
	authorization := Gate3Authorization{Schema: Gate3AuthorizationSchema, Approved: true, ExecutionSHA256: execution.ExecutionSHA256, PlanSHA256: plan.PlanSHA256, RunID: plan.RunID, CohortID: plan.CohortID, TokenSHA256: strings.Repeat("a", 64), ExpiresAtUnix: now.Add(10 * time.Minute).Unix()}
	runner := &fakeGate3Runner{failAt: "hard-power:after-stage-before-commit-2"}
	result, err := ExecuteGate3(context.Background(), plan, execution, authorization, now, runner)
	if err == nil || !result.Preserved || result.CleanupRun || runner.calls[len(runner.calls)-1] != "preserve" {
		t.Fatalf("Gate 3 failure did not stop and preserve: result=%+v err=%v calls=%v", result, err, runner.calls)
	}
	for _, call := range runner.calls {
		if call == "checkpoint:after-stage-before-commit-3" || call == "controlled-shutdown" {
			t.Fatalf("Gate 3 continued after failure: %v", runner.calls)
		}
	}
}

func TestGate3HarnessEvidenceIsIndependentlyValidated(t *testing.T) {
	trial := Gate3Trial{Boundary: "after-commit-before-reload", Attempt: 2, TrialID: "after-commit-before-reload-2"}
	cohortID := "cohort-001"
	root := filepath.Join("/var/lib/dockpipe-qualification/cohorts", cohortID, trial.Boundary, "attempt-2")
	evidence := map[string]any{
		"schema": "dockpipe.sqlite-vm-recovery-evidence.v1", "cohort_id": cohortID, "trial_id": trial.TrialID,
		"boundary": trial.Boundary, "attempt": trial.Attempt, "root": root, "root_identity": "123:456",
		"database": filepath.Join(root, "sqlite", "main", "aggregate.sqlite"), "expected_revision": 2, "observed_revision": 2,
		"observed_digest": strings.Repeat("a", 64), "pre_metadata_sha256": strings.Repeat("b", 64), "post_metadata_sha256": strings.Repeat("c", 64),
		"compile_options_sha256": strings.Repeat("d", 64), "sqlite_version": "3.53.3", "sqlite_source_id": "2026-06-26 20:14:12 d4c0e51e4aeb96955b99185ab9cde75c339e2c29c3f3f12428d364a10d782c62",
		"vfs": "unix", "quick_check": "ok", "retries": 0, "replays": 0, "repairs": 0, "fallbacks": 0,
	}
	raw, err := json.Marshal(evidence)
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := protocol.Canonicalize(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateGate3HarnessEvidence(canonical, cohortID, trial, true); err != nil {
		t.Fatalf("valid recovery evidence rejected: %v", err)
	}
	evidence["observed_revision"] = 1
	raw, _ = json.Marshal(evidence)
	canonical, _ = protocol.Canonicalize(raw)
	if err := validateGate3HarnessEvidence(canonical, cohortID, trial, true); err == nil {
		t.Fatal("expected observed revision substitution rejection")
	}
}

func gate3TestPlan(t *testing.T, execution Contract) Gate3Plan {
	t.Helper()
	plan := Gate3Plan{
		Schema: Gate3PlanSchema, ExecutionSHA256: execution.ExecutionSHA256, ContractSHA256: execution.ContractSHA256, ProvisioningSHA256: execution.PlanSHA256,
		RunID: execution.RunID, CohortID: execution.CohortID, MachineUUID: "11111111-1111-4111-8111-111111111111", DiskSerial: "dockpipe-data-000001", Scenario: "sqlite-rollback-recovery",
		HarnessSHA256: strings.Repeat("f", 64), Boundaries: slices.Clone(Gate3Boundaries), TrialsPerBoundary: Gate3TrialsPerBoundary,
		Launch: execution.Launch.Command, QMP: execution.Launch.QMP, AgentSocket: execution.Launch.AgentSocket, ConsoleSocket: execution.FirstBootObservation.SocketPath,
		EvidenceRoot: filepath.Join(filepath.Dir(execution.Guest.Evidence), "gate3"), BootTimeoutSeconds: Gate3BootTimeoutSeconds, ActionTimeoutSeconds: Gate3ActionTimeoutSeconds, PowerTimeoutSeconds: Gate3PowerTimeoutSeconds,
	}
	plan.PlanSHA256, _ = plan.Digest()
	if err := plan.Validate(execution); err != nil {
		t.Fatal(err)
	}
	return plan
}
