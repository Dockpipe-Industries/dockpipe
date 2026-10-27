package remote

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure/operationrecord"
	"dockpipe/src/lib/infrastructure/osauth"
	"dockpipe/src/lib/infrastructure/remoteidentity"
)

const operatorProofHeader = "X-Dockpipe-Operator-Proof"
const operatorAbsoluteLifetime = time.Hour

type SecurityCapabilities struct {
	Idle       bool `json:"idle"`
	ScreenLock bool `json:"screen_lock"`
	Sleep      bool `json:"sleep"`
}

type SecurityStatus struct {
	contract.RemoteSecurityPolicy
	AuthorityAvailable bool                 `json:"authority_available"`
	Supported          bool                 `json:"supported"`
	Locked             bool                 `json:"locked"`
	AbsoluteSeconds    int                  `json:"absolute_seconds"`
	ExpiresAt          time.Time            `json:"expires_at,omitempty"`
	Capabilities       SecurityCapabilities `json:"capabilities"`
	Detail             string               `json:"detail"`
}

type operatorProof struct {
	Session   string `json:"session"`
	UID       string `json:"uid"`
	Audience  string `json:"audience"`
	Scope     string `json:"scope"`
	Nonce     string `json:"nonce"`
	Deadline  int64  `json:"deadline"`
	Signature string `json:"signature"`
}

type SecurityProof struct {
	Header    string `json:"header"`
	Nonce     string `json:"nonce"`
	BrokerKey string `json:"broker_key"`
}

// SecurityAuthority retains its signing credential only in memory after OS
// authentication. OS account compromise is outside this boundary; it is not a
// substitute for a privileged keystore or a sandbox for same-user malware.
type SecurityAuthority struct {
	mutex             sync.Mutex
	platform          osauth.Platform
	policy            contract.RemoteSecurityPolicy
	policyPath        string
	audience          string
	uid               string
	generation        uint64
	authCancel        context.CancelFunc
	identity          *remoteidentity.Identity
	session           string
	unlockedAt        time.Time
	expiresAt         time.Time
	snapshot          osauth.Snapshot
	sampledAt         time.Time
	sampleStarted     time.Time
	detail            string
	available         bool
	policyUncertain   bool
	nonces            map[string]time.Time
	verificationSlots chan struct{}
	now               func() time.Time
}

func NewSecurityAuthority(platform osauth.Platform, directory, audience string, uid int) (*SecurityAuthority, error) {
	if remoteidentity.Fingerprint(audience) == "" {
		return nil, errors.New("security authority requires a broker machine identity")
	}
	if err := PrivateDirectory(directory); err != nil {
		return nil, err
	}
	policy := contract.DefaultRemoteSecurityPolicy()
	path := filepath.Join(directory, "policy.json")
	if err := readPrivateBounded(path, &policy, 4096); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	return &SecurityAuthority{platform: platform, policy: policy, policyPath: path,
		audience: audience, uid: strconv.Itoa(uid), nonces: map[string]time.Time{}, verificationSlots: make(chan struct{}, 4), now: time.Now,
		detail: "Remote access starts locked; authenticate with the operating system to unlock."}, nil
}

func (authority *SecurityAuthority) Lock() {
	authority.mutex.Lock()
	defer authority.mutex.Unlock()
	authority.locked("Remote access is locked.")
}

func (authority *SecurityAuthority) locked(reason string) {
	wasActive := authority.identity != nil || authority.authCancel != nil
	authority.generation++
	if authority.authCancel != nil {
		authority.authCancel()
	}
	if authority.identity != nil {
		clear(authority.identity.PrivateKey)
	}
	authority.identity = nil
	authority.session = ""
	authority.expiresAt = time.Time{}
	clear(authority.nonces)
	authority.detail = reason
	if wasActive {
		authority.audit("remote.security.lock", reason, "")
	}
}

func (authority *SecurityAuthority) audit(unit, message, failure string) {
	status := operationrecord.OperationStatusDone
	if failure != "" {
		status = operationrecord.OperationStatusFail
	}
	now := authority.now()
	operationrecord.LogOperationResult(os.Stderr, operationrecord.OperationResult{
		Unit: unit, Status: status, Message: message, Error: failure,
		StartedAt: now, FinishedAt: now, IDs: map[string]string{"os_uid": authority.uid},
	})
}

