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
	case "/v1/network-catalog", "/v1/network-connect", "/v1/network-pulse", "/v1/network-trust":
		return "remote.network.session"
	case DesktopAdministratorGrantPath, DesktopAdministratorRelayTicketPath, DesktopAdministratorApprovalPath:
		return "remote.desktop.administrator-input"
	case "/v1/desktop-catalog", "/v1/desktop-status":
		return "remote.desktop.view"
	case DesktopRelayTicketPath, "/v1/desktop-open", "/v1/desktop-pulse", "/v1/desktop-signal", "/v1/desktop-close":
		return "remote.desktop.session"
	case "/v1/edge-status", "/v1/nodes", "/v1/jobs", "/v1/job", "/v1/job-log", "/v1/observe", "/v1/pairings", JobInterruptionInspectPath,
		"/v1/features-catalog", "/v1/features-read-submit", "/v1/features-read-status", "/v1/terminal-read":
		return "remote.read"
	case "/v1/submit", "/v1/cancel", JobInterruptionAcknowledgePath, "/v1/features-write-submit", "/v1/features-write-status", "/v1/features-cancel", "/v1/features-reconcile",
		"/v1/terminal-open", "/v1/terminal-pulse", "/v1/terminal-input", "/v1/terminal-resize", "/v1/terminal-close", "/v1/terminal-release":
		return "remote.execute"
	case "/v1/revoke", "/v1/remove", "/v1/edge-plan", "/v1/edge-activate":
		return "remote.manage"
	case "/v1/pairing-open", "/v1/pairing-close", "/v1/approve-begin", "/v1/approve", "/v1/deny", "/v1/guest-grant", "/v1/guest-grant-revoke":
		return "remote.pair"
	default:
		return ""
	}
}
