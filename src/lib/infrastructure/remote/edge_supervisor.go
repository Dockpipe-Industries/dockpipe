package remote

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure/operationrecord"
)

type edgeBaseline struct {
	Config     contract.EdgeConfig `json:"config"`
	Digest     string              `json:"digest"`
	Readiness  string              `json:"readiness"`
	BaseDigest string              `json:"base_digest"`
}

type edgeActivationJournal struct {
	Baseline       edgeBaseline                   `json:"baseline"`
	CutoverStarted bool                           `json:"cutover_started"`
	Request        contract.EdgeActivationRequest `json:"request"`
	Result         contract.EdgeActivationStatus  `json:"result"`
}

// EdgeSupervisor is part of the existing broker process. It owns its children
// and one bounded activation/recovery transaction, independently of HTTP clients.
type EdgeSupervisor struct {
	baseline      edgeBaseline
	watchInterval time.Duration
	done          chan struct{}
	mutex         sync.Mutex
	root          string
	base          contract.EdgeConfig
	journal       edgeActivationJournal
	poisoned      bool
	ctx           context.Context
	requests      chan contract.EdgeActivationRequest
	failures      chan error
	current       managedEdge
	readiness     string
	start         func(context.Context, contract.EdgeConfig) (managedEdge, error)
	ready         func(context.Context, managedEdge, string) error
	public        func(context.Context, string) error
	interval      time.Duration
	now           func() time.Time
}

func newEdgeSupervisor(ctx context.Context, root string, base contract.EdgeConfig, public func(context.Context, string) error) *EdgeSupervisor {
	return &EdgeSupervisor{watchInterval: 5 * time.Second, done: make(chan struct{}), ctx: ctx, root: root, base: base, requests: make(chan contract.EdgeActivationRequest, 1), failures: make(chan error, 1),
		start: startEdgeChild, ready: checkEdgeReadiness, public: public, interval: time.Second, now: time.Now}
}

func (supervisor *EdgeSupervisor) journalPath() string {
	return filepath.Join(supervisor.root, "edge-activation.json")
}
func (supervisor *EdgeSupervisor) imagePath(id, name string) string {
	return filepath.Join(supervisor.root, "edge-activations", id, name)
}

func (supervisor *EdgeSupervisor) accept(request contract.EdgeActivationRequest, preview bool) (contract.EdgeActivationStatus, error) {
	if err := request.Validate(); err != nil {
		return contract.EdgeActivationStatus{}, err
	}
	if request.ExpectedConfigDigest != contract.EdgeConfigDigest(supervisor.base) || request.Candidate.Endpoint != supervisor.base.Endpoint {
		return contract.EdgeActivationStatus{}, errors.New("activation belongs to a different edge configuration")
	}
	supervisor.mutex.Lock()
	defer supervisor.mutex.Unlock()
	if supervisor.baseline.Digest == "" {
		return contract.EdgeActivationStatus{}, errors.New("activation requires a verified native baseline image; legacy process execution remains supported")
	}
	if supervisor.poisoned {
		return contract.EdgeActivationStatus{}, errors.New("edge journal requires inspection")
	}
	if supervisor.journal.Result.ID == request.ID() {
		return supervisor.journal.Result, nil
	}
	status := supervisor.journal.Result.Status
	if status == "accepted" || status == "activating" || status == "recovering" || (status == "healthy" && supervisor.now().Before(supervisor.journal.Result.RecoveryUntil)) {
		return contract.EdgeActivationStatus{}, errors.New("an edge activation or its recovery window is already active")
	}
	if preview {
		return contract.EdgeActivationStatus{Schema: contract.EdgeActivationSchema, ID: request.ID(), Status: "ready", Message: "Exact broker configuration accepted. Execution verifies live image and private snapshots before connector replacement; recovery authority is bounded."}, nil
	}
	journal := edgeActivationJournal{Baseline: supervisor.baseline, Request: request, Result: contract.EdgeActivationStatus{Schema: contract.EdgeActivationSchema, ID: request.ID(), Status: "accepted", Message: "Activation accepted by local broker; do not resubmit on connection loss.", CheckedAt: supervisor.now().UTC(), RecoveryUntil: supervisor.now().Add(time.Duration(request.RecoverySeconds) * time.Second).UTC()}}
	if err := WritePrivate(supervisor.journalPath(), journal); err != nil {
		return contract.EdgeActivationStatus{}, err
	}
	supervisor.journal = journal
	supervisor.requests <- request
	return journal.Result, nil
}

