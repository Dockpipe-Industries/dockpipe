//go:build linux

package remote

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	contract "dockpipe/src/lib/domain/remote"
)

type fixtureEdge struct {
	stopHook func()
	live     atomic.Bool
	image    string
	stops    atomic.Int32
}

func newFixtureEdge(image string) *fixtureEdge {
	edge := &fixtureEdge{image: image}
	edge.live.Store(true)
	return edge
}
func (edge *fixtureEdge) snapshot(string, string) error {
	return errors.New("fixture snapshot not expected")
}
func (edge *fixtureEdge) alive() bool { return edge.live.Load() }
func (edge *fixtureEdge) stop() {
	edge.live.Store(false)
	edge.stops.Add(1)
	if edge.stopHook != nil {
		edge.stopHook()
	}
}
func (edge *fixtureEdge) digest() (string, error) {
	if !edge.alive() {
		return "", errors.New("exited")
	}
	return edge.image, nil
}
func (edge *fixtureEdge) ownsReadiness(string) bool { return edge.alive() }

func edgeActivationFixture(t *testing.T) (*EdgeSupervisor, contract.EdgeActivationRequest, *fixtureEdge) {
	t.Helper()
	root := privateTestDirectory(t)
	oldPath := filepath.Join(root, "old")
	nextPath := filepath.Join(root, "new")
	for path, data := range map[string][]byte{oldPath: []byte("old approved binary"), nextPath: []byte("new approved binary")} {
		if err := os.WriteFile(path, data, 0700); err != nil {
			t.Fatal(err)
		}
	}
	oldSum := sha256.Sum256([]byte("old approved binary"))
	nextSum := sha256.Sum256([]byte("new approved binary"))
	oldDigest := hex.EncodeToString(oldSum[:])
	nextDigest := hex.EncodeToString(nextSum[:])
	base := contract.EdgeConfig{Schema: contract.Version, Endpoint: "https://edge.example.test", Executable: oldPath, Arguments: []string{"serve"}}
	request := contract.EdgeActivationRequest{Schema: contract.EdgeActivationSchema, ExpectedConfigDigest: contract.EdgeConfigDigest(base), Candidate: base, Recovery: base,
		CandidateSHA256: nextDigest, RecoverySHA256: oldDigest, ReadinessURL: "http://127.0.0.1:45000/fixture-health", StartupSeconds: 2, StableSeconds: 2, RecoverySeconds: 30, Maintenance: "allow-connector-replacement"}
	request.Candidate.Executable = nextPath
	supervisor := newEdgeSupervisor(context.Background(), root, base, func(context.Context, string) error { return nil })
	supervisor.interval = 10 * time.Millisecond
	supervisor.ready = func(context.Context, managedEdge, string) error { return nil }
	previous := newFixtureEdge(oldDigest)
	supervisor.current = previous
	supervisor.baseline = edgeBaseline{Config: base, Digest: oldDigest, BaseDigest: contract.EdgeConfigDigest(base)}
	supervisor.start = func(ctx context.Context, config contract.EdgeConfig) (managedEdge, error) {
		if filepath.Base(config.Executable) == "candidate" {
			return newFixtureEdge(nextDigest), nil
		}
		return newFixtureEdge(oldDigest), nil
	}
	if _, err := supervisor.accept(request, false); err != nil {
		t.Fatal(err)
	}
	<-supervisor.requests
	return supervisor, request, previous
}

func TestEdgeActivationKeepsBaselineUntilReadyAndVerifiesAfterCutover(t *testing.T) {
	supervisor, request, previous := edgeActivationFixture(t)
	checkedBeforeStop := false
	supervisor.ready = func(ctx context.Context, child managedEdge, address string) error {
		digest, _ := child.digest()
		if digest == request.CandidateSHA256 && previous.alive() {
			checkedBeforeStop = true
		}
		return nil
	}
	journal := supervisor.journal
	supervisor.activate(&journal)
	if journal.Result.Status != "healthy" || !checkedBeforeStop || previous.alive() || journal.Result.ActiveSHA256 != request.CandidateSHA256 {
		t.Fatal(journal.Result, checkedBeforeStop, previous.alive())
	}
	if !supervisor.maintenanceActive() {
		t.Fatal("new jobs admitted inside rollback window")
	}
	status, err := supervisor.accept(request, false)
	if err != nil || status.Status != "healthy" || len(supervisor.requests) != 0 {
		t.Fatal("exact request replayed", status, err)
	}
}