// Sample never treats API traffic as input. A missing idle adapter falls back
// to a fixed timeout from unlock; missing required lock/sleep detection fails closed.
func (authority *SecurityAuthority) Sample(ctx context.Context) {
	started := authority.now()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	snapshot, err := authority.platform.Snapshot(ctx)
	if _, supported := authority.platform.(osauth.SuspendClock); !supported {
		snapshot.SleepKnown = false
	}
	if err == nil && snapshot.IdleKnown && snapshot.Idle < 0 {
		err = errors.New("operating system returned a negative input age")
	}
	authority.mutex.Lock()
	defer authority.mutex.Unlock()
	if started.Before(authority.sampleStarted) {
		return
	}
	authority.sampleStarted = started
	previous := authority.snapshot
	authority.snapshot = snapshot
	// Using query start avoids extending trusted activity by command latency.
	authority.sampledAt = started
	authority.available = err == nil
	if err != nil {
		if authority.identity != nil || authority.authCancel != nil {
			authority.locked("Operating-system security state is unavailable; remote access locked.")
		}
		return
	}
	if (authority.identity != nil || authority.authCancel != nil) && authority.policy.LockOnSleep && snapshot.SleepGeneration != previous.SleepGeneration {
		authority.locked("Computer sleep/resume invalidated remote access.")
	}
	if authority.authCancel != nil && authority.policy.LockOnScreenLock && (!snapshot.LockKnown || snapshot.Locked) {
		authority.locked("OS session locked or lock detection disappeared during authentication.")
	}
	if authority.identity != nil {
		authority.checkLocked()
	}
}

func (authority *SecurityAuthority) checkLocked() error {
	if authority.policyUncertain {
		return errors.New("remote security policy publication is uncertain; inspect the policy and restart the authority")
	}
	if authority.identity == nil {
		return errors.New("remote access is locked; run dockpipe remote unlock")
	}
	now := authority.now()
	if !authority.available || now.Sub(authority.sampledAt) > 3*time.Second ||
		(authority.policy.LockOnScreenLock && (!authority.snapshot.LockKnown || authority.snapshot.Locked)) ||
		(authority.policy.LockOnSleep && !authority.snapshot.SleepKnown) {
		authority.locked("Required operating-system lock/sleep state is unavailable or locked.")
		return errors.New(authority.detail)
	}
	if authority.policy.LockOnSleep {
		clock, supported := authority.platform.(osauth.SuspendClock)
		if !supported {
			authority.locked("Suspend detection is unavailable; remote access locked.")
			return errors.New(authority.detail)
		}
		generation, err := clock.SuspendGeneration()
		if err != nil || generation != authority.snapshot.SleepGeneration {
			authority.locked("Computer sleep/resume invalidated remote access.")
			return errors.New(authority.detail)
		}
	}
	deadline := authority.unlockedAt.Add(time.Duration(authority.policy.IdleSeconds) * time.Second)
	if authority.snapshot.IdleKnown {
		deadline = authority.sampledAt.Add(time.Duration(authority.policy.IdleSeconds)*time.Second - authority.snapshot.Idle)
	}
	if authority.expiresAt.Before(deadline) {
		deadline = authority.expiresAt
	}
	if !securityBefore(now, deadline) {
		authority.locked("Remote access auto-locked after inactivity or its one-hour maximum lifetime.")
		return errors.New(authority.detail)
	}
	return nil
}

// Monotonic time bounds rollback while awake; wall time also bounds suspended
// time on systems whose monotonic clock stops during sleep. Clock changes never
// extend an otherwise expired lease.
func securityBefore(now, deadline time.Time) bool {
	return now.Before(deadline) && now.UnixNano() < deadline.UnixNano()
}

func (authority *SecurityAuthority) authenticate(ctx context.Context, pid int) (uint64, func(), error) {
	authority.mutex.Lock()
	if authority.policyUncertain {
		authority.mutex.Unlock()
		return 0, nil, errors.New("remote security policy publication is uncertain; inspect the policy and restart the authority")
	}
	if authority.authCancel != nil {
		authority.mutex.Unlock()
		return 0, nil, errors.New("operating-system authentication is already pending")
	}
	generation := authority.generation
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	authority.authCancel = cancel
	authority.mutex.Unlock()
	err := authority.platform.Authenticate(ctx, pid)
	if err == nil {
		err = ctx.Err()
	}
	authority.mutex.Lock()
	if generation != authority.generation && err == nil {
		err = errors.New("remote access was locked or its policy changed during authentication")
	}
	if err != nil {
		authority.authCancel = nil
		cancel()
		authority.audit("remote.security.authenticate", "OS authentication did not complete.", "authentication denied, cancelled or invalidated")
	}
	authority.mutex.Unlock()
	if err != nil {
		return generation, nil, err
	}
	// Keep the authentication attempt pending through the final OS snapshot and
	// publication. A lock or sleep in that interval must invalidate late success.
	release := func() {
		authority.mutex.Lock()
		authority.authCancel = nil
		cancel()
		authority.mutex.Unlock()
	}
	return generation, release, nil
}

