package recovery

import (
	"errors"
	"strings"
	"testing"
)

type fakeStore struct {
	ticket   Ticket
	exists   bool
	failSave bool
}

func (f *fakeStore) Load(string) (Ticket, bool, error) { return f.ticket, f.exists, nil }
func (f *fakeStore) Save(ticket Ticket) error {
	if f.failSave {
		return errors.New("injected durable-write failure")
	}
	f.ticket, f.exists = ticket, true
	return nil
}
func (f *fakeStore) Delete(string) error { f.exists = false; return nil }

func identity() Identity {
	return Identity{MachineUUID: "machine", DiskSerial: "disk", BootID: "boot", RunID: "run-001", CohortID: "cohort-001", TrialID: "trial-001", Scenario: "sqlite", DurabilityBoundary: "after-fsync", Nonce: strings.Repeat("a", 64), HarnessSHA256: strings.Repeat("b", 64)}
}

func TestPendingTicketBlocksAndRecoveryIsConsumedBeforeResult(t *testing.T) {
	store := &fakeStore{}
	sm, err := New(store, identity())
	if err != nil {
		t.Fatal(err)
	}
	ticket := Ticket{Identity: identity(), Status: "pending"}
	if err := sm.AcceptPending(ticket); err != nil {
		t.Fatal(err)
	}
	if err := sm.AcceptPending(ticket); err == nil {
		t.Fatal("expected pending ticket to block new work")
	}
	result := Result{Identity: identity(), Outcome: "recovered", Evidence: strings.Repeat("c", 64)}
	if _, err := sm.ConsumeRecovery(result); err != nil {
		t.Fatal(err)
	}
	if store.ticket.Status != "consumed" || len(store.ticket.ResultHash) != 64 {
		t.Fatalf("result returned before durable consumed state: %+v", store.ticket)
	}
	if _, err := sm.ConsumeRecovery(result); err == nil {
		t.Fatal("expected lost-result no-resend rule")
	}
	if err := sm.CleanupConsumed(identity().TrialID, true, true); err == nil {
		t.Fatal("expected cleanup rejection during qualification")
	}
	if err := sm.CleanupConsumed(identity().TrialID, false, true); err != nil {
		t.Fatal(err)
	}
}

func TestRecoveryRejectsSubstitutionAndDurableWriteFailure(t *testing.T) {
	store := &fakeStore{}
	sm, _ := New(store, identity())
	ticket := Ticket{Identity: identity(), Status: "pending"}
	if err := sm.AcceptPending(ticket); err != nil {
		t.Fatal(err)
	}
	bad := Result{Identity: identity(), Outcome: "recovered", Evidence: strings.Repeat("c", 64)}
	bad.Identity.Nonce = "substituted"
	if _, err := sm.ConsumeRecovery(bad); err == nil {
		t.Fatal("expected nonce substitution rejection")
	}
	store.failSave = true
	good := Result{Identity: identity(), Outcome: "recovered", Evidence: strings.Repeat("c", 64)}
	if _, err := sm.ConsumeRecovery(good); err == nil {
		t.Fatal("expected fake filesystem durable-write failure")
	}
}

func TestFileStoreUsesOwnerOnlyTicketFiles(t *testing.T) {
	store := FileStore{Root: t.TempDir()}
	ticket := Ticket{Identity: identity(), Status: "pending"}
	if err := store.Save(ticket); err != nil {
		t.Fatal(err)
	}
	loaded, exists, err := store.Load(identity().TrialID)
	if err != nil || !exists || loaded.Status != "pending" {
		t.Fatalf("file store round trip: %+v %v %v", loaded, exists, err)
	}
}