func TestEdgeReadinessFailurePreservesLiveBaseline(t *testing.T) {
	supervisor, request, previous := edgeActivationFixture(t)
	supervisor.ready = func(ctx context.Context, child managedEdge, address string) error { return errors.New("not ready") }
	journal := supervisor.journal
	supervisor.activate(&journal)
	if journal.Result.Status != "recovered" || !previous.alive() || previous.stops.Load() != 0 || journal.Result.ActiveSHA256 != request.RecoverySHA256 {
		t.Fatal(journal.Result)
	}
}

func TestEdgePublicFailureAfterCutoverRestoresApprovedImage(t *testing.T) {
	supervisor, request, previous := edgeActivationFixture(t)
	supervisor.public = func(context.Context, string) error {
		digest, _ := supervisor.current.digest()
		if digest == request.CandidateSHA256 {
			return errors.New("public path broken")
		}
		return nil
	}
	journal := supervisor.journal
	supervisor.activate(&journal)
	if journal.Result.Status != "recovered" || previous.alive() || journal.Result.ActiveSHA256 != request.RecoverySHA256 {
		t.Fatal(journal.Result)
	}
}

func TestEdgeRecoveryFailureAndExpiredAuthorityDoNotLoop(t *testing.T) {
	supervisor, request, _ := edgeActivationFixture(t)
	journal := supervisor.journal
	supervisor.activate(&journal)
	starts := 0
	supervisor.current.stop()
	supervisor.start = func(context.Context, contract.EdgeConfig) (managedEdge, error) {
		starts++
		return nil, errors.New("recovery unavailable")
	}
	supervisor.recover(&journal)
	if journal.Result.Status != "failed" || starts != 1 {
		t.Fatal(journal.Result, starts)
	}
	journal.Result.RecoveryUntil = time.Now().Add(-time.Second)
	supervisor.recover(&journal)
	if starts != 1 || journal.Result.Status != "failed" {
		t.Fatal("expired recovery restarted", request.ID())
	}
}

func TestEdgeRejectsChangedImagesAndProviderConfiguration(t *testing.T) {
	for _, changedInput := range []bool{false, true} {
		supervisor, request, previous := edgeActivationFixture(t)
		if changedInput {
			request.Inputs = map[string]string{request.Candidate.Executable: strings.Repeat("0", 64)}
		} else {
			previous.image = strings.Repeat("0", 64)
		}
		journal := supervisor.journal
		journal.Request = request
		supervisor.activate(&journal)
		if journal.Result.Status != "failed" || !previous.alive() || previous.stops.Load() != 0 {
			t.Fatal(journal.Result)
		}
	}
}

func TestEdgeInterruptedActivationBootsFallbackAndFailedJournalDoesNotRetry(t *testing.T) {
	supervisor, request, _ := edgeActivationFixture(t)
	journal := supervisor.journal
	if err := snapshotEdgeBinary(request.Recovery.Executable, supervisor.imagePath(request.ID(), "recovery"), request.RecoverySHA256); err != nil {
		t.Fatal(err)
	}
	if err := supervisor.publish(&journal, "activating", "interrupted", ""); err != nil {
		t.Fatal(err)
	}
	if err := supervisor.boot(); err != nil {
		t.Fatal(err)
	}
	if supervisor.journal.Result.Status != "recovered" {
		t.Fatal(supervisor.journal.Result)
	}
	supervisor.current.stop()
	journal.CutoverStarted = true
	if err := supervisor.publish(&journal, "failed", "failed recovery", ""); err != nil {
		t.Fatal(err)
	}
	called := false
	supervisor.start = func(context.Context, contract.EdgeConfig) (managedEdge, error) {
		called = true
		return nil, errors.New("must not run")
	}
	if err := supervisor.boot(); err != nil || called {
		t.Fatal("failed recovery replayed on startup", err)
	}
}