func (supervisor *EdgeSupervisor) status(id string) (contract.EdgeActivationStatus, error) {
	supervisor.mutex.Lock()
	defer supervisor.mutex.Unlock()
	if supervisor.journal.Result.ID != id {
		return contract.EdgeActivationStatus{}, errors.New("activation identity not found")
	}
	return supervisor.journal.Result, nil
}

func (supervisor *EdgeSupervisor) publish(journal *edgeActivationJournal, status, message, digest string) error {
	journal.Result.Status = status
	journal.Result.Message = message
	journal.Result.ActiveSHA256 = digest
	journal.Result.CheckedAt = supervisor.now().UTC()
	supervisor.mutex.Lock()
	defer supervisor.mutex.Unlock()
	if supervisor.journal.Result.ID != "" && supervisor.journal.Result.ID != journal.Result.ID {
		return errors.New("edge transaction superseded before publication")
	}
	if err := WritePrivate(supervisor.journalPath(), journal); err != nil {
		supervisor.poisoned = true
		supervisor.journal.Result.Status = "failed"
		supervisor.journal.Result.Message = "Durable edge journal failed; preserve state and inspect locally."
		return err
	}
	supervisor.journal = *journal
	operationrecord.LogOperationResult(os.Stderr, operationrecord.OperationResult{Unit: "remote.edge.activation", Status: operationrecord.OperationStatusProgress, Message: status, IDs: map[string]string{"activation": journal.Result.ID}})
	return nil
}

