package guest

import (
	"encoding/json"
	"fmt"
	"io"

	"dockpipe.vm/tools/internal/protocol"
)

const (
	checkpointObservationSchema    = "dockpipe.vm.gate3-checkpoint-observation.v1"
	checkpointObservationPrefix    = "dockpipe-gate3-checkpoint-observation "
	checkpointStageRequestReceived = "request-received"
	checkpointStagePendingAccepted = "pending-ticket-accepted"
	checkpointStageHarnessEmitted  = "harness-evidence-emitted"
)

type checkpointObservation struct {
	Schema         string `json:"schema"`
	Stage          string `json:"stage"`
	RunID          string `json:"run_id"`
	CohortID       string `json:"cohort_id"`
	TrialID        string `json:"trial_id"`
	Boundary       string `json:"boundary"`
	Attempt        int    `json:"attempt"`
	BootID         string `json:"boot_id"`
	TicketSHA256   string `json:"ticket_sha256,omitempty"`
	EvidenceSHA256 string `json:"evidence_sha256,omitempty"`
}

func (s *Service) writeCheckpointObservation(request HarnessRequest, stage, ticketSHA256, evidenceSHA256 string) error {
	if s.Observability == nil {
		return fmt.Errorf("Gate 3 checkpoint observability is unavailable")
	}
	observation := checkpointObservation{
		Schema: checkpointObservationSchema, Stage: stage, RunID: request.RunID,
		CohortID: request.CohortID, TrialID: request.TrialID, Boundary: request.Boundary,
		Attempt: request.Attempt, BootID: request.BootID, TicketSHA256: ticketSHA256,
		EvidenceSHA256: evidenceSHA256,
	}
	switch stage {
	case checkpointStageRequestReceived:
		if ticketSHA256 != "" || evidenceSHA256 != "" {
			return fmt.Errorf("invalid Gate 3 checkpoint observation transition")
		}
	case checkpointStagePendingAccepted:
		if !shaPattern.MatchString(ticketSHA256) || evidenceSHA256 != "" {
			return fmt.Errorf("invalid Gate 3 checkpoint observation transition")
		}
	case checkpointStageHarnessEmitted:
		if !shaPattern.MatchString(ticketSHA256) || !shaPattern.MatchString(evidenceSHA256) {
			return fmt.Errorf("invalid Gate 3 checkpoint observation transition")
		}
	default:
		return fmt.Errorf("invalid Gate 3 checkpoint observation transition")
	}
	data, err := json.Marshal(observation)
	if err != nil {
		return err
	}
	data, err = protocol.Canonicalize(data)
	if err != nil {
		return err
	}
	line := append([]byte(checkpointObservationPrefix), data...)
	line = append(line, '\n')
	written, err := s.Observability.Write(line)
	if err != nil {
		return fmt.Errorf("write Gate 3 checkpoint observation: %w", err)
	}
	if written != len(line) {
		return io.ErrShortWrite
	}
	return nil
}