func TestEdgeWatchdogRecoversAfterObserverDisconnect(t *testing.T) {
	supervisor, request, _ := edgeActivationFixture(t)
	journal := supervisor.journal
	supervisor.activate(&journal)
	if journal.Result.Status != "healthy" {
		t.Fatal(journal.Result)
	}
	candidate := supervisor.current.(*fixtureEdge)
	brokerContext, cancelBroker := context.WithCancel(context.Background())
	defer cancelBroker()
	supervisor.ctx = brokerContext
	supervisor.watchInterval = 10 * time.Millisecond
	go supervisor.run()
	// A disconnected observer does not cancel the broker-owned activation context.
	observerContext, cancelObserver := context.WithCancel(context.Background())
	cancelObserver()
	if observerContext.Err() == nil {
		t.Fatal("observer fixture did not cancel")
	}
	candidate.live.Store(false)
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		status, err := supervisor.status(request.ID())
		if err == nil && status.Status == "recovered" {
			cancelBroker()
			<-supervisor.done
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancelBroker()
	<-supervisor.done
	t.Fatal("broker did not recover a dropped candidate independently of the observer")
}

func TestEdgeExpiredWatchdogDoesNotRestart(t *testing.T) {
	supervisor, request, _ := edgeActivationFixture(t)
	journal := supervisor.journal
	supervisor.activate(&journal)
	journal.Result.RecoveryUntil = time.Now().Add(-time.Second)
	if err := supervisor.publish(&journal, "healthy", "fixture expired", request.CandidateSHA256); err != nil {
		t.Fatal(err)
	}
	candidate := supervisor.current.(*fixtureEdge)
	candidate.live.Store(false)
	var starts atomic.Int32
	supervisor.start = func(context.Context, contract.EdgeConfig) (managedEdge, error) {
		starts.Add(1)
		return nil, errors.New("unexpected restart")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	supervisor.ctx = ctx
	supervisor.watchInterval = 10 * time.Millisecond
	go supervisor.run()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		status, _ := supervisor.status(request.ID())
		if status.Status == "failed" {
			cancel()
			<-supervisor.done
			if starts.Load() != 0 {
				t.Fatal("expired recovery restarted a connector")
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	<-supervisor.done
	t.Fatal("expired watchdog did not report degraded health")
}

func TestEdgePreflightFailurePreservesNormalPinnedStartup(t *testing.T) {
	supervisor, request, previous := edgeActivationFixture(t)
	journal := supervisor.journal
	// Existing installed candidate can change while the original connector keeps
	// running. Reject activation without losing the pinned normal startup image.
	if err := os.WriteFile(request.Candidate.Executable, []byte("changed"), 0700); err != nil {
		t.Fatal(err)
	}
	supervisor.activate(&journal)
	if journal.Result.Status != "failed" || !previous.alive() || journal.CutoverStarted {
		t.Fatal(journal.Result)
	}
	previous.stop()
	if err := supervisor.boot(); err != nil {
		t.Fatal(err)
	}
	if supervisor.journal.Result.Status != "recovered" || supervisor.journal.Result.ActiveSHA256 != request.RecoverySHA256 {
		t.Fatal(supervisor.journal.Result)
	}
}

func TestEdgeRecoveryDoesNotLaunchAfterStopConsumesDeadline(t *testing.T) {
	supervisor, _, _ := edgeActivationFixture(t)
	journal := supervisor.journal
	supervisor.activate(&journal)
	candidate := supervisor.current.(*fixtureEdge)
	candidate.stopHook = func() { time.Sleep(150 * time.Millisecond) }
	journal.Result.RecoveryUntil = time.Now().Add(50 * time.Millisecond)
	var starts int
	supervisor.start = func(context.Context, contract.EdgeConfig) (managedEdge, error) {
		starts++
		return nil, errors.New("must not launch")
	}
	supervisor.recover(&journal)
	if starts != 0 || journal.Result.Status != "failed" {
		t.Fatal("fallback launched after recovery deadline", starts, journal.Result)
	}
}

func TestEdgeRejectedDigestDoesNotPoisonPreservedBaselineWatchdog(t *testing.T) {
	supervisor, request, previous := edgeActivationFixture(t)
	request.RecoverySHA256 = Hash("incorrect expected baseline")
	// Accept a fresh request with an incorrect recovery expectation while preserving
	// the actual current deployment selected before that request.
	supervisor.journal.Request = request
	supervisor.journal.Result.ID = request.ID()
	journal := supervisor.journal
	supervisor.activate(&journal)
	if journal.Result.Status != "failed" || !previous.alive() {
		t.Fatal(journal.Result)
	}
	previous.stop()
	if err := supervisor.boot(); err != nil {
		t.Fatal(err)
	}
	actualBaseline := supervisor.baseline.Digest
	if supervisor.journal.Result.ActiveSHA256 != actualBaseline {
		t.Fatal(supervisor.journal.Result)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	supervisor.ctx = ctx
	supervisor.watchInterval = 10 * time.Millisecond
	go supervisor.run()
	time.Sleep(100 * time.Millisecond)
	status, _ := supervisor.status(request.ID())
	cancel()
	<-supervisor.done
	if status.Status != "recovered" {
		t.Fatal("valid preserved baseline was falsely declared failed", status)
	}
}
