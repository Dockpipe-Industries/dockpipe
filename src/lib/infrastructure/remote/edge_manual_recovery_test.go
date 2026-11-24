//go:build linux

package remote

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	contract "dockpipe/src/lib/domain/remote"
)

func manualRecoveryFixture(t *testing.T, role string) (*EdgeSupervisor, contract.EdgeActivationRequest, *fixtureEdge, []byte) {
	t.Helper()
	supervisor, original, previous := edgeActivationFixture(t)
	for role, config := range map[string]contract.EdgeConfig{"candidate": original.Candidate, "recovery": original.Recovery} {
		digest := original.CandidateSHA256
		if role == "recovery" {
			digest = original.RecoverySHA256
		}
		if err := snapshotEdgeBinary(config.Executable, supervisor.imagePath(original.ID(), role), digest); err != nil {
			t.Fatal(err)
		}
	}
	failed := supervisor.journal
	failed.CutoverStarted = true
	failed.Result.Status = "failed"
	failed.Result.RecoveryUntil = time.Now().Add(-time.Hour).UTC()
	supervisor.current = nil
	supervisor.journal = failed
	data, err := json.MarshalIndent(failed, "", "    ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(append([]byte("\n"), data...), '\n')
	if err := os.WriteFile(supervisor.journalPath(), data, 0600); err != nil {
		t.Fatal(err)
	}
	request := original
	request.Candidate = original.Candidate
	request.CandidateSHA256 = original.CandidateSHA256
	if role == "recovery" {
		request.Candidate = original.Recovery
		request.CandidateSHA256 = original.RecoverySHA256
	}
	request.Candidate.Executable = supervisor.imagePath(original.ID(), role)
	request.Recovery = request.Candidate
	request.RecoverySHA256 = request.CandidateSHA256
	request.ManualRecovery = &contract.EdgeManualRecovery{FailedActivationID: original.ID(), Nonce: Hash("fresh exact approval"), ExpiresAt: time.Now().Add(2 * time.Minute).UTC()}
	return supervisor, request, previous, data
}

func TestEdgeManualRecoveryOneExactApprovedImageAndImmutableFailedArchive(t *testing.T) {
	for _, role := range []string{"candidate", "recovery"} {
		t.Run(role, func(t *testing.T) {
			supervisor, request, previous, originalBytes := manualRecoveryFixture(t, role)
			preview, err := supervisor.accept(request, true)
			if err != nil || preview.Status != "ready" || len(supervisor.requests) != 0 {
				t.Fatal(preview, err)
			}
			if data, _ := os.ReadFile(supervisor.journalPath()); !bytes.Equal(data, originalBytes) {
				t.Fatal("preview changed failed journal")
			}
			if _, err := supervisor.accept(request, false); err != nil {
				t.Fatal(err)
			}
			<-supervisor.requests
			archive, err := os.ReadFile(supervisor.imagePath(request.ManualRecovery.FailedActivationID, "failed-journal.json"))
			if err != nil || !bytes.Equal(archive, originalBytes) {
				t.Fatal("failed journal byte identity was lost", err)
			}
			starts := 0
			publicChecks := 0
			clock := time.Now()
			supervisor.now = func() time.Time {
				return clock
			}
			supervisor.ready = func(context.Context, managedEdge, string) error {
				clock = clock.Add(time.Second)
				return nil
			}
			supervisor.public = func(context.Context, string) error {
				publicChecks++
				return nil
			}
			supervisor.start = func(ctx context.Context, config contract.EdgeConfig) (managedEdge, error) {
				starts++
				var intent edgeActivationJournal
				if err := ReadPrivate(supervisor.journalPath(), &intent); err != nil || intent.Result.Status != "activating" || !intent.CutoverStarted || intent.Result.ID != request.ID() {
					t.Fatal("launch preceded durable exact intent", intent.Result, err)
				}
				if config.Executable != supervisor.imagePath(request.ID(), "candidate") {
					t.Fatal("launch escaped private image snapshot")
				}
				return newFixtureEdge(request.CandidateSHA256), nil
			}
			journal := supervisor.journal
			supervisor.activateManualRecovery(&journal)
			if journal.Result.Status != "healthy" || starts != 1 || publicChecks == 0 || previous.stops.Load() != 0 || supervisor.now().Before(journal.Result.RecoveryUntil) {
				t.Fatal(journal.Result, starts, publicChecks, previous.stops.Load())
			}
			if status, err := supervisor.accept(request, false); err != nil || status.Status != "healthy" || len(supervisor.requests) != 0 {
				t.Fatal("exact approval replayed a start", status, err)
			}
			supervisor.current.stop()
			supervisor.current = nil
			supervisor.start = func(context.Context, contract.EdgeConfig) (managedEdge, error) {
				starts++
				return newFixtureEdge(request.CandidateSHA256), nil
			}
			if err := supervisor.boot(); err != nil || supervisor.journal.Result.Status != "healthy" || starts != 2 {
				t.Fatal("healthy manual deployment lost normal pinned startup", err, starts, supervisor.journal.Result)
			}
			rollback := clock.Add(-time.Hour)
			supervisor.now = func() time.Time {
				return rollback
			}
			if supervisor.maintenanceActive() {
				t.Fatal("clock rollback renewed manual maintenance/recovery window")
			}
			journal = supervisor.journal
			supervisor.recover(&journal)
			if starts != 2 || journal.Result.Status != "failed" || !supervisor.current.alive() {
				t.Fatal("clock rollback created automatic manual recovery authority", starts, journal.Result)
			}
			supervisor.current.stop()
		})
	}
}

func TestEdgeManualRecoveryRejectsStaleOrDifferentAuthorityWithoutJournalMutation(t *testing.T) {
	cases := map[string]func(*EdgeSupervisor, *contract.EdgeActivationRequest){
		"expired": func(_ *EdgeSupervisor, request *contract.EdgeActivationRequest) {
			request.ManualRecovery.ExpiresAt = time.Now().Add(-time.Second)
		},
		"unbounded expiry": func(_ *EdgeSupervisor, request *contract.EdgeActivationRequest) {
			request.ManualRecovery.ExpiresAt = time.Now().Add(6 * time.Minute)
		},
		"different failed activation": func(_ *EdgeSupervisor, request *contract.EdgeActivationRequest) {
			request.ManualRecovery.FailedActivationID = Hash("other")
		},
		"owned connector": func(supervisor *EdgeSupervisor, request *contract.EdgeActivationRequest) {
			supervisor.current = newFixtureEdge(request.CandidateSHA256)
		},
		"unapproved image": func(_ *EdgeSupervisor, request *contract.EdgeActivationRequest) {
			request.CandidateSHA256 = Hash("unapproved")
			request.RecoverySHA256 = request.CandidateSHA256
		},
		"changed arguments": func(_ *EdgeSupervisor, request *contract.EdgeActivationRequest) {
			request.Candidate.Arguments = []string{"different"}
			request.Recovery = request.Candidate
		},
		"different readiness": func(_ *EdgeSupervisor, request *contract.EdgeActivationRequest) {
			request.ReadinessURL = "http://127.0.0.1:45001/ready"
		},
		"changed inputs": func(_ *EdgeSupervisor, request *contract.EdgeActivationRequest) {
			request.Inputs = map[string]string{request.Candidate.Executable: request.CandidateSHA256}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			supervisor, request, _, originalBytes := manualRecoveryFixture(t, "candidate")
			mutate(supervisor, &request)
			if _, err := supervisor.accept(request, false); err == nil {
				t.Fatal("different or stale recovery authority accepted")
			}
			if data, _ := os.ReadFile(supervisor.journalPath()); !bytes.Equal(data, originalBytes) || len(supervisor.requests) != 0 {
				t.Fatal("rejected recovery changed failed journal or queued work")
			}
		})
	}
}

