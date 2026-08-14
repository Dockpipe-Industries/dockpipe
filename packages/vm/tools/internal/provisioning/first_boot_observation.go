package provisioning

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"dockpipe.vm/tools/internal/manifest"
)

const (
	FirstBootObservationSchema               = "dockpipe.vm.first-boot-observation.v1"
	FirstBootObservationGuestSource          = "isa-serial/ttyS0"
	FirstBootObservationSink                 = "controller-owned-bounded-file"
	FirstBootObservationTransport            = "unix-stream-qemu-client"
	FirstBootObservationFilename             = "first-boot-console.log"
	FirstBootObservationSocketFilename       = "first-boot-console.sock"
	FirstBootObservationMaxBytes       int64 = 4 * 1024 * 1024
	FirstBootObservationOverflowPolicy       = "fail-closed-preserve-prefix"
)

// FirstBootObservationPlan is an inert reviewed policy bound into fresh
// provisioning and executor plans. It grants no authority by itself.
type FirstBootObservationPlan struct {
	Schema                string `json:"schema"`
	RunID                 string `json:"run_id"`
	CohortID              string `json:"cohort_id"`
	EvidenceRoot          string `json:"evidence_root"`
	RuntimeRoot           string `json:"runtime_root"`
	GuestSource           string `json:"guest_source"`
	Sink                  string `json:"sink"`
	Transport             string `json:"transport"`
	EvidencePath          string `json:"evidence_path"`
	SocketPath            string `json:"socket_path"`
	EvidenceMode          uint32 `json:"evidence_mode"`
	EvidenceExclusive     bool   `json:"evidence_exclusive"`
	SocketMode            uint32 `json:"socket_mode"`
	SocketExclusive       bool   `json:"socket_exclusive"`
	MaxBytes              int64  `json:"max_bytes"`
	OverflowPolicy        string `json:"overflow_policy"`
	FsyncOnFailure        bool   `json:"fsync_on_failure"`
	PassiveCapture        bool   `json:"passive_capture"`
	ControllerListener    bool   `json:"controller_listener"`
	QEMUClient            bool   `json:"qemu_client"`
	Reconnect             bool   `json:"reconnect"`
	StopAndJoin           bool   `json:"stop_and_join"`
	FsyncParentOnFailure  bool   `json:"fsync_parent_on_failure"`
	SeedMutation          bool   `json:"seed_mutation"`
	PrivatePayloadRead    bool   `json:"private_payload_read"`
	Network               bool   `json:"network"`
	Execute               bool   `json:"execute"`
	AuthorizationRequired bool   `json:"authorization_required"`
}

func PlanFirstBootObservation(evidenceRoot, runtimeRoot, runID, cohortID string) (FirstBootObservationPlan, error) {
	for label, root := range map[string]string{"evidence": evidenceRoot, "runtime": runtimeRoot} {
		if !filepath.IsAbs(root) || filepath.Clean(root) != root || strings.ContainsAny(root, ",\r\n") {
			return FirstBootObservationPlan{}, fmt.Errorf("first-boot observation %s root must be absolute, clean, and QEMU-safe", label)
		}
	}
	if !idPattern.MatchString(runID) || !idPattern.MatchString(cohortID) {
		return FirstBootObservationPlan{}, fmt.Errorf("first-boot observation run and cohort identities are invalid")
	}
	plan := FirstBootObservationPlan{
		Schema:                FirstBootObservationSchema,
		RunID:                 runID,
		CohortID:              cohortID,
		EvidenceRoot:          evidenceRoot,
		RuntimeRoot:           runtimeRoot,
		GuestSource:           FirstBootObservationGuestSource,
		Sink:                  FirstBootObservationSink,
		Transport:             FirstBootObservationTransport,
		EvidencePath:          filepath.Join(evidenceRoot, runID, cohortID, FirstBootObservationFilename),
		SocketPath:            filepath.Join(runtimeRoot, runID, cohortID, FirstBootObservationSocketFilename),
		EvidenceMode:          0o600,
		EvidenceExclusive:     true,
		SocketMode:            0o600,
		SocketExclusive:       true,
		MaxBytes:              FirstBootObservationMaxBytes,
		OverflowPolicy:        FirstBootObservationOverflowPolicy,
		FsyncOnFailure:        true,
		PassiveCapture:        true,
		ControllerListener:    true,
		QEMUClient:            true,
		StopAndJoin:           true,
		FsyncParentOnFailure:  true,
		AuthorizationRequired: true,
	}
	return plan, plan.Validate()
}

func (p FirstBootObservationPlan) Validate() error {
	if p.Schema != FirstBootObservationSchema || !idPattern.MatchString(p.RunID) || !idPattern.MatchString(p.CohortID) {
		return fmt.Errorf("first-boot observation identity is invalid")
	}
	if p.GuestSource != FirstBootObservationGuestSource || p.Sink != FirstBootObservationSink || p.Transport != FirstBootObservationTransport || p.EvidenceMode != 0o600 || !p.EvidenceExclusive || p.SocketMode != 0o600 || !p.SocketExclusive || p.MaxBytes != FirstBootObservationMaxBytes || p.OverflowPolicy != FirstBootObservationOverflowPolicy || !p.FsyncOnFailure || !p.FsyncParentOnFailure || !p.PassiveCapture || !p.ControllerListener || !p.QEMUClient || p.Reconnect || !p.StopAndJoin {
		return fmt.Errorf("first-boot observation capture policy changed")
	}
	if !filepath.IsAbs(p.EvidenceRoot) || filepath.Clean(p.EvidenceRoot) != p.EvidenceRoot || strings.ContainsAny(p.EvidenceRoot, ",\r\n") || p.EvidencePath != filepath.Join(p.EvidenceRoot, p.RunID, p.CohortID, FirstBootObservationFilename) || !filepath.IsAbs(p.RuntimeRoot) || filepath.Clean(p.RuntimeRoot) != p.RuntimeRoot || strings.ContainsAny(p.RuntimeRoot, ",\r\n") || p.SocketPath != filepath.Join(p.RuntimeRoot, p.RunID, p.CohortID, FirstBootObservationSocketFilename) {
		return fmt.Errorf("first-boot observation evidence path is invalid")
	}
	if err := manifest.ValidateLinuxUnixSocketPath("first-boot console", p.SocketPath); err != nil {
		return err
	}
	if p.SeedMutation || p.PrivatePayloadRead || p.Network || p.Execute || !p.AuthorizationRequired {
		return fmt.Errorf("first-boot observation plan must remain passive, inert, and separately authorized")
	}
	return nil
}

func (p FirstBootObservationPlan) CanonicalJSON() (string, error) {
	if err := p.Validate(); err != nil {
		return "", err
	}
	b, err := json.Marshal(p)
	return string(b), err
}