func (supervisor *EdgeSupervisor) boot() error {
	config := supervisor.base
	var journal edgeActivationJournal
	journalErr := ReadPrivate(supervisor.journalPath(), &journal)
	if journalErr != nil && !os.IsNotExist(journalErr) {
		return journalErr
	}
	selectedPath := filepath.Join(supervisor.root, "edge-selected.json")
	selectedErr := ReadPrivate(selectedPath, &supervisor.baseline)
	if selectedErr != nil && !os.IsNotExist(selectedErr) {
		return selectedErr
	}
	recoveryBudget := false
	expected := ""
	resumedStatus := ""
	if journalErr == nil {
		if err := journal.Request.Validate(); err != nil {
			return err
		}
		if journal.Result.ID != journal.Request.ID() || journal.Request.ExpectedConfigDigest != contract.EdgeConfigDigest(supervisor.base) {
			return errors.New("edge journal does not match configured broker")
		}
		supervisor.journal = journal
		switch journal.Result.Status {
		case "accepted", "activating", "healthy", "recovering", "recovered", "failed":
		default:
			return errors.New("unknown edge journal status")
		}
		if journal.Result.Status == "healthy" {
			config = journal.Request.Candidate
			config.Executable = supervisor.imagePath(journal.Result.ID, "candidate")
			expected = journal.Request.CandidateSHA256
			supervisor.readiness = journal.Request.ReadinessURL
			resumedStatus = "healthy"
		} else if !journal.CutoverStarted {
			// Failed/preflight transactions never revoke the prior deployment's normal
			// service startup authority. This is not replaying activation or rollback.
			supervisor.baseline = journal.Baseline
			config = journal.Baseline.Config
			expected = journal.Baseline.Digest
			supervisor.readiness = journal.Baseline.Readiness
			resumedStatus = "recovered"
		} else {
			if journal.Result.Status == "failed" {
				return nil
			}
			if !supervisor.now().Before(journal.Result.RecoveryUntil) && journal.Result.Status != "recovered" {
				return supervisor.publish(&journal, "failed", "Interrupted activation recovery authority expired; no connector was started.", "")
			}
			recoveryBudget = journal.Result.Status != "recovered"
			config = journal.Request.Recovery
			config.Executable = supervisor.imagePath(journal.Result.ID, "recovery")
			expected = journal.Request.RecoverySHA256
			supervisor.readiness = journal.Request.ReadinessURL
			resumedStatus = "recovered"
		}
		if expected == "" || !contract.ValidEdgeDigest(expected) {
			return errors.New("missing pinned baseline identity")
		}
		if err := verifyEdgeInputs(journal.Request.Inputs); err != nil && (journal.CutoverStarted || journal.Result.Status == "healthy") {
			return supervisor.publish(&journal, "failed", "Provider configuration changed; no connector was started.", "")
		}
	} else if selectedErr == nil {
		config = supervisor.baseline.Config
		expected = supervisor.baseline.Digest
		supervisor.readiness = supervisor.baseline.Readiness
	}
	if expected != "" {
		if supervisor.baseline.BaseDigest != "" && supervisor.baseline.BaseDigest != contract.EdgeConfigDigest(supervisor.base) {
			return errors.New("pinned baseline belongs to another edge configuration")
		}
		if err := verifyEdgeSnapshot(config.Executable, expected); err != nil {
			return err
		}
	}
	if recoveryBudget && (!supervisor.now().Before(journal.Result.RecoveryUntil) || supervisor.ctx.Err() != nil) {
		return supervisor.publish(&journal, "failed", "Recovery budget expired before connector launch.", "")
	}
	declaredDigest, declaredErr := edgeDeclaredDigest(config.Executable)
	child, err := supervisor.start(supervisor.ctx, config)
	if err != nil {
		return err
	}
	supervisor.current = child
	if journalErr != nil {
		if runtime.GOOS == "linux" && expected == "" {
			digest, err := child.digest()
			if err != nil || declaredErr != nil || digest != declaredDigest {
				// Keep legacy script/non-inspectable edges runnable. Activation
				// is unavailable until the provider declares a native process.
				return nil
			}
			snapshot := filepath.Join(supervisor.root, "edge-baselines", digest)
			if err := child.snapshot(snapshot, digest); err != nil {
				child.stop()
				return err
			}
			pinned := config
			pinned.Executable = snapshot
			supervisor.baseline = edgeBaseline{Config: pinned, Digest: digest, Readiness: supervisor.readiness, BaseDigest: contract.EdgeConfigDigest(supervisor.base)}
			if err := WritePrivate(selectedPath, supervisor.baseline); err != nil {
				child.stop()
				return err
			}
		}
		return nil
	}
	ctx, cancel := context.WithTimeout(supervisor.ctx, time.Duration(journal.Request.StartupSeconds+journal.Request.StableSeconds)*time.Second)
	defer cancel()
	if recoveryBudget {
		var deadlineCancel context.CancelFunc
		ctx, deadlineCancel = context.WithDeadline(ctx, journal.Result.RecoveryUntil)
		defer deadlineCancel()
	}
	if err := supervisor.waitHealthy(ctx, child, supervisor.readiness, expected, journal.Request.StableSeconds, true); err != nil {
		if resumedStatus == "healthy" && supervisor.now().Before(journal.Result.RecoveryUntil) {
			supervisor.recover(&journal)
			return nil
		}
		child.stop()
		supervisor.current = nil
		journal.CutoverStarted = true // A failed recovery must never retry on restart.
		return supervisor.publish(&journal, "failed", "Pinned connector startup failed health validation; local inspection is required.", "")
	}
	supervisor.selectBaseline(config, expected, supervisor.readiness)
	return supervisor.publish(&journal, resumedStatus, "Pinned baseline/connector startup verified active image and signed public endpoint; interrupted activation was not replayed.", expected)
}

