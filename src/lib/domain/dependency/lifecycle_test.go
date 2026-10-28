package dependency

import (
	"strings"
	"testing"
	"time"
)

func lifecycleFixture() Inventory {
	return Inventory{Schema: InventorySchema, Provider: ProviderIdentity{ID: "fixture.tool", Version: "1.2.3"},
		Machine: "test-host", ObservedAt: time.Now().UTC(), Gaps: []string{}, Observations: []Observation{{
			ID: "install-one", Dependency: "example-tool", Location: "/tools/example", Method: "native",
			InstalledVersion: "1.0", ActiveState: "unknown",
			Assessment: AdvisoryAssessment{Status: "affected", FixedVersion: "1.1", AdvisoryURL: "https://vendor.example/advisory", Reason: "fixture advisory"},
		}}}
}

func TestLifecyclePoliciesNeverOverridePinsOrEnableExecution(t *testing.T) {
	for _, policy := range []UpdatePolicy{PolicyManual, PolicyNotify, PolicySecurityOnly, PolicyCompatible, PolicyLatest} {
		t.Run(string(policy), func(t *testing.T) {
			inventory := lifecycleFixture()
			before := inventory.Observations[0].InstalledVersion
			plan, err := BuildDryRunPlan(inventory, policy, "1.0", time.Now())
			if err != nil || plan.Executable || !plan.DryRun || plan.Steps[0].Decision != "pin-conflict" || len(plan.Blockers) == 0 {
				t.Fatalf("unsafe plan: %+v, %v", plan, err)
			}
			again, err := BuildDryRunPlan(inventory, policy, "1.0", time.Now())
			if err != nil || again.ID != plan.ID || inventory.Observations[0].InstalledVersion != before {
				t.Fatal("dry-run changed its source or immutable identity")
			}
			inventory.Machine = "other-host"
			changed, err := BuildDryRunPlan(inventory, policy, "1.0", time.Now())
			if err != nil || changed.ID == plan.ID {
				t.Fatal("plan identity did not bind the target")
			}
		})
	}
}

func TestLifecycleInstalledFixDoesNotClearUnknownActiveStateOrCoverage(t *testing.T) {
	inventory := lifecycleFixture()
	inventory.Observations[0].Assessment.Status = "not_affected"
	assessment, err := AssessInventory(inventory, time.Now())
	if err != nil || assessment.Status != "unknown" {
		t.Fatalf("unknown active version became clean: %+v, %v", assessment, err)
	}
	inventory.Observations[0].ActiveState = "stopped"
	inventory.Gaps = []string{"service inventory inaccessible"}
	assessment, err = AssessInventory(inventory, time.Now())
	if err != nil || assessment.Status != "unknown" {
		t.Fatal("incomplete coverage became clean")
	}
	inventory.Gaps = nil
	inventory.ObservedAt = time.Now().Add(-25 * time.Hour)
	assessment, err = AssessInventory(inventory, time.Now())
	if err != nil || !assessment.Stale || assessment.Status != "unknown" {
		t.Fatal("stale evidence became clean")
	}
}

func TestLifecycleRejectsMalformedOrAmbiguousEvidence(t *testing.T) {
	for _, mutate := range []func(*Inventory){
		func(value *Inventory) { value.Schema = "future" },
		func(value *Inventory) { value.Machine = "../other" },
		func(value *Inventory) { value.Observations = append(value.Observations, value.Observations[0]) },
		func(value *Inventory) { value.Observations[0].Assessment.Status = "safe" },
		func(value *Inventory) {
			value.Observations[0].Assessment.AdvisoryURL = "https://secret@vendor.example/advisory"
		},
		func(value *Inventory) { value.Observations[0].InstalledVersion = strings.Repeat("x", 129) },
		func(value *Inventory) { value.ObservedAt = time.Now().Add(time.Hour) },
	} {
		inventory := lifecycleFixture()
		mutate(&inventory)
		if err := inventory.Validate(time.Now()); err == nil {
			t.Fatal("invalid lifecycle evidence accepted")
		}
	}
	if _, err := BuildDryRunPlan(lifecycleFixture(), "automatic", "", time.Now()); err == nil {
		t.Fatal("unknown policy accepted")
	}
}
