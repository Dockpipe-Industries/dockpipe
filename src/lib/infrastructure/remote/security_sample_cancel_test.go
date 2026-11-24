package remote

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"dockpipe/src/lib/infrastructure/osauth"
)

type scriptedSecuritySamplePlatform struct {
	*securityFixturePlatform
	take func(context.Context) (osauth.Snapshot, error)
}

func (platform *scriptedSecuritySamplePlatform) Snapshot(ctx context.Context) (osauth.Snapshot, error) {
	return platform.take(ctx)
}

type publishedSecuritySample struct {
	sequence  uint64
	sampled   time.Time
	snapshot  osauth.Snapshot
	available bool
}

func readPublishedSecuritySample(authority *SecurityAuthority) publishedSecuritySample {
	authority.mutex.Lock()
	defer authority.mutex.Unlock()
	return publishedSecuritySample{authority.publishedSampleSequence, authority.sampledAt, authority.snapshot, authority.available}
}

func securitySampleCancellationFixture(t *testing.T) (*SecurityAuthority, *scriptedSecuritySamplePlatform) {
	t.Helper()
	base := &securityFixturePlatform{directory: privateTestDirectory(t), snapshot: osauth.Snapshot{IdleKnown: true, LockKnown: true, SleepKnown: true}}
	identity := remoteidentityMustNew(t)
	authority, err := NewSecurityAuthority(base, base.directory, identity.PublicKey, os.Getuid())
	if err != nil {
		t.Fatal(err)
	}
	if err = authority.Unlock(context.Background(), os.Getpid()); err != nil {
		t.Fatal(err)
	}
	platform := &scriptedSecuritySamplePlatform{securityFixturePlatform: base, take: base.Snapshot}
	authority.platform = platform
	t.Cleanup(authority.Lock)
	return authority, platform
}

func securityAuthorityActive(authority *SecurityAuthority) bool {
	authority.mutex.Lock()
	defer authority.mutex.Unlock()
	return authority.identity != nil
}

func TestSecuritySampleCallerCancellationPreservesPublishedState(t *testing.T) {
	for _, kind := range []string{"cancel", "deadline"} {
		t.Run(kind, func(t *testing.T) {
			authority, platform := securitySampleCancellationFixture(t)
			before := readPublishedSecuritySample(authority)
			var ctx context.Context
			var cancel context.CancelFunc
			if kind == "cancel" {
				ctx, cancel = context.WithCancel(context.Background())
			} else {
				ctx, cancel = context.WithTimeout(context.Background(), 30*time.Millisecond)
			}
			defer cancel()
			entered := make(chan struct{})
			done := make(chan struct{})
			platform.take = func(query context.Context) (osauth.Snapshot, error) {
				close(entered)
				<-query.Done()
				return osauth.Snapshot{Locked: true}, fmt.Errorf("native query interrupted: %w", query.Err())
			}
			go func() { defer close(done); authority.Sample(ctx) }()
			<-entered
			if kind == "cancel" {
				cancel()
			}
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("caller cancellation did not end native sample")
			}
			if after := readPublishedSecuritySample(authority); after != before {
				t.Fatal("caller-aborted sample overwrote global OS evidence")
			}
			if !securityAuthorityActive(authority) {
				t.Fatal("caller-aborted sample locked the unrelated global authority")
			}
		})
	}
}

func TestSecuritySampleCallerAbortKeepsPositiveEvidence(t *testing.T) {
	for _, kind := range []string{"known-lock", "known-resume"} {
		t.Run(kind, func(t *testing.T) {
			authority, platform := securitySampleCancellationFixture(t)
			before := readPublishedSecuritySample(authority)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			platform.take = func(query context.Context) (osauth.Snapshot, error) {
				cancel()
				partial := osauth.Snapshot{Locked: true}
				if kind == "known-lock" {
					partial.LockKnown = true
				} else {
					partial.SleepKnown = true
					partial.SleepGeneration = before.snapshot.SleepGeneration + 1
				}
				return partial, query.Err()
			}
			authority.Sample(ctx)
			if securityAuthorityActive(authority) {
				t.Fatal("affirmative lock/sleep evidence was discarded with caller cancellation")
			}
			if after := readPublishedSecuritySample(authority); after != before {
				t.Fatal("partial aborted evidence refreshed global sample metadata")
			}
		})
	}
}