func (supervisor *EdgeSupervisor) run() {
	defer close(supervisor.done)
	defer func() {
		if supervisor.current != nil {
			supervisor.current.stop()
		}
	}()
	ticker := time.NewTicker(supervisor.watchInterval)
	defer ticker.Stop()
	failures := 0
	for {
		select {
		case <-supervisor.ctx.Done():
			return
		case request := <-supervisor.requests:
			supervisor.mutex.Lock()
			journal := supervisor.journal
			supervisor.mutex.Unlock()
			if journal.Result.ID != request.ID() {
				continue
			}
			supervisor.activate(&journal)
			failures = 0
		case <-ticker.C:
			supervisor.mutex.Lock()
			journal := supervisor.journal
			supervisor.mutex.Unlock()
			if journal.Result.ID == "" {
				if supervisor.current == nil || !supervisor.current.alive() {
					supervisor.failures <- errors.New("remote edge process exited")
					return
				}
				continue
			}
			if journal.Result.Status != "healthy" && journal.Result.Status != "recovered" {
				continue
			}
			expected := journal.Request.CandidateSHA256
			if journal.Result.Status == "recovered" {
				expected = journal.Result.ActiveSHA256
			}
			if err := supervisor.check(supervisor.ctx, supervisor.current, supervisor.readiness, expected, true); err != nil {
				failures++
			} else {
				failures = 0
				supervisor.mutex.Lock()
				supervisor.journal.Result.CheckedAt = supervisor.now().UTC()
				supervisor.mutex.Unlock()
			}
			if failures < 2 {
				continue
			}
			if journal.Result.Status == "healthy" && supervisor.now().Before(journal.Result.RecoveryUntil) {
				supervisor.recover(&journal)
			} else {
				_ = supervisor.publish(&journal, "failed", "Connector health was lost outside available recovery authority; automatic restart/rollback stopped. Inspect locally.", "")
			}
			failures = 0
		}
	}
}

func (supervisor *EdgeSupervisor) activate(journal *edgeActivationJournal) {
	request := journal.Request
	if err := verifyEdgeInputs(request.Inputs); err != nil {
		_ = supervisor.publish(journal, "failed", "Provider configuration changed before activation; baseline preserved.", "")
		return
	}
	if supervisor.current == nil {
		_ = supervisor.publish(journal, "failed", "No supervised baseline process exists; activation did not start.", "")
		return
	}
	digest, err := supervisor.current.digest()
	if err != nil || digest != request.RecoverySHA256 {
		_ = supervisor.publish(journal, "failed", "Running baseline image does not match the approved recovery digest; no connector was stopped.", "")
		return
	}
	if err := supervisor.public(supervisor.ctx, supervisor.base.Endpoint); err != nil {
		_ = supervisor.publish(journal, "failed", "Baseline public broker identity/TLS probe failed; no connector was stopped.", digest)
		return
	}
	candidateConfig := request.Candidate
	candidateConfig.Executable = supervisor.imagePath(request.ID(), "candidate")
	recoveryPath := supervisor.imagePath(request.ID(), "recovery")
	if err := snapshotEdgeBinary(request.Candidate.Executable, candidateConfig.Executable, request.CandidateSHA256); err != nil {
		_ = supervisor.publish(journal, "failed", "Candidate snapshot failed integrity checks; baseline preserved.", digest)
		return
	}
	if err := snapshotEdgeBinary(request.Recovery.Executable, recoveryPath, request.RecoverySHA256); err != nil {
		_ = supervisor.publish(journal, "failed", "Recovery snapshot failed integrity checks; baseline preserved.", digest)
		return
	}
	if err := supervisor.publish(journal, "activating", "Starting candidate alongside baseline; no old connector has been stopped.", digest); err != nil {
		return
	}
	if supervisor.ctx.Err() != nil || !supervisor.now().Before(journal.Result.RecoveryUntil) {
		_ = supervisor.publish(journal, "failed", "Activation budget expired before candidate launch; baseline preserved.", digest)
		return
	}
	candidate, err := supervisor.start(supervisor.ctx, candidateConfig)
	if err != nil {
		supervisor.recover(journal)
		return
	}
	ctx, cancel := context.WithTimeout(supervisor.ctx, time.Duration(request.StartupSeconds)*time.Second)
	err = supervisor.waitHealthy(ctx, candidate, request.ReadinessURL, request.CandidateSHA256, 0, false)
	cancel()
	if err != nil {
		candidate.stop()
		supervisor.recover(journal)
		return
	}
	if err := supervisor.publish(journal, "activating", "Candidate readiness and active image verified; entering approved connector replacement.", digest); err != nil {
		candidate.stop()
		return
	}
	if err := verifyEdgeInputs(request.Inputs); err != nil {
		candidate.stop()
		supervisor.recover(journal)
		return
	}
	if supervisor.ctx.Err() != nil || !supervisor.now().Before(journal.Result.RecoveryUntil) {
		candidate.stop()
		_ = supervisor.publish(journal, "failed", "Activation budget expired before cutover; baseline preserved.", digest)
		return
	}
	journal.CutoverStarted = true
	if err := supervisor.publish(journal, "activating", "Durable cutover intent recorded before old connector shutdown.", digest); err != nil {
		candidate.stop()
		return
	}
	previous := supervisor.current
	previous.stop()
	if previous.alive() {
		candidate.stop()
		_ = supervisor.publish(journal, "failed", "Baseline process did not stop; candidate was stopped to avoid a false cutover result.", "")
		return
	}
	supervisor.current = candidate
	supervisor.readiness = request.ReadinessURL
	ctx, cancel = context.WithTimeout(supervisor.ctx, time.Duration(request.StartupSeconds+request.StableSeconds)*time.Second)
	err = supervisor.waitHealthy(ctx, candidate, request.ReadinessURL, request.CandidateSHA256, request.StableSeconds, true)
	cancel()
	if err != nil {
		supervisor.recover(journal)
		return
	}
	supervisor.selectBaseline(candidateConfig, request.CandidateSHA256, request.ReadinessURL)
	_ = supervisor.publish(journal, "healthy", "Candidate active image, owned readiness listener and signed HTTPS broker round trip verified after baseline stopped. Recovery monitoring remains bounded by recovery_until.", request.CandidateSHA256)
}

