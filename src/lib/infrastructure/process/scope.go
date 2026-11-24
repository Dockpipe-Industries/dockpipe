package process

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// ScopeReference contains kernel lifecycle metadata, never arguments or input.
// The reference is persisted before a process can enter its operation scope.
// A delegated cgroup owns cooperative process descendants, including setsid;
// it is not a security sandbox against a same-UID program deliberately migrating
// into another writable cgroup. Boot-fenced recovery requires a new kernel life.
type ScopeReference struct {
	Mode         string `json:"mode,omitempty"`
	Boot         string `json:"boot"`
	Path         string `json:"path"`
	Inode        uint64 `json:"inode"`
	BootSequence uint32 `json:"windows_boot_sequence,omitempty"`
}

func (reference ScopeReference) Validate() error {
	boot := strings.ToLower(reference.Boot)
	if boot != reference.Boot || len(boot) != 36 || boot[8] != '-' || boot[13] != '-' || boot[18] != '-' || boot[23] != '-' || strings.Trim(boot, "0123456789abcdef-") != "" {
		return errors.New("invalid kernel boot identity")
	}
	if reference.Mode == "host-reboot" {
		if reference.Path != "" || reference.Inode != 0 {
			return errors.New("boot-fenced scope cannot select a process resource")
		}
		return nil
	}
	if reference.Mode != "cgroup-v2" && reference.Mode != "" || !filepath.IsLocal(reference.Path) || len(reference.Path) > 4096 || strings.ContainsAny(reference.Path, "\x00\r\n") || reference.Inode == 0 || reference.BootSequence != 0 {
		return errors.New("invalid cgroup scope reference")
	}
	return nil
}

// CaptureHostBootScope records the native evidence needed to reconcile a
// process scope after a reboot.
func CaptureHostBootScope() (ScopeReference, error) {
	boot, err := HostBootIdentity()
	if err != nil {
		return ScopeReference{}, err
	}

	sequence, err := hostBootSequence()
	if err != nil {
		return ScopeReference{}, err
	}

	reference := ScopeReference{Mode: "host-reboot", Boot: boot, BootSequence: sequence}
	return reference, reference.Validate()
}

func stopHostBootScope(reference ScopeReference) error {
	current, err := CaptureHostBootScope()
	if err != nil {
		return err
	}

	if current.BootSequence != 0 {
		if reference.BootSequence == 0 || reference.BootSequence > current.BootSequence {
			return ErrScopeUnavailable
		}
		if reference.BootSequence == current.BootSequence {
			return ErrRebootRequired
		}
		return nil
	}
	if reference.BootSequence != 0 {
		return ErrScopeUnavailable
	}
	if reference.Boot == current.Boot {
		return ErrRebootRequired
	}

	return nil
}

type Scope struct {
	mu        sync.Mutex
	id        string
	reference *ScopeReference
	persist   func(ScopeReference) error
	root      string
	mode      string
}

type scopeContextKey struct{}

var ErrScopeUnavailable = errors.New("durable process scope is unavailable")
var ErrRebootRequired = errors.New("the worker host must reboot before this uncertain process scope can be reconciled")

func RecoveryMode() string {
	configured := os.Getenv("DOCKPIPE_PROCESS_RECOVERY")
	if configured == "host-reboot" && (runtime.GOOS == "linux" || runtime.GOOS == "darwin" || runtime.GOOS == "windows") {
		return configured
	}
	if configured != "" && configured != "cgroup-v2" {
		return "unavailable"
	}
	if runtime.GOOS == "linux" {
		return "cgroup-v2"
	}
	return "unavailable"
}

func WithScope(ctx context.Context, id string, persist func(ScopeReference) error) (context.Context, *Scope) {
	return WithScopeMode(ctx, id, RecoveryMode(), persist)
}

func WithScopeMode(ctx context.Context, id, mode string, persist func(ScopeReference) error) (context.Context, *Scope) {
	if mode == "" {
		mode = RecoveryMode()
	}
	if mode != "host-reboot" && mode != "cgroup-v2" {
		mode = "unavailable"
	}
	scope := &Scope{id: id, persist: persist, root: os.Getenv("DOCKPIPE_PROCESS_SCOPE_ROOT"), mode: mode}
	return context.WithValue(ctx, scopeContextKey{}, scope), scope
}

func RunWithScope(ctx context.Context, command *exec.Cmd) error {
	closeScope, err := AttachScope(ctx, command)
	if err != nil {
		return err
	}
	defer closeScope()
	return Run(command)
}

// AttachScope must complete before Start. The caller retains ordinary command
// supervision and closes the returned kernel handle after Start/Wait finishes.
func AttachScope(ctx context.Context, command *exec.Cmd) (func(), error) {
	scope, scoped := ctx.Value(scopeContextKey{}).(*Scope)
	if !scoped {
		return func() {}, nil
	}
	scope.mu.Lock()
	defer scope.mu.Unlock()
	if scope.reference == nil {
		if scope.mode == "unavailable" {
			return nil, ErrScopeUnavailable
		}
		var reference ScopeReference
		var err error
		if scope.mode == "host-reboot" {
			reference, err = CaptureHostBootScope()
		} else {
			reference, err = createScope(scope.id, scope.root)
		}
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrScopeUnavailable, err)
		}
		if scope.persist == nil {
			return nil, errors.Join(
				errors.New("durable process scope requires a receipt publisher"),
				discardScope(reference),
			)
		}
		if err := scope.persist(reference); err != nil {
			return nil, errors.Join(err, discardScope(reference))
		}
		scope.reference = &reference
	}
	return attachScope(command, *scope.reference)
}

func discardScope(reference ScopeReference) error {
	if reference.Mode == "host-reboot" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	return StopScope(ctx, reference)
}

func (scope *Scope) Stop(ctx context.Context) error {
	scope.mu.Lock()
	defer scope.mu.Unlock()
	if scope.reference == nil {
		return nil
	}
	return StopScope(ctx, *scope.reference)
}
