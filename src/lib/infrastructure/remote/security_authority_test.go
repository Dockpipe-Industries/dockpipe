package remote

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure/osauth"
)

type securityFixturePlatform struct {
	mutex        sync.Mutex
	directory    string
	snapshot     osauth.Snapshot
	authError    error
	authCalls    int
	authenticate func(context.Context) error
}

func (platform *securityFixturePlatform) Authenticate(ctx context.Context, pid int) error {
	platform.mutex.Lock()
	platform.authCalls++
	err, authenticate := platform.authError, platform.authenticate
	platform.mutex.Unlock()
	if authenticate != nil {
		return authenticate(ctx)
	}
	return err
}

func (platform *securityFixturePlatform) Snapshot(ctx context.Context) (osauth.Snapshot, error) {
	platform.mutex.Lock()
	defer platform.mutex.Unlock()
	return platform.snapshot, nil
}

func (platform *securityFixturePlatform) SecurityDirectory() (string, error) {
	return platform.directory, nil
}

func (platform *securityFixturePlatform) SuspendGeneration() (uint64, error) {
	platform.mutex.Lock()
	defer platform.mutex.Unlock()
	return platform.snapshot.SleepGeneration, nil
}

func testSecurityAuthority(t *testing.T, broker *Broker) (*SecurityAuthority, *securityFixturePlatform) {
	t.Helper()
	platform := &securityFixturePlatform{directory: privateTestDirectory(t), snapshot: osauth.Snapshot{IdleKnown: true, LockKnown: true, SleepKnown: true}}
	authority, err := NewSecurityAuthority(platform, platform.directory, broker.state.Identity.PublicKey, os.Getuid())
	if err != nil {
		t.Fatal(err)
	}
	broker.security = authority
	if err := authority.Unlock(context.Background(), os.Getpid()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(authority.Lock)
	return authority, platform
}

func TestOperatorHTTPRejectsBearerAndReplayedOrSubstitutedProof(t *testing.T) {
	_, broker, admin, server := testBroker(t)
	authority := broker.security
	raw, err := NewClient(server.URL, admin.Token)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.HTTP.CloseIdleConnections()
	if err := raw.Call(context.Background(), "/v1/nodes", nil, nil); err == nil {
		t.Fatal("administrator bearer bypassed OS unlock proof")
	}
	proof, err := authority.Sign(context.Background(), broker.state.Identity.PublicKey, "/v1/nodes", bodyDigest([]byte("null")))
	if err != nil {
		t.Fatal(err)
	}
	raw.OperatorProof = func(context.Context, string, []byte) (SecurityProof, error) { return proof, nil }
	if err := raw.Call(context.Background(), "/v1/nodes", nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := raw.Call(context.Background(), "/v1/nodes", nil, nil); err == nil {
		t.Fatal("operator proof replay accepted")
	}
	for _, change := range []struct{ path, body string }{{"/v1/jobs", "null"}, {"/v1/nodes", "{}"}} {
		proof, err := authority.Sign(context.Background(), broker.state.Identity.PublicKey, "/v1/nodes", bodyDigest([]byte("null")))
		if err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequest(http.MethodPost, change.path, nil)
		request.Header.Set(operatorProofHeader, proof.Header)
		if _, err := authority.Verify(context.Background(), request.Header.Get(operatorProofHeader), change.path, []byte(change.body)); err == nil {
			t.Fatal("substituted operator operation or body accepted")
		}
	}
	authority.Lock()
	if err := admin.Call(context.Background(), "/v1/nodes", nil, nil); err == nil {
		t.Fatal("locked client issued new operator action")
	}
}

func TestOperatorIdleSleepAndAbsoluteExpiry(t *testing.T) {
	_, broker, _, _ := testBroker(t)
	for _, mode := range []string{"idle", "sleep", "screen", "absolute", "fallback", "missing-lock"} {
		t.Run(mode, func(t *testing.T) {
			authority, platform := testSecurityAuthority(t, broker)
			now := time.Now()
			authority.now = func() time.Time { return now }
			platform.mutex.Lock()
			switch mode {
			case "idle":
				platform.snapshot.Idle = 300 * time.Second
			case "sleep":
				platform.snapshot.SleepGeneration++
			case "screen":
				platform.snapshot.Locked = true
			case "absolute":
				now = now.Add(time.Hour + time.Second)
			case "fallback":
				platform.snapshot.IdleKnown = false
				now = now.Add(301 * time.Second)
			case "missing-lock":
				platform.snapshot.LockKnown = false
			}
			platform.mutex.Unlock()
			authority.Sample(context.Background())
			if !authority.Status().Locked {
				t.Fatal("remote access survived its security boundary")
			}
		})
	}
}

func TestOperatorPolicyWeakeningAuthenticatesAndChangesLock(t *testing.T) {
	_, broker, _, _ := testBroker(t)
	authority, platform := testSecurityAuthority(t, broker)
	policy := contract.DefaultRemoteSecurityPolicy()
	policy.IdleSeconds = 60
	if err := authority.UpdatePolicy(context.Background(), os.Getpid(), policy); err != nil {
		t.Fatal(err)
	}
	if platform.authCalls != 1 || !authority.Status().Locked {
		t.Fatal("tightening prompted or unlocked access")
	}
	policy.IdleSeconds = 600
	platform.authError = errors.New("authentication denied")
	if err := authority.UpdatePolicy(context.Background(), os.Getpid(), policy); err == nil || authority.Status().IdleSeconds != 60 {
		t.Fatal("unauthenticated policy weakening accepted")
	}
	platform.authError = nil
	if err := authority.UpdatePolicy(context.Background(), os.Getpid(), policy); err != nil || !authority.Status().Locked {
		t.Fatalf("authenticated policy change: %v", err)
	}
}

func TestLockCancelsLateAuthenticationAndDoesNotBlockWorkers(t *testing.T) {
	_, broker, _, _ := testBroker(t)
	authority, platform := testSecurityAuthority(t, broker)
	started := make(chan struct{})
	release := make(chan struct{})
	platform.authenticate = func(context.Context) error { close(started); <-release; return nil }
	done := make(chan error, 1)
	go func() { done <- authority.Unlock(context.Background(), os.Getpid()) }()
	<-started
	authority.Lock()
	broker.mutex.Lock()
	brokerIdentity := broker.state.Identity.PublicKey
	broker.mutex.Unlock()
	if brokerIdentity == "" {
		t.Fatal("broker state is unavailable while OS authentication is pending")
	}
	close(release)
	if err := <-done; err == nil || !authority.Status().Locked {
		t.Fatal("late OS authentication restored a locked session")
	}
}

func TestSleepBetweenProofVerificationAndAdmissionLocksAccess(t *testing.T) {
	_, broker, _, _ := testBroker(t)
	authority, platform := testSecurityAuthority(t, broker)
	proof, err := authority.Sign(context.Background(), broker.state.Identity.PublicKey, "/v1/nodes", bodyDigest([]byte("null")))
	if err != nil {
		t.Fatal(err)
	}
	admission, err := authority.Verify(context.Background(), proof.Header, "/v1/nodes", []byte("null"))
	if err != nil {
		t.Fatal(err)
	}
	platform.mutex.Lock()
	platform.snapshot.SleepGeneration++
	platform.mutex.Unlock()
	if err := authority.CheckAdmission(admission); err == nil || !authority.Status().Locked {
		t.Fatal("proof survived suspend before final admission")
	}
}

func TestSleepDuringAuthenticationCannotUnlockOnResume(t *testing.T) {
	_, broker, _, _ := testBroker(t)
	authority, platform := testSecurityAuthority(t, broker)
	authority.Lock()
	platform.authenticate = func(context.Context) error {
		platform.mutex.Lock()
		platform.snapshot.SleepGeneration++
		platform.mutex.Unlock()
		return nil
	}
	if err := authority.Unlock(context.Background(), os.Getpid()); err == nil || !authority.Status().Locked {
		t.Fatal("successful OS callback after suspend restored authority")
	}
}

func TestFailedPolicyPublicationRequiresInspectionBeforeUnlock(t *testing.T) {
	_, broker, _, _ := testBroker(t)
	authority, platform := testSecurityAuthority(t, broker)
	if err := os.Mkdir(authority.policyPath, 0o700); err != nil {
		t.Fatal(err)
	}
	policy := contract.DefaultRemoteSecurityPolicy()
	policy.IdleSeconds = 60
	if err := authority.UpdatePolicy(context.Background(), os.Getpid(), policy); err == nil {
		t.Fatal("policy publication unexpectedly succeeded")
	}
	previousCalls := platform.authCalls
	if err := authority.Unlock(context.Background(), os.Getpid()); err == nil || platform.authCalls != previousCalls || !authority.Status().Locked {
		t.Fatal("uncertain policy allowed fresh authorization")
	}
}