func (supervisor *EdgeSupervisor) recover(journal *edgeActivationJournal) {
	if !supervisor.now().Before(journal.Result.RecoveryUntil) {
		_ = supervisor.publish(journal, "failed", "Recovery authority expired; no additional connector mutation was attempted.", "")
		return
	}
	if err := supervisor.publish(journal, "recovering", "Candidate health failed; restoring the exact preapproved baseline once.", ""); err != nil {
		return
	}
	request := journal.Request
	ctx, cancel := context.WithDeadline(supervisor.ctx, journal.Result.RecoveryUntil)
	defer cancel()
	ctx, limit := context.WithTimeout(ctx, time.Duration(request.StartupSeconds+request.StableSeconds+4)*time.Second)
	defer limit()
	// A failed pre-cutover candidate never needs to interrupt the still-live baseline.
	if supervisor.current != nil && supervisor.current.alive() {
		digest, err := supervisor.current.digest()
		if err == nil && digest == request.RecoverySHA256 {
			if err := supervisor.check(ctx, supervisor.current, supervisor.readiness, digest, true); err == nil {
				_ = supervisor.publish(journal, "recovered", "Original running connector was preserved and its public broker round trip verified; activation failed.", digest)
				return
			}
		}
	}
	if err := verifyEdgeInputs(request.Inputs); err != nil {
		_ = supervisor.publish(journal, "failed", "Provider configuration changed; no recovery connector was launched.", "")
		return
	}
	config := request.Recovery
	config.Executable = supervisor.imagePath(request.ID(), "recovery")
	if err := verifyEdgeSnapshot(config.Executable, request.RecoverySHA256); err != nil {
		_ = supervisor.publish(journal, "failed", "Recovery snapshot failed verification; no unrelated binary was executed.", "")
		return
	}
	if ctx.Err() != nil || !supervisor.now().Before(journal.Result.RecoveryUntil) {
		_ = supervisor.publish(journal, "failed", "Recovery budget expired before stopping a connector.", "")
		return
	}
	priorMutation := journal.CutoverStarted
	journal.CutoverStarted = true
	if err := supervisor.publish(journal, "recovering", "Durable recovery replacement intent recorded before process mutation.", ""); err != nil {
		return
	}
	if ctx.Err() != nil || !supervisor.now().Before(journal.Result.RecoveryUntil) {
		journal.CutoverStarted = priorMutation
		_ = supervisor.publish(journal, "failed", "Recovery budget expired before connector replacement.", "")
		return
	}
	if supervisor.current != nil {
		supervisor.current.stop()
	}
	if ctx.Err() != nil || !supervisor.now().Before(journal.Result.RecoveryUntil) {
		_ = supervisor.publish(journal, "failed", "Recovery budget expired before replacement launch.", "")
		return
	}
	child, err := supervisor.start(supervisor.ctx, config)
	if err != nil {
		_ = supervisor.publish(journal, "failed", "Preapproved recovery connector could not start; no retry loop was entered.", "")
		return
	}
	supervisor.current = child
	supervisor.readiness = request.ReadinessURL
	if err := supervisor.waitHealthy(ctx, child, supervisor.readiness, request.RecoverySHA256, request.StableSeconds, true); err != nil {
		child.stop()
		supervisor.current = nil
		_ = supervisor.publish(journal, "failed", "Recovery connector failed health validation; automatic attempts stopped and local broker remains available.", "")
		return
	}
	supervisor.selectBaseline(config, request.RecoverySHA256, request.ReadinessURL)
	_ = supervisor.publish(journal, "recovered", "Preapproved recovery image, readiness and signed public broker round trip verified. Activation failed; investigate before approving another attempt.", request.RecoverySHA256)
}