func TestEdgeManualRecoveryCancelledOrExpiredNeverStartsOrBootReplays(t *testing.T) {
	for _, mode := range []string{"cancelled", "expired", "interrupted"} {
		t.Run(mode, func(t *testing.T) {
			supervisor, request, previous, _ := manualRecoveryFixture(t, "candidate")
			if _, err := supervisor.accept(request, false); err != nil {
				t.Fatal(err)
			}
			<-supervisor.requests
			starts := 0
			supervisor.start = func(context.Context, contract.EdgeConfig) (managedEdge, error) {
				starts++
				return nil, errors.New("unexpected launch")
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			supervisor.ctx = ctx
			journal := supervisor.journal
			if mode == "cancelled" {
				cancel()
				supervisor.activateManualRecovery(&journal)
			} else if mode == "expired" {
				supervisor.now = func() time.Time {
					return request.ManualRecovery.ExpiresAt.Add(time.Second)
				}
				supervisor.activateManualRecovery(&journal)
			}
			if err := supervisor.boot(); err != nil || starts != 0 || supervisor.journal.Result.Status != "failed" || previous.stops.Load() != 0 {
				t.Fatal("manual approval restarted on boot", err, starts, supervisor.journal.Result)
			}
		})
	}
}

func TestEdgeManualRecoveryFailedArchiveCannotBeReplaced(t *testing.T) {
	supervisor, request, _, originalBytes := manualRecoveryFixture(t, "candidate")
	archive := supervisor.imagePath(request.ManualRecovery.FailedActivationID, "failed-journal.json")
	if err := os.WriteFile(archive, []byte("different preserved evidence"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := supervisor.accept(request, false); err == nil {
		t.Fatal("existing archive was replaced")
	}
	if data, _ := os.ReadFile(supervisor.journalPath()); !bytes.Equal(data, originalBytes) {
		t.Fatal("archive failure replaced original failed journal")
	}
}

func TestEdgeManualRecoveryHealthFailureStopsOnlyNewChildAndDoesNotRetry(t *testing.T) {
	supervisor, request, previous, _ := manualRecoveryFixture(t, "candidate")
	if _, err := supervisor.accept(request, false); err != nil {
		t.Fatal(err)
	}
	<-supervisor.requests
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	supervisor.ctx = ctx
	child := newFixtureEdge(request.CandidateSHA256)
	starts := 0
	supervisor.start = func(context.Context, contract.EdgeConfig) (managedEdge, error) {
		starts++
		return child, nil
	}
	supervisor.ready = func(context.Context, managedEdge, string) error {
		cancel()
		return errors.New("health unavailable")
	}
	journal := supervisor.journal
	supervisor.activateManualRecovery(&journal)
	if journal.Result.Status != "failed" || starts != 1 || child.stops.Load() != 1 || previous.stops.Load() != 0 || supervisor.current != nil {
		t.Fatal(journal.Result, starts, child.stops.Load(), previous.stops.Load())
	}
	supervisor.ctx = context.Background()
	if err := supervisor.boot(); err != nil || starts != 1 {
		t.Fatal("failed manual attempt retried", err, starts)
	}
}

func TestEdgeManualRecoveryHealthyPublicationFailureStopsNewChild(t *testing.T) {
	supervisor, request, previous, _ := manualRecoveryFixture(t, "candidate")
	if err := supervisor.archiveFailedJournal(); err != nil {
		t.Fatal(err)
	}
	// The existing-byte branch must establish durability again, rather than
	// treating an earlier visible archive as proof of a completed sync.
	if err := supervisor.archiveFailedJournal(); err != nil {
		t.Fatal(err)
	}
	if _, err := supervisor.accept(request, false); err != nil {
		t.Fatal(err)
	}
	<-supervisor.requests
	originalBaseline := supervisor.baseline
	child := newFixtureEdge(request.CandidateSHA256)
	supervisor.start = func(context.Context, contract.EdgeConfig) (managedEdge, error) {
		return child, nil
	}
	clock := time.Now()
	supervisor.now = func() time.Time {
		return clock
	}
	checks := 0
	supervisor.ready = func(context.Context, managedEdge, string) error {
		checks++
		clock = clock.Add(time.Second)
		if checks == 3 {
			if err := os.Rename(supervisor.journalPath(), filepath.Join(supervisor.root, "intent-before-write-failure.json")); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(supervisor.journalPath(), 0700); err != nil {
				t.Fatal(err)
			}
		}
		return nil
	}
	journal := supervisor.journal
	supervisor.activateManualRecovery(&journal)
	if !supervisor.poisoned || supervisor.current != nil || child.stops.Load() != 1 || previous.stops.Load() != 0 {
		t.Fatal("uncertain healthy publication retained a new connector", supervisor.poisoned, child.stops.Load(), previous.stops.Load())
	}
	if contract.EdgeConfigDigest(supervisor.baseline.Config) != contract.EdgeConfigDigest(originalBaseline.Config) {
		t.Fatal("failed healthy publication replaced the prior selected baseline")
	}
}

func TestEdgeManualRecoveryConcurrentPreviewRespectsActorPhases(t *testing.T) {
	for _, healthy := range []bool{false, true} {
		name := "failure"
		if healthy {
			name = "healthy"
		}
		t.Run(name, func(t *testing.T) {
			supervisor, request, _, _ := manualRecoveryFixture(t, "candidate")
			ctx, cancel := context.WithCancel(context.Background())
			supervisor.ctx = ctx
			supervisor.watchInterval = time.Hour
			var starts atomic.Int32
			supervisor.start = func(context.Context, contract.EdgeConfig) (managedEdge, error) {
				starts.Add(1)
				return newFixtureEdge(request.CandidateSHA256), nil
			}
			supervisor.ready = func(context.Context, managedEdge, string) error {
				if !healthy {
					return errors.New("fixture public startup unavailable")
				}
				return nil
			}
			if _, err := supervisor.accept(request, false); err != nil {
				cancel()
				t.Fatal(err)
			}
			go supervisor.run()
			defer func() {
				cancel()
				<-supervisor.done
			}()
			completed := false
			for attempt := 0; attempt < 1500; attempt++ {
				supervisor.mutex.Lock()
				snapshot := supervisor.journal
				supervisor.mutex.Unlock()
				preview := snapshot.Request
				preview.ManualRecovery = &contract.EdgeManualRecovery{FailedActivationID: snapshot.Result.ID,
					Nonce: Hash(fmt.Sprintf("fresh preview %d", attempt)), ExpiresAt: time.Now().Add(time.Minute).UTC()}
				_, _ = supervisor.accept(preview, true)
				if snapshot.Result.Status == "healthy" || snapshot.Result.Status == "failed" {
					completed = true
					break
				}
				time.Sleep(5 * time.Millisecond)
			}
			if !completed || starts.Load() != 1 {
				t.Fatal("concurrent preview changed single-attempt lifecycle", completed, starts.Load())
			}
		})
	}
}