func TestSecuritySampleGenuineFailureStillLocksAfterCallerAbort(t *testing.T) {
	for _, kind := range []string{"native-error", "independent-context-error", "nil-error-locked", "nil-error-unknown", "negative-idle", "own-timeout-parent-late-cancel", "own-timeout-nil-error-parent-late-cancel"} {
		t.Run(kind, func(t *testing.T) {
			authority, platform := securitySampleCancellationFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			platform.take = func(query context.Context) (osauth.Snapshot, error) {
				if kind == "independent-context-error" {
					return osauth.Snapshot{}, context.Canceled
				}
				if kind == "own-timeout-parent-late-cancel" || kind == "own-timeout-nil-error-parent-late-cancel" {
					<-query.Done()
					cancel()
					if kind == "own-timeout-nil-error-parent-late-cancel" {
						return osauth.Snapshot{LockKnown: true, SleepKnown: true}, nil
					}
					return osauth.Snapshot{}, query.Err()
				}
				cancel()
				switch kind {
				case "native-error":
					return osauth.Snapshot{}, errors.New("genuine OS service failure")
				case "nil-error-locked":
					return osauth.Snapshot{LockKnown: true, Locked: true, SleepKnown: true}, nil
				case "nil-error-unknown":
					return osauth.Snapshot{Locked: true}, nil
				case "negative-idle":
					return osauth.Snapshot{IdleKnown: true, Idle: -time.Second}, query.Err()
				}
				panic("unhandled fixture")
			}
			authority.Sample(ctx)
			if securityAuthorityActive(authority) {
				t.Fatal("caller cancellation bypassed genuine OS failure or the sampler-owned deadline")
			}
		})
	}
}

type securitySampleOrderingKey struct{}

func TestSecuritySampleOlderIndependentLockSurvivesNewerCallerAbort(t *testing.T) {
	authority, platform := securitySampleCancellationFixture(t)
	olderEntered := make(chan struct{})
	newerEntered := make(chan struct{})
	releaseOlder := make(chan struct{})
	olderDone := make(chan struct{})
	newerDone := make(chan struct{})
	platform.take = func(query context.Context) (osauth.Snapshot, error) {
		if query.Value(securitySampleOrderingKey{}) == "older" {
			close(olderEntered)
			<-releaseOlder
			return osauth.Snapshot{LockKnown: true, Locked: true, SleepKnown: true}, nil
		}
		close(newerEntered)
		<-query.Done()
		return osauth.Snapshot{}, query.Err()
	}
	older := context.WithValue(context.Background(), securitySampleOrderingKey{}, "older")
	go func() { defer close(olderDone); authority.Sample(older) }()
	<-olderEntered
	newer, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { defer close(newerDone); authority.Sample(newer) }()
	<-newerEntered
	cancel()
	<-newerDone
	close(releaseOlder)
	<-olderDone
	published := readPublishedSecuritySample(authority)
	if !published.snapshot.LockKnown || !published.snapshot.Locked || securityAuthorityActive(authority) {
		t.Fatal("newer caller-abort suppressed older independent affirmative lock evidence")
	}
}

func TestSecuritySampleRepeatedCallerAbortsCannotRefreshStalenessOrLeases(t *testing.T) {
	authority, platform := securitySampleCancellationFixture(t)
	before := readPublishedSecuritySample(authority)
	authority.mutex.Lock()
	now := authority.sampledAt
	unlockedAt, expiresAt := authority.unlockedAt, authority.expiresAt
	authority.now = func() time.Time { return now }
	authority.mutex.Unlock()
	platform.take = func(query context.Context) (osauth.Snapshot, error) { return osauth.Snapshot{}, query.Err() }
	for count := 1; count <= 4; count++ {
		now = before.sampled.Add(time.Duration(count) * time.Second)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		authority.Sample(ctx)
	}
	if after := readPublishedSecuritySample(authority); after != before {
		t.Fatal("repeated caller aborts refreshed global native activity")
	}
	authority.mutex.Lock()
	unchanged := authority.unlockedAt == unlockedAt && authority.expiresAt == expiresAt
	authority.mutex.Unlock()
	if !unchanged {
		t.Fatal("caller cancellation changed idle or absolute authority lifetime")
	}
	if !authority.Status().Locked {
		t.Fatal("caller cancellation bypassed the existing three-second staleness gate")
	}
}