func (supervisor *EdgeSupervisor) check(ctx context.Context, child managedEdge, address, expected string, public bool) error {
	if child == nil {
		return errors.New("no active connector")
	}
	digest, err := child.digest()
	if err != nil || digest != expected {
		return errors.New("active executable digest mismatch")
	}
	if address != "" {
		if err := supervisor.ready(ctx, child, address); err != nil {
			return err
		}
	}
	if public {
		return supervisor.public(ctx, supervisor.base.Endpoint)
	}
	return nil
}

func (supervisor *EdgeSupervisor) waitHealthy(ctx context.Context, child managedEdge, address, expected string, stableSeconds int, public bool) error {
	var stableSince time.Time
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := supervisor.check(ctx, child, address, expected, public); err == nil {
			if stableSince.IsZero() {
				stableSince = supervisor.now()
			}
			if supervisor.now().Sub(stableSince) >= time.Duration(stableSeconds)*time.Second {
				return nil
			}
		} else {
			stableSince = time.Time{}
		}
		timer := time.NewTimer(supervisor.interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func (broker *Broker) StartEdgeSupervisor(ctx context.Context, root string, edge contract.EdgeConfig) (func(), <-chan error, error) {
	edgeContext, cancel := context.WithCancel(ctx)
	publicKey := broker.state.Identity.PublicKey
	supervisor := newEdgeSupervisor(edgeContext, root, edge, func(ctx context.Context, endpoint string) error {
		return probeEdgePublic(ctx, endpoint, publicKey, edgeHTTPClient())
	})
	if err := supervisor.boot(); err != nil {
		cancel()
		if supervisor.current != nil {
			supervisor.current.stop()
		}
		return nil, nil, fmt.Errorf("edge recovery startup failed: %w", err)
	}
	broker.mutex.Lock()
	broker.edge = supervisor
	broker.mutex.Unlock()
	go supervisor.run()
	return func() {
		cancel()
		select {
		case <-supervisor.done:
		case <-time.After(10 * time.Second):
		}
	}, supervisor.failures, nil
}

func (supervisor *EdgeSupervisor) maintenanceActive() bool {
	supervisor.mutex.Lock()
	defer supervisor.mutex.Unlock()
	status := supervisor.journal.Result.Status
	return status == "accepted" || status == "activating" || status == "recovering" || (status == "healthy" && supervisor.now().Before(supervisor.journal.Result.RecoveryUntil))
}

func verifyEdgeInputs(inputs map[string]string) error {
	for path, expected := range inputs {
		if err := verifyEdgeSnapshot(path, expected); err != nil {
			return err
		}
	}
	return nil
}

func (supervisor *EdgeSupervisor) selectBaseline(config contract.EdgeConfig, digest, readiness string) {
	supervisor.mutex.Lock()
	defer supervisor.mutex.Unlock()
	supervisor.baseline = edgeBaseline{Config: config, Digest: digest, Readiness: readiness, BaseDigest: contract.EdgeConfigDigest(supervisor.base)}
}
