// Package terminalsession owns native interactive terminal sessions. A shell
// profile grants broad interactive authority; keystrokes are not parsed into
// purportedly validated commands. The session is not an operating-system sandbox.
package terminalsession

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"path/filepath"
	"strings"
	"time"

	"dockpipe/src/lib/domain/invocation"
)

// These limits match the desktop terminal's eight visible sessions, retained
// output, pending input and character-cell dimensions. Leases are deliberately
// short: a disconnected remote controller cannot leave a shell unattended.
const (
	MaxSessions  = 8
	MaxOutput    = 256 * 1024
	MaxInput     = 64 * 1024
	MaxDimension = 1000
	MaxLease     = 10 * time.Second
	inputTimeout = 2 * time.Second
)

var (
	ErrUnsupported       = errors.New("native terminal is unsupported on this platform")
	ErrInvalid           = errors.New("invalid terminal request")
	ErrCapacity          = errors.New("terminal session limit reached; release a completed session")
	ErrClosed            = errors.New("terminal session is closed")
	ErrActive            = errors.New("terminal session is still active")
	ErrSequence          = errors.New("terminal input sequence is not the next or latest acknowledged sequence")
	ErrInputUncertain    = errors.New("terminal input delivery is uncertain; session closed and bytes must not be retried")
	ErrInputBusy         = errors.New("terminal input is busy; wait for its acknowledgement")
	ErrLeaseExpired      = errors.New("terminal session lease expired")
	ErrCleanupIncomplete = errors.New("terminal process cleanup could not be verified")
)

// Profile comes only from trusted host-owner configuration, never from a remote
// request. Env is the complete environment, not a set of caller overrides. No
// ambient environment, executable lookup, cwd fallback or shell substitution is
// performed. Executable/Dir remain native filesystem preconditions at launch.
type Profile struct {
	ID         string   `json:"id"`
	Executable string   `json:"executable"`
	Args       []string `json:"args"`
	Env        []string `json:"env"`
	Dir        string   `json:"dir"`
	// Route and EnvironmentMode are explicit only for a trusted local provider.
	// A host route binds declared overrides, not a snapshot of host ambient env.
	Route           string `json:"route,omitempty"`
	EnvironmentMode string `json:"environment_mode,omitempty"`
}

// Authority must be asserted by trusted native/transport composition. A remote
// target always needs an authorizer. Native process identity is attribution,
// not proof of a human grant. Authorize must be safe for concurrent calls.
type Authority struct {
	Target    invocation.Target
	Identity  invocation.Identity
	Authorize invocation.Authorize
}

type Size struct{ Columns, Rows int }

func (size Size) valid() bool {
	return size.Columns >= 1 && size.Columns <= MaxDimension && size.Rows >= 1 && size.Rows <= MaxDimension
}

func (profile Profile) validate() error {
	if profile.ID == "" || !filepath.IsAbs(profile.Executable) || !filepath.IsAbs(profile.Dir) {
		return ErrInvalid
	}
	if profile.Route != "" && profile.Route != "native" && profile.Route != "host" {
		return ErrInvalid
	}
	if profile.Route == "host" && profile.EnvironmentMode != "inherit-host" || profile.Route != "host" && profile.EnvironmentMode != "" && profile.EnvironmentMode != "complete" {
		return ErrInvalid
	}
	values := append([]string{profile.ID, profile.Executable, profile.Dir}, profile.Args...)
	values = append(values, profile.Env...)
	for _, value := range values {
		if strings.ContainsRune(value, 0) {
			return ErrInvalid
		}
	}
	for _, entry := range profile.Env {
		// Windows drive entries (=C:=...) are valid owner-provided environment.
		separator := strings.IndexByte(strings.TrimPrefix(entry, "="), '=')
		if separator < 1 {
			return ErrInvalid
		}
	}
	return nil
}

func digest(values ...string) string {
	hash := sha256.New()
	var length [8]byte
	for _, value := range values {
		binary.BigEndian.PutUint64(length[:], uint64(len(value)))
		_, _ = hash.Write(length[:])
		_, _ = hash.Write([]byte(value))
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func admit[T any](ctx context.Context, authority Authority, action, revision, inputDigest string, execute func(context.Context) (T, error)) (T, error) {
	id := invocation.ID{Capability: "dockpipe.terminal", Action: action, Version: "1"}
	contract, err := invocation.New(invocation.Definition{ID: id, Target: authority.Target, RequireAuthorization: authority.Authorize != nil},
		func(context.Context, struct{}) (invocation.Prepared[T], error) {
			return invocation.Prepared[T]{Digest: inputDigest, Revision: revision, Execute: execute}, nil
		}, authority.Authorize)
	if err != nil {
		var zero T
		return zero, err
	}
	return contract.Invoke(ctx, invocation.Call{Contract: id, Identity: authority.Identity, Target: authority.Target}, struct{}{})
}

func validateAuthority(authority Authority) error {
	if authority.Identity.Issuer == "" || authority.Identity.Subject == "" {
		return invocation.ErrInvalidCall
	}
	_, err := invocation.New(invocation.Definition{ID: invocation.ID{Capability: "dockpipe.terminal", Action: "open", Version: "1"}, Target: authority.Target},
		func(context.Context, struct{}) (invocation.Prepared[struct{}], error) {
			return invocation.Prepared[struct{}]{}, nil
		}, authority.Authorize)
	return err
}

func (manager *Manager) remote() bool {
	return manager.authority.Target.Kind == invocation.Remote
}
