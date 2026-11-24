//go:build linux || darwin

package remote

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"dockpipe/src/lib/infrastructure/osauth"
)

func TestSecurityIPCConcurrentProofRequestsApplyBackpressure(t *testing.T) {
	_, broker, _, _ := testBroker(t)
	directory, err := os.MkdirTemp("", "dp-conc-")
	if err != nil {
		t.Fatal(err)
	}
	directory, err = filepath.EvalSymlinks(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(directory)
	platform := &slowNetworkAdmissionPlatform{securityFixturePlatform: &securityFixturePlatform{directory: directory, snapshot: osauth.Snapshot{LockKnown: true, SleepKnown: true}}, delay: 100 * time.Millisecond}
	stop, err := broker.startOperatorSecurityAt(context.Background(), platform, platform.directory)
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	if err = broker.security.Unlock(ctx, os.Getpid()); err != nil {
		t.Fatal(err)
	}
	const clients = 12
	start := make(chan struct{})
	results := make(chan error, clients)
	for index := 0; index < clients; index++ {
		go func() {
			<-start
			response, err := callSecurityAt(ctx, platform.directory, SecurityRequest{Action: "sign", Audience: broker.state.Identity.PublicKey, Path: "/v1/network-pulse", Digest: bodyDigest([]byte("null"))})
			if err == nil && response.Proof == nil {
				err = errors.New("missing one-use proof")
			}
			results <- err
		}()
	}
	close(start)
	failures := 0
	for count := 0; count < clients; count++ {
		select {
		case err := <-results:
			if err != nil {
				failures++
				t.Logf("local proof request refused: %T", err)
			}
		case <-ctx.Done():
			t.Fatal("bounded local proof burst did not finish")
		}
	}
	if failures != 0 {
		t.Fatalf("global IPC burst dropped %d of %d legitimate proof requests before any HTTP admission", failures, clients)
	}
}

func TestOperatorVerificationQueueCancellationAndSaturationConsumeProofs(t *testing.T) {
	for _, scenario := range []string{"cancel", "capacity", "lock", "expired", "generation"} {
		t.Run(scenario, func(t *testing.T) {
			_, broker, _, _ := testBroker(t)
			authority := broker.security
			queryPlatform := &countedSecurityQueryPlatform{securityFixturePlatform: authority.platform.(*securityFixturePlatform)}
			authority.platform = queryPlatform
			proof, err := authority.Sign(context.Background(), broker.state.Identity.PublicKey, "/v1/network-pulse", bodyDigest([]byte("null")))
			if err != nil {
				t.Fatal(err)
			}
			for count := 0; count < cap(authority.verificationSlots); count++ {
				authority.verificationSlots <- struct{}{}
			}
			if scenario == "capacity" {
				for count := 0; count < cap(authority.verificationWaiters); count++ {
					authority.verificationWaiters <- struct{}{}
				}
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() {
				_, err := authority.Verify(ctx, proof.Header, "/v1/network-pulse", []byte("null"))
				done <- err
			}()
			if scenario != "capacity" {
				deadline := time.NewTimer(time.Second)
				defer deadline.Stop()
				for len(authority.verificationWaiters) == 0 {
					select {
					case <-deadline.C:
						t.Fatal("valid proof never entered bounded waiting admission")
					case <-time.After(time.Millisecond):
					}
				}
				switch scenario {
				case "cancel":
					cancel()
				case "generation":
					authority.mutex.Lock()
					authority.generation++
					authority.mutex.Unlock()
					<-authority.verificationSlots
				case "lock":
					authority.Lock()
					<-authority.verificationSlots
				case "expired":
					authority.mutex.Lock()
					authority.now = func() time.Time { return time.Now().Add(30 * time.Second) }
					authority.mutex.Unlock()
					<-authority.verificationSlots
				}
			}
			select {
			case err = <-done:
				if err == nil {
					t.Fatal("cancelled, locked, expired or saturated admission succeeded")
				}
				if scenario == "cancel" && !errors.Is(err, context.Canceled) {
					t.Fatal("queue cancellation lost its outcome", err)
				}
			case <-time.After(time.Second):
				t.Fatal("bounded admission failed to release queued work")
			}
			if queryPlatform.queries.Load() != 1 {
				t.Fatal("ended admission performed a native query after waiting")
			}
			if scenario != "capacity" && len(authority.verificationWaiters) != 0 {
				t.Fatal("cancelled admission retained waiting capacity")
			}
			if _, err = authority.Verify(context.Background(), proof.Header, "/v1/network-pulse", []byte("null")); err == nil {
				t.Fatal("consumed queued proof was replayed")
			}
		})
	}
}

func TestOperatorVerificationQueueHonorsOriginalProofDeadline(t *testing.T) {
	_, broker, _, _ := testBroker(t)
	authority := broker.security
	for count := 0; count < cap(authority.verificationSlots); count++ {
		authority.verificationSlots <- struct{}{}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := authority.acquireVerification(ctx, operatorAdmission{deadline: time.Now().Add(30 * time.Millisecond)})
	if err == nil || ctx.Err() != nil {
		t.Fatal("original proof expiry did not bound queue waiting", err)
	}
	if len(authority.verificationWaiters) != 0 {
		t.Fatal("expired proof retained a queue reservation")
	}
}

func TestOperatorVerificationQueueWaitBudget(t *testing.T) {
	_, broker, _, _ := testBroker(t)
	authority := broker.security
	for count := 0; count < cap(authority.verificationSlots); count++ {
		authority.verificationSlots <- struct{}{}
	}
	ctx, cancel := context.WithTimeout(context.Background(), operatorVerificationQueueWait+time.Second)
	defer cancel()
	started := time.Now()
	_, err := authority.acquireVerification(ctx, operatorAdmission{deadline: time.Now().Add(time.Minute)})
	if err == nil || ctx.Err() != nil || time.Since(started) > operatorVerificationQueueWait+500*time.Millisecond {
		t.Fatal("verification waiting exceeded its own budget", err)
	}
	if len(authority.verificationWaiters) != 0 {
		t.Fatal("queue timeout retained waiting capacity")
	}
}

type countedSecurityQueryPlatform struct {
	*securityFixturePlatform
	queries atomic.Int64
}

func (platform *countedSecurityQueryPlatform) Snapshot(ctx context.Context) (osauth.Snapshot, error) {
	platform.queries.Add(1)
	return platform.securityFixturePlatform.Snapshot(ctx)
}
