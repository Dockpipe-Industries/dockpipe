package dependency

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const InventorySchema = "dockpipe.dependency.inventory/v1"
const MaxInventoryBytes = 1 << 20

var lifecycleIdentifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

type ProviderIdentity struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

type AdvisoryAssessment struct {
	Status       string `json:"status"`
	AdvisoryURL  string `json:"advisory_url"`
	FixedVersion string `json:"fixed_version"`
	Reason       string `json:"reason"`
}

type Observation struct {
	ID               string             `json:"id"`
	Dependency       string             `json:"dependency"`
	Location         string             `json:"location"`
	Method           string             `json:"method"`
	InstalledVersion string             `json:"installed_version"`
	ActiveVersion    string             `json:"active_version"`
	ActiveState      string             `json:"active_state"`
	Assessment       AdvisoryAssessment `json:"assessment"`
}

type Inventory struct {
	Schema       string           `json:"schema"`
	Provider     ProviderIdentity `json:"provider"`
	Machine      string           `json:"machine"`
	ObservedAt   time.Time        `json:"observed_at"`
	Observations []Observation    `json:"observations"`
	Gaps         []string         `json:"gaps"`
}

func ValidLifecycleIdentifier(value string) bool {
	return lifecycleIdentifier.MatchString(value)
}

func boundedLifecycleText(value string, limit int) bool {
	return len(value) <= limit && !strings.ContainsAny(value, "\x00\r\n")
}

func (inventory Inventory) Validate(now time.Time) error {
	if inventory.Schema != InventorySchema || !ValidLifecycleIdentifier(inventory.Provider.ID) ||
		inventory.Provider.Version == "" || !boundedLifecycleText(inventory.Provider.Version, 128) ||
		!ValidLifecycleIdentifier(inventory.Machine) || inventory.ObservedAt.IsZero() ||
		inventory.ObservedAt.After(now.Add(5*time.Minute)) {
		return errors.New("invalid dependency inventory identity, schema or observation time")
	}
	if len(inventory.Observations) > 128 || len(inventory.Gaps) > 64 {
		return errors.New("dependency inventory exceeds observation or coverage limits")
	}
	seen := map[string]bool{}
	for _, observation := range inventory.Observations {
		if !ValidLifecycleIdentifier(observation.ID) || !ValidLifecycleIdentifier(observation.Dependency) || seen[observation.ID] {
			return errors.New("invalid or duplicate dependency observation identity")
		}
		seen[observation.ID] = true
		if observation.Location == "" || !boundedLifecycleText(observation.Location, 4096) ||
			observation.Method == "" || !boundedLifecycleText(observation.Method, 128) ||
			!boundedLifecycleText(observation.InstalledVersion, 128) || !boundedLifecycleText(observation.ActiveVersion, 128) {
			return errors.New("invalid dependency installation observation")
		}
		switch observation.ActiveState {
		case "running", "stopped", "unknown":
		default:
			return errors.New("invalid dependency active state")
		}
		assessment := observation.Assessment
		switch assessment.Status {
		case "affected", "not_affected", "unknown":
		default:
			return errors.New("invalid dependency advisory assessment")
		}
		if !boundedLifecycleText(assessment.Reason, 2048) || !boundedLifecycleText(assessment.FixedVersion, 128) ||
			!boundedLifecycleText(assessment.AdvisoryURL, 2048) {
			return errors.New("dependency advisory evidence exceeds bounds")
		}
		if assessment.Status != "unknown" && assessment.AdvisoryURL == "" {
			return errors.New("a known advisory assessment requires its source")
		}
		if assessment.Status == "not_affected" && observation.InstalledVersion == "" && observation.ActiveVersion == "" {
			return errors.New("an unknown version cannot establish advisory remediation")
		}
		if assessment.AdvisoryURL != "" {
			source, err := url.Parse(assessment.AdvisoryURL)
			if err != nil || source.Scheme != "https" || source.Hostname() == "" || source.User != nil || source.Fragment != "" {
				return errors.New("advisory source must be an HTTPS URL without credentials")
			}
		}
	}
	for _, gap := range inventory.Gaps {
		if gap == "" || !boundedLifecycleText(gap, 2048) {
			return errors.New("invalid dependency inventory coverage gap")
		}
	}
	return nil
}

type InventoryAssessment struct {
	Schema          string `json:"schema"`
	InventoryDigest string `json:"inventory_digest"`
	Status          string `json:"status"`
	Affected        int    `json:"affected"`
	Unknown         int    `json:"unknown"`
	Stale           bool   `json:"stale"`
}

func AssessInventory(inventory Inventory, now time.Time) (InventoryAssessment, error) {
	if err := inventory.Validate(now); err != nil {
		return InventoryAssessment{}, err
	}
	data, err := json.Marshal(inventory)
	if err != nil {
		return InventoryAssessment{}, err
	}
	digest := sha256.Sum256(data)
	result := InventoryAssessment{Schema: "dockpipe.dependency.assessment/v1", InventoryDigest: hex.EncodeToString(digest[:]), Status: "not_affected"}
	result.Stale = now.Sub(inventory.ObservedAt) > 24*time.Hour
	for _, observation := range inventory.Observations {
		if observation.Assessment.Status == "affected" {
			result.Affected++
		}
		if observation.Assessment.Status == "unknown" || observation.ActiveState == "unknown" ||
			(observation.ActiveState == "running" && observation.ActiveVersion == "") {
			result.Unknown++
		}
	}
	if result.Unknown > 0 || result.Stale || len(inventory.Gaps) > 0 || len(inventory.Observations) == 0 {
		result.Status = "unknown"
	}
	if result.Affected > 0 {
		result.Status = "affected"
	}
	return result, nil
}

type UpdatePolicy string

const (
	PolicyManual       UpdatePolicy = "manual"
	PolicyNotify       UpdatePolicy = "notify"
	PolicySecurityOnly UpdatePolicy = "security-only"
	PolicyCompatible   UpdatePolicy = "compatible"
	PolicyLatest       UpdatePolicy = "latest"
)

func (policy UpdatePolicy) Validate() error {
	switch policy {
	case PolicyManual, PolicyNotify, PolicySecurityOnly, PolicyCompatible, PolicyLatest:
		return nil
	default:
		return fmt.Errorf("unsupported dependency policy %q", policy)
	}
}