func TestSecuritySampleCanceledSigningAndVerificationNeverAdmitOrReplay(t *testing.T) {
	for _, kind := range []string{"sign", "verify"} {
		t.Run(kind, func(t *testing.T) {
			authority, platform := securitySampleCancellationFixture(t)
			proof, err := authority.Sign(context.Background(), authority.audience, "/v1/network-pulse", bodyDigest([]byte("null")))
			if err != nil {
				t.Fatal(err)
			}
			if kind == "sign" {
				authority.mutex.Lock()
				clear(authority.nonces)
				authority.mutex.Unlock()
			}
			before := readPublishedSecuritySample(authority)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			entered := make(chan struct{})
			platform.take = func(query context.Context) (osauth.Snapshot, error) {
				close(entered)
				<-query.Done()
				return osauth.Snapshot{}, query.Err()
			}
			done := make(chan error, 1)
			go func() {
				if kind == "sign" {
					next, err := authority.Sign(ctx, authority.audience, "/v1/network-pulse", bodyDigest([]byte("null")))
					if next.Header != "" {
						err = errors.New("canceled signer produced a fresh proof")
					}
					done <- err
				} else {
					_, err := authority.Verify(ctx, proof.Header, "/v1/network-pulse", []byte("null"))
					done <- err
				}
			}()
			<-entered
			cancel()
			if err = <-done; !errors.Is(err, context.Canceled) {
				t.Fatal("request cancellation lost its failure outcome", err)
			}
			if after := readPublishedSecuritySample(authority); after != before || !securityAuthorityActive(authority) {
				t.Fatal("request cancellation poisoned the independent global OS authority")
			}
			authority.mutex.Lock()
			remaining := len(authority.nonces)
			authority.mutex.Unlock()
			if remaining != 0 {
				t.Fatal("canceled request retained or produced an authorization nonce")
			}
			if kind == "verify" {
				if _, err = authority.Verify(context.Background(), proof.Header, "/v1/network-pulse", []byte("null")); err == nil {
					t.Fatal("canceled verification replayed its consumed proof")
				}
			}
		})
	}
}

func TestSecuritySampleStaleAbortedPartialEvidenceCannotRevokeFreshUnlock(t *testing.T) {
	for _, kind := range []string{"known-lock", "known-sleep"} {
		t.Run(kind, func(t *testing.T) {
			authority, platform := securitySampleCancellationFixture(t)
			// Equal clock readings must still preserve the order the samples began.
			now := authority.now()
			authority.now = func() time.Time { return now }

			entered := make(chan struct{})
			release := make(chan struct{})
			done := make(chan struct{})
			platform.take = func(query context.Context) (osauth.Snapshot, error) {
				if query.Value(securitySampleOrderingKey{}) == "older" {
					close(entered)
					<-release
					if kind == "known-lock" {
						return osauth.Snapshot{LockKnown: true, Locked: true}, query.Err()
					}
					return osauth.Snapshot{SleepKnown: true, SleepGeneration: 0}, query.Err()
				}
				return platform.securityFixturePlatform.Snapshot(query)
			}
			older, cancel := context.WithCancel(context.WithValue(context.Background(), securitySampleOrderingKey{}, "older"))
			defer cancel()
			go func() { defer close(done); authority.Sample(older) }()
			<-entered
			if kind == "known-sleep" {
				platform.securityFixturePlatform.mutex.Lock()
				platform.securityFixturePlatform.snapshot.SleepGeneration = 1
				platform.securityFixturePlatform.mutex.Unlock()
			}
			authority.Sample(context.Background())
			if err := authority.Unlock(context.Background(), os.Getpid()); err != nil {
				t.Fatal(err)
			}
			fresh := readPublishedSecuritySample(authority)
			cancel()
			close(release)
			<-done
			if !securityAuthorityActive(authority) {
				t.Fatal("older aborted partial evidence revoked a newer independently sampled unlock")
			}
			if after := readPublishedSecuritySample(authority); after != fresh {
				t.Fatal("older aborted partial evidence overwrote a newer valid sample")
			}
		})
	}
}
