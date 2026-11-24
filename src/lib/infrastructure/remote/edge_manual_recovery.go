package remote

import (
	"bytes"
	"context"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"time"

	contract "dockpipe/src/lib/domain/remote"
)

func (supervisor *EdgeSupervisor) validateManualRecovery(request contract.EdgeActivationRequest) error {
	approval := request.ManualRecovery
	failed := supervisor.journal
	if approval == nil || failed.Result.Status != "failed" || !failed.CutoverStarted ||
		failed.Result.ID != approval.FailedActivationID || failed.Request.ID() != approval.FailedActivationID {
		return errors.New("manual recovery requires the exact current failed cutover")
	}
	if supervisor.current != nil {
		return errors.New("manual recovery requires no supervised connector; existing processes are not stopped")
	}
	now := supervisor.now()
	if !approval.ExpiresAt.After(now) || approval.ExpiresAt.After(now.Add(5*time.Minute)) {
		return errors.New("manual recovery approval must expire within five minutes")
	}
	if prior := failed.Request.ManualRecovery; prior != nil && prior.Nonce == approval.Nonce {
		return errors.New("manual recovery requires a fresh approval nonce")
	}
	if !maps.Equal(request.Inputs, failed.Request.Inputs) || request.ReadinessURL != failed.Request.ReadinessURL {
		return errors.New("manual recovery changed approved provider inputs or readiness")
	}
	approved := false
	for _, role := range []string{"candidate", "recovery"} {
		config := failed.Request.Candidate
		digest := failed.Request.CandidateSHA256
		if role == "recovery" {
			config = failed.Request.Recovery
			digest = failed.Request.RecoverySHA256
		}
		// An original activation pins these images in its transaction directory.
		// A failed explicit attempt already names an earlier approved snapshot,
		// which remains usable even if cancellation preceded its own snapshot.
		if failed.Request.ManualRecovery == nil {
			config.Executable = supervisor.imagePath(failed.Result.ID, role)
		}
		if contract.EdgeConfigDigest(request.Candidate) == contract.EdgeConfigDigest(config) && request.CandidateSHA256 == digest {
			approved = true
		}
	}
	if !approved {
		return errors.New("manual recovery must select one exact prior approved candidate or recovery snapshot")
	}
	if err := verifyEdgeInputs(request.Inputs); err != nil {
		return err
	}
	return verifyEdgeSnapshot(request.Candidate.Executable, request.CandidateSHA256)
}

// archiveFailedJournal preserves the original byte representation, not a
// remarshaled approximation. Existing archives cannot be replaced.
func (supervisor *EdgeSupervisor) archiveFailedJournal() error {
	data, err := readPrivateData(supervisor.journalPath(), 64<<20)
	if err != nil {
		return err
	}
	var journal edgeActivationJournal
	if err := Decode(data, &journal); err != nil {
		return err
	}
	if !reflect.DeepEqual(journal, supervisor.journal) {
		return errors.New("failed edge journal changed before recovery admission")
	}
	archive := supervisor.imagePath(journal.Result.ID, "failed-journal.json")
	if prior, err := readPrivateData(archive, 64<<20); err == nil {
		if !bytes.Equal(prior, data) {
			return errors.New("failed edge journal archive differs; preserve state for inspection")
		}
		file, err := os.Open(archive)
		if err != nil {
			return err
		}
		syncErr := file.Sync()
		closeErr := file.Close()
		if syncErr != nil {
			return syncErr
		}
		if closeErr != nil {
			return closeErr
		}
		return syncEdgeArchiveDirectory(filepath.Dir(archive))
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := PrivateDirectory(filepath.Dir(archive)); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(archive), ".failed-journal-")
	if err != nil {
		return err
	}
	defer temporary.Close()
	defer os.Remove(temporary.Name())
	if _, err := temporary.Write(data); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Link(temporary.Name(), archive); err != nil {
		return err
	}
	if err := os.Remove(temporary.Name()); err != nil {
		return err
	}
	return syncEdgeArchiveDirectory(filepath.Dir(archive))
}

func syncEdgeArchiveDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func (supervisor *EdgeSupervisor) activateManualRecovery(journal *edgeActivationJournal) {
	request := journal.Request
	fail := func(message string) {
		_ = supervisor.publish(journal, "failed", message, "")
	}
	if request.ManualRecovery == nil || supervisor.current != nil {
		fail("Explicit recovery ownership changed; no connector was started or stopped.")
		return
	}
	ctx, cancel := context.WithDeadline(supervisor.ctx, journal.Result.RecoveryUntil)
	defer cancel()
	if ctx.Err() != nil || !supervisor.now().Before(request.ManualRecovery.ExpiresAt) {
		fail("Explicit recovery approval expired or was cancelled before launch.")
		return
	}
	if err := verifyEdgeInputs(request.Inputs); err != nil {
		fail("Approved provider inputs changed; no recovery connector was started.")
		return
	}
	config := request.Candidate
	config.Executable = supervisor.imagePath(request.ID(), "candidate")
	if err := snapshotEdgeBinary(request.Candidate.Executable, config.Executable, request.CandidateSHA256); err != nil {
		fail("Explicit recovery image snapshot failed verification; no connector was started.")
		return
	}
	// Persist the one launch intent before executing a process. Boot never
	// retries an accepted or activating explicit recovery after interruption.
	if err := supervisor.publish(journal, "activating", "Durable explicit recovery launch intent recorded; one approved image may start.", ""); err != nil {
		return
	}
	if ctx.Err() != nil || !supervisor.now().Before(request.ManualRecovery.ExpiresAt) || supervisor.current != nil {
		fail("Explicit recovery authority expired or ownership changed before launch.")
		return
	}
	if err := verifyEdgeInputs(request.Inputs); err != nil {
		fail("Approved provider inputs changed before launch; no connector was started.")
		return
	}
	if ctx.Err() != nil || !supervisor.now().Before(request.ManualRecovery.ExpiresAt) {
		fail("Explicit recovery authority expired during final input verification; no connector was started.")
		return
	}
	// The child belongs to broker lifetime after success. Startup/health has its
	// separate bounded context; failure below stops only this newly owned child.
	child, err := supervisor.start(supervisor.ctx, config)
	if err != nil {
		fail("Explicit recovery connector could not start; no automatic retry was attempted.")
		return
	}
	supervisor.current = child
	supervisor.readiness = request.ReadinessURL
	health, healthCancel := context.WithTimeout(ctx, time.Duration(request.StartupSeconds+request.StableSeconds)*time.Second)
	err = supervisor.waitHealthy(health, child, request.ReadinessURL, request.CandidateSHA256, request.StableSeconds, true)
	healthCancel()
	if err != nil || ctx.Err() != nil {
		child.stop()
		supervisor.current = nil
		fail("Explicit recovery failed bounded image/readiness/public TLS health validation; no further connector was started.")
		return
	}
	supervisor.mutex.Lock()
	previousBaseline := supervisor.baseline
	supervisor.mutex.Unlock()
	supervisor.selectBaseline(config, request.CandidateSHA256, request.ReadinessURL)
	// A successful deployment has normal startup authority, but this explicit
	// single attempt grants no renewed rollback/restart loop to the watchdog.
	journal.Result.RecoveryUntil = supervisor.now().UTC()
	if err := supervisor.publish(journal, "healthy", "Explicit recovery image, owned readiness and signed public TLS broker round trip verified; automatic recovery authority ended.", request.CandidateSHA256); err != nil {
		child.stop()
		supervisor.current = nil
		supervisor.mutex.Lock()
		supervisor.baseline = previousBaseline
		supervisor.mutex.Unlock()
	}
}
