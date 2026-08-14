package recovery

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

type Identity struct {
	MachineUUID        string `json:"machine_uuid"`
	DiskSerial         string `json:"disk_serial"`
	BootID             string `json:"boot_id"`
	RunID              string `json:"run_id"`
	CohortID           string `json:"cohort_id"`
	TrialID            string `json:"trial_id"`
	Scenario           string `json:"scenario"`
	DurabilityBoundary string `json:"durability_boundary"`
	Nonce              string `json:"nonce"`
	HarnessSHA256      string `json:"harness_sha256"`
}

type Ticket struct {
	Identity   Identity `json:"identity"`
	Status     string   `json:"status"`
	ResultHash string   `json:"result_hash,omitempty"`
}

type Result struct {
	Identity Identity `json:"identity"`
	Outcome  string   `json:"outcome"`
	Evidence string   `json:"evidence_sha256"`
}

type Store interface {
	Load(runID string) (Ticket, bool, error)
	Save(ticket Ticket) error
	Delete(runID string) error
}

type StateMachine struct {
	store    Store
	expected Identity
}

func New(store Store, expected Identity) (*StateMachine, error) {
	if store == nil || errIdentity(expected) != nil {
		return nil, fmt.Errorf("recovery state requires a store and complete expected identity")
	}
	return &StateMachine{store: store, expected: expected}, nil
}

func (s *StateMachine) AcceptPending(ticket Ticket) error {
	if ticket.Status != "pending" || !sameIdentity(ticket.Identity, s.expected) {
		return fmt.Errorf("ticket does not exactly match the authenticated recovery identity")
	}
	if _, exists, err := s.store.Load(ticket.Identity.TrialID); err != nil {
		return err
	} else if exists {
		return fmt.Errorf("existing pending or consumed ticket blocks the run")
	}
	return s.store.Save(ticket)
}

// ConsumeRecovery persists consumed state and the result hash before returning
// the sole signed-result payload to the caller. A lost result is never resent.
func (s *StateMachine) ConsumeRecovery(result Result) (Result, error) {
	ticket, exists, err := s.store.Load(s.expected.TrialID)
	if err != nil {
		return Result{}, err
	}
	if !exists || ticket.Status != "pending" {
		return Result{}, fmt.Errorf("matching pending ticket is required; consumed tickets never resume")
	}
	if !sameIdentity(ticket.Identity, s.expected) || !sameIdentity(result.Identity, s.expected) {
		return Result{}, fmt.Errorf("recovery identity, nonce, boundary, or harness substitution rejected")
	}
	if result.Outcome == "" || len(result.Evidence) != 64 {
		return Result{}, fmt.Errorf("recovery result is incomplete")
	}
	b, err := json.Marshal(result)
	if err != nil {
		return Result{}, err
	}
	sum := sha256.Sum256(b)
	ticket.Status = "consumed"
	ticket.ResultHash = hex.EncodeToString(sum[:])
	if err := s.store.Save(ticket); err != nil {
		return Result{}, fmt.Errorf("persist consumed recovery ticket: %w", err)
	}
	return result, nil
}

func (s *StateMachine) CleanupConsumed(trialID string, qualificationActive, explicit bool) error {
	if qualificationActive || !explicit || trialID != s.expected.TrialID {
		return fmt.Errorf("consumed ticket cleanup is explicit and outside qualification only")
	}
	ticket, exists, err := s.store.Load(trialID)
	if err != nil {
		return err
	}
	if !exists || ticket.Status != "consumed" {
		return fmt.Errorf("only consumed tickets may be cleaned")
	}
	return s.store.Delete(trialID)
}

func sameIdentity(a, b Identity) bool { return a == b }

func errIdentity(id Identity) error {
	for _, value := range []string{id.MachineUUID, id.DiskSerial, id.BootID, id.RunID, id.CohortID, id.TrialID, id.Scenario, id.DurabilityBoundary, id.Nonce, id.HarnessSHA256} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("incomplete identity")
		}
	}
	return nil
}
