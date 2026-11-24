// Package invocation admits typed native operations without a registry,
// transport, credential store, or persistent execution service.
package invocation

import "errors"

var (
	ErrInvalidContract          = errors.New("invalid native invocation contract")
	ErrInvalidCall              = errors.New("invalid native invocation identity or target")
	ErrContractMismatch         = errors.New("native invocation contract or target changed")
	ErrInvalidPreparation       = errors.New("invalid prepared native invocation")
	ErrAuthorizationUnavailable = errors.New("required native invocation authorization is unavailable")
)

// ID names the exact implementation selected by trusted native composition.
// Resolver discovery capabilities are not invocation IDs or authority grants.
type ID struct {
	Capability string
	Action     string
	Version    string
}

func (id ID) valid() bool {
	return id.Capability != "" && id.Action != "" && id.Version != ""
}

type TargetKind uint8

const (
	Local TargetKind = iota + 1
	Remote
)

// Target is selected by the trusted entrypoint, never inferred from a missing
// endpoint. Remote identities bind a machine key and its current worker lifetime.
type Target struct {
	Kind        TargetKind
	ID          string
	Fingerprint string
	Generation  string
}

func (target Target) valid() bool {
	if target.ID == "" {
		return false
	}
	switch target.Kind {
	case Local:
		return target.Fingerprint == "" && target.Generation == ""
	case Remote:
		return validDigest(target.Fingerprint) && target.Generation != ""
	default:
		return false
	}
}

// Identity is an assertion supplied by a trusted local or authenticated
// transport adapter. It must not be populated from caller JSON as proof of
// identity. Admission does not authenticate these strings or issue credentials.
type Identity struct {
	Issuer  string
	Subject string
}

// Call describes the selected invocation; it contains no requested permissions.
type Call struct {
	Contract ID
	Identity Identity
	Target   Target
}

// Binding is passed by value to existing domain authorization. Digest binds the
// exact owned input, while Revision may bind a domain version or reviewed state.
// Neither field should contain raw arguments, paths, credentials or file content.
type Binding struct {
	Call
	Digest   string
	Revision string
}

func validDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		isDigit := character >= '0' && character <= '9'
		isLowerHexLetter := character >= 'a' && character <= 'f'
		if !isDigit && !isLowerHexLetter {
			return false
		}
	}
	return true
}
