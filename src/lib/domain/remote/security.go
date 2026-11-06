package remote

import "errors"

const OperatorSecurityVersion = "dockpipe.remote.os-session/v1"
const RemoteSecurityPolicyVersion = "dockpipe.remote.security-policy/v1"

// RemoteSecurityPolicy is owned by the current OS account, outside project and
// connection configuration. No unlimited or background-renewed lease exists.
type RemoteSecurityPolicy struct {
	Schema           string `json:"schema"`
	IdleSeconds      int    `json:"idle_seconds"`
	AbsoluteSeconds  int    `json:"absolute_seconds"`
	LockOnScreenLock bool   `json:"lock_on_screen_lock"`
	LockOnSleep      bool   `json:"lock_on_sleep"`
}

func DefaultRemoteSecurityPolicy() RemoteSecurityPolicy {
	return RemoteSecurityPolicy{
		Schema: RemoteSecurityPolicyVersion, IdleSeconds: 300, AbsoluteSeconds: 3600,
		LockOnScreenLock: true, LockOnSleep: true,
	}
}

func (policy RemoteSecurityPolicy) Validate() error {
	if policy.Schema != RemoteSecurityPolicyVersion {
		return errors.New("unsupported global remote security policy schema")
	}
	if policy.IdleSeconds < 60 || policy.IdleSeconds > 43200 {
		return errors.New("remote inactivity timeout must be between 60 and 43200 seconds")
	}
	if policy.AbsoluteSeconds < 60 || policy.AbsoluteSeconds > 43200 {
		return errors.New("remote absolute session lifetime must be between 60 and 43200 seconds")
	}
	return nil
}

func (policy RemoteSecurityPolicy) Weakens(previous RemoteSecurityPolicy) bool {
	return policy.IdleSeconds > previous.IdleSeconds ||
		policy.AbsoluteSeconds > previous.AbsoluteSeconds ||
		(!policy.LockOnScreenLock && previous.LockOnScreenLock) ||
		(!policy.LockOnSleep && previous.LockOnSleep)
}

// OperatorScope admits a known operation, never arbitrary signing or a wildcard.
func OperatorScope(path string) string {
	switch path {
	case "/v1/edge-status", "/v1/nodes", "/v1/jobs", "/v1/job", "/v1/observe", "/v1/pairings":
		return "remote.read"
	case "/v1/submit", "/v1/cancel":
		return "remote.execute"
	case "/v1/revoke", "/v1/edge-plan", "/v1/edge-activate":
		return "remote.manage"
	case "/v1/pairing-open", "/v1/pairing-close", "/v1/approve-begin", "/v1/approve", "/v1/deny":
		return "remote.pair"
	default:
		return ""
	}
}