func (authority *SecurityAuthority) Unlock(ctx context.Context, pid int) error {
	generation, release, err := authority.authenticate(ctx, pid)
	if err != nil {
		return err
	}
	defer release()
	authority.Sample(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	identity, err := remoteidentity.New()
	if err != nil {
		return err
	}
	defer clear(identity.PrivateKey)
	session, err := Secret()
	if err != nil {
		return err
	}
	authority.mutex.Lock()
	defer authority.mutex.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if generation != authority.generation {
		return errors.New("remote access changed during authentication; authenticate again")
	}
	if !authority.available || (authority.policy.LockOnScreenLock && !authority.snapshot.LockKnown) ||
		(authority.policy.LockOnSleep && !authority.snapshot.SleepKnown) {
		return errors.New("required OS lock/sleep integration is unavailable; remote access remains locked")
	}
	if authority.identity != nil {
		clear(authority.identity.PrivateKey)
	}
	authority.identity = &remoteidentity.Identity{PublicKey: identity.PublicKey, PrivateKey: append([]byte(nil), identity.PrivateKey...)}
	authority.session = session
	authority.unlockedAt = authority.now()
	authority.expiresAt = authority.unlockedAt.Add(operatorAbsoluteLifetime)
	clear(authority.nonces)
	if err := authority.checkLocked(); err != nil {
		return err
	}
	authority.detail = "OS-authenticated remote access is unlocked; background activity does not renew it."
	authority.audit("remote.security.unlock", "OS-authenticated remote access unlocked.", "")
	return nil
}

func (authority *SecurityAuthority) UpdatePolicy(ctx context.Context, pid int, policy contract.RemoteSecurityPolicy) error {
	if err := policy.Validate(); err != nil {
		return err
	}
	authority.mutex.Lock()
	if authority.policyUncertain {
		authority.mutex.Unlock()
		return errors.New("remote security policy publication is uncertain; inspect the policy and restart the authority")
	}
	previous, generation := authority.policy, authority.generation
	authority.mutex.Unlock()
	if policy.Weakens(previous) {
		var err error
		var release func()
		generation, release, err = authority.authenticate(ctx, pid)
		if err != nil {
			return err
		}
		defer release()
	}
	authority.mutex.Lock()
	defer authority.mutex.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if generation != authority.generation || previous != authority.policy {
		return errors.New("security policy changed concurrently; review the current policy and retry")
	}
	if err := WritePrivate(authority.policyPath, policy); err != nil {
		authority.policyUncertain = true
		authority.locked("Security policy publication failed; remote access locked for inspection.")
		return err
	}
	authority.policy = policy
	authority.locked("Remote policy changed; authenticate again to unlock.")
	authority.audit("remote.security.policy", "Global remote security policy changed; access remains locked.", "")
	return nil
}

func (authority *SecurityAuthority) Status() SecurityStatus {
	authority.mutex.Lock()
	defer authority.mutex.Unlock()
	_ = authority.checkLocked()
	detail := authority.detail
	if !authority.snapshot.IdleKnown {
		detail += " OS idle detection is unavailable: the inactivity timeout is a fixed limit from unlock, never refreshed by requests."
	}
	if authority.snapshot.Detail != "" {
		detail += " " + authority.snapshot.Detail
	}
	supported := authority.available && !authority.policyUncertain && (!authority.policy.LockOnScreenLock || authority.snapshot.LockKnown) &&
		(!authority.policy.LockOnSleep || authority.snapshot.SleepKnown)
	if !supported {
		detail += " Required OS lock/sleep integration is unavailable; remote access cannot unlock with the current policy."
	}
	return SecurityStatus{RemoteSecurityPolicy: authority.policy, AuthorityAvailable: true, Supported: supported,
		Locked: authority.identity == nil, AbsoluteSeconds: int(operatorAbsoluteLifetime.Seconds()),
		ExpiresAt: authority.expiresAt, Detail: detail,
		Capabilities: SecurityCapabilities{Idle: authority.snapshot.IdleKnown, ScreenLock: authority.snapshot.LockKnown, Sleep: authority.snapshot.SleepKnown}}
}

func operatorMessage(proof operatorProof, path, digest string) []byte {
	proof.Signature = ""
	claims, _ := json.Marshal(proof)
	return []byte(contract.OperatorSecurityVersion + "\nPOST\n" + path + "\n" + digest + "\n" + string(claims))
}

func bodyDigest(body []byte) string {
	digest := sha256.Sum256(body)
	return hex.EncodeToString(digest[:])
}

func (authority *SecurityAuthority) Sign(ctx context.Context, audience, path, digest string) (SecurityProof, error) {
	if audience != authority.audience || contract.OperatorScope(path) == "" || len(digest) != 64 {
		return SecurityProof{}, errors.New("remote signing request has an invalid audience, operation or body digest")
	}
	if _, err := hex.DecodeString(digest); err != nil {
		return SecurityProof{}, errors.New("invalid remote request body digest")
	}
	authority.Sample(ctx)
	if err := ctx.Err(); err != nil {
		return SecurityProof{}, err
	}
	authority.mutex.Lock()
	defer authority.mutex.Unlock()
	if err := authority.checkLocked(); err != nil {
		return SecurityProof{}, err
	}
	now := authority.now()
	for nonce, expires := range authority.nonces {
		if !securityBefore(now, expires) {
			delete(authority.nonces, nonce)
		}
	}
	if len(authority.nonces) >= 128 {
		return SecurityProof{}, errors.New("too many outstanding remote requests; wait for their 20-second deadline")
	}
	nonce, err := Secret()
	if err != nil {
		return SecurityProof{}, err
	}
	expires := now.Add(20 * time.Second)
	if authority.expiresAt.Before(expires) {
		expires = authority.expiresAt
	}
	proof := operatorProof{Session: authority.session, UID: authority.uid, Audience: audience,
		Scope: contract.OperatorScope(path), Nonce: nonce, Deadline: expires.UnixMilli()}
	proof.Signature, err = authority.identity.Sign(operatorMessage(proof, path, digest))
	if err != nil {
		return SecurityProof{}, err
	}
	encoded, err := json.Marshal(proof)
	if err != nil {
		return SecurityProof{}, err
	}
	authority.nonces[nonce] = expires
	return SecurityProof{Header: base64.RawURLEncoding.EncodeToString(encoded), Nonce: nonce, BrokerKey: audience}, nil
}

type operatorAdmission struct {
	nonce      string
	session    string
	generation uint64
	deadline   time.Time
}

func (authority *SecurityAuthority) Verify(ctx context.Context, header, path string, body []byte) (operatorAdmission, error) {
	if len(header) > 4096 {
		return operatorAdmission{}, errors.New("invalid remote session proof")
	}
	encoded, err := base64.RawURLEncoding.DecodeString(header)
	if err != nil {
		return operatorAdmission{}, errors.New("invalid remote session proof")
	}
	var proof operatorProof
	if err := Decode(encoded, &proof); err != nil {
		return operatorAdmission{}, errors.New("invalid remote session proof")
	}
	authority.mutex.Lock()
	if err := authority.checkLocked(); err != nil {
		authority.mutex.Unlock()
		return operatorAdmission{}, err
	}
	expires, exists := authority.nonces[proof.Nonce]
	delete(authority.nonces, proof.Nonce)
	if !exists || proof.Session != authority.session || proof.UID != authority.uid || proof.Audience != authority.audience ||
		proof.Scope == "" || proof.Scope != contract.OperatorScope(path) || proof.Deadline != expires.UnixMilli() ||
		!securityBefore(authority.now(), expires) || !remoteidentity.Verify(authority.identity.PublicKey, operatorMessage(proof, path, bodyDigest(body)), proof.Signature) {
		authority.mutex.Unlock()
		return operatorAdmission{}, errors.New("invalid, expired or replayed remote session proof")
	}
	admission := operatorAdmission{nonce: proof.Nonce, session: authority.session, generation: authority.generation, deadline: expires}
	authority.mutex.Unlock()
	// Only a cryptographically valid, already consumed local proof can cause an
	// OS query. Do this before the broker job mutex, with bounded concurrency.
	select {
	case authority.verificationSlots <- struct{}{}:
		defer func() { <-authority.verificationSlots }()
	default:
		return operatorAdmission{}, errors.New("remote authorization verification is busy")
	}
	authority.Sample(ctx)
	if err := ctx.Err(); err != nil {
		return operatorAdmission{}, err
	}
	return admission, authority.CheckAdmission(admission)
}

func (authority *SecurityAuthority) CheckAdmission(admission operatorAdmission) error {
	authority.mutex.Lock()
	defer authority.mutex.Unlock()
	if err := authority.checkLocked(); err != nil {
		return err
	}
	if admission.nonce == "" || admission.session != authority.session || admission.generation != authority.generation || !securityBefore(authority.now(), admission.deadline) {
		return errors.New("remote authority changed before operation admission")
	}
	return nil
}
