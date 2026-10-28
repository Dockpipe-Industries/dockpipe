package dependency

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"
)

type PlanStep struct {
	ObservationID string   `json:"observation_id"`
	TargetVersion string   `json:"target_version,omitempty"`
	Decision      string   `json:"decision"`
	Reasons       []string `json:"reasons"`
}

// DryRunPlan is evidence for review, never an execution grant. Pins here are an
// explicit additional exact constraint; they do not replace ecosystem lockfiles.
type DryRunPlan struct {
	Schema          string              `json:"schema"`
	ID              string              `json:"id"`
	InventoryDigest string              `json:"inventory_digest"`
	Provider        ProviderIdentity    `json:"provider"`
	Machine         string              `json:"machine"`
	Policy          UpdatePolicy        `json:"policy"`
	Pin             string              `json:"pin,omitempty"`
	DryRun          bool                `json:"dry_run"`
	Executable      bool                `json:"executable"`
	Assessment      InventoryAssessment `json:"assessment"`
	Steps           []PlanStep          `json:"steps"`
	Blockers        []string            `json:"blockers"`
}

func BuildDryRunPlan(inventory Inventory, policy UpdatePolicy, pin string, now time.Time) (DryRunPlan, error) {
	if err := policy.Validate(); err != nil {
		return DryRunPlan{}, err
	}
	if !boundedLifecycleText(pin, 128) {
		return DryRunPlan{}, errors.New("invalid exact dependency pin")
	}
	assessment, err := AssessInventory(inventory, now)
	if err != nil {
		return DryRunPlan{}, err
	}
	plan := DryRunPlan{Schema: "dockpipe.dependency.plan/v1", InventoryDigest: assessment.InventoryDigest,
		Provider: inventory.Provider, Machine: inventory.Machine, Policy: policy, Pin: pin,
		DryRun: true, Executable: false, Assessment: assessment, Steps: []PlanStep{},
		Blockers: []string{
			"Inventory plans grant no execution authority. A supported local provider change requires a separate exact request and operation approval.",
			"Package and ecosystem lock constraints have not been resolved; an explicit pin cannot override them.",
		}}
	if assessment.Stale {
		plan.Blockers = append(plan.Blockers, "Inventory is stale; discover the target again before approval.")
	}
	for _, observation := range inventory.Observations {
		step := PlanStep{ObservationID: observation.ID, Decision: "review", Reasons: []string{}}
		switch observation.Assessment.Status {
		case "affected":
			step.TargetVersion = observation.Assessment.FixedVersion
			step.Reasons = append(step.Reasons, "Provider reports advisory applicability; the fixed version is guidance, not a verified artifact candidate.")
		case "not_affected":
			step.Decision = "no-advisory-remediation"
			step.Reasons = append(step.Reasons, "Not affected by the reported advisory; this does not establish latest-version or overall security status.")
		default:
			step.Decision = "unknown"
			step.Reasons = append(step.Reasons, "Provider cannot establish advisory applicability.")
		}
		if pin != "" && step.TargetVersion != "" && step.TargetVersion != pin {
			step.Decision = "pin-conflict"
			step.Reasons = append(step.Reasons, "The proposed advisory fix conflicts with the exact pin; explicit pin review is required.")
		}
		if observation.ActiveState == "unknown" || (observation.ActiveState == "running" && observation.ActiveVersion == "") {
			step.Reasons = append(step.Reasons, "Active installation version is unproven; installed bytes alone do not prove remediation.")
		}
		if policy == PolicyNotify {
			step.Reasons = append(step.Reasons, "Notify policy permits assessment only.")
		}
		if policy == PolicyCompatible || policy == PolicyLatest {
			step.Reasons = append(step.Reasons, "Compatible/latest candidate resolution is unavailable; provider advisory floors are not latest-version evidence.")
		}
		plan.Steps = append(plan.Steps, step)
	}
	data, err := json.Marshal(plan)
	if err != nil {
		return DryRunPlan{}, err
	}
	digest := sha256.Sum256(data)
	plan.ID = hex.EncodeToString(digest[:])
	return plan, nil
}
