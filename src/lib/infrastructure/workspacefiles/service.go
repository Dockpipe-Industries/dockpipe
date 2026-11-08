package workspacefiles

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"unicode/utf8"

	"dockpipe/src/lib/domain/invocation"
)

// Service owns a root handle and an adapter-verified caller/target. A remote
// adapter selects the root from worker-local grants, never a caller's host path.
// Authorize remains owned by the existing policy/remote authority service.
type Service struct {
	rootPath  string
	target    invocation.Target
	identity  invocation.Identity
	authorize invocation.Authorize
	lifecycle sync.RWMutex
	rootMutex sync.Mutex
	root      *os.Root
	closed    bool
	mutation  sync.Mutex
}

func New(root string, target invocation.Target, identity invocation.Identity, authorize invocation.Authorize) (*Service, error) {
	if !filepath.IsAbs(root) || len(root) > MaxPathBytes || !utf8.ValidString(root) || strings.ContainsRune(root, '\x00') {
		return nil, ErrOutsideRoot
	}
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		return nil, errors.New("workspace file operations are unavailable on this platform")
	}
	return &Service{rootPath: filepath.Clean(root), target: target, identity: identity, authorize: authorize}, nil
}

func (service *Service) Close() error {
	service.lifecycle.Lock()
	defer service.lifecycle.Unlock()
	service.rootMutex.Lock()
	defer service.rootMutex.Unlock()
	service.closed = true
	if service.root == nil {
		return nil
	}
	err := service.root.Close()
	service.root = nil
	return err
}

func (service *Service) openRoot() (*os.Root, error) {
	service.rootMutex.Lock()
	defer service.rootMutex.Unlock()
	if service.closed {
		return nil, os.ErrClosed
	}
	if service.root != nil {
		return service.root, nil
	}
	info, err := os.Lstat(service.rootPath)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, ErrOutsideRoot
	}
	root, err := os.OpenRoot(service.rootPath)
	if err != nil {
		return nil, err
	}
	opened, err := root.Stat(".")
	if err != nil || !os.SameFile(info, opened) {
		root.Close()
		return nil, ErrChanged
	}
	service.root = root
	return root, nil
}

func (service *Service) Run(ctx context.Context, request Request) (Result, error) {
	service.lifecycle.RLock()
	defer service.lifecycle.RUnlock()
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if service.closed {
		return Result{}, os.ErrClosed
	}
	// The dispatcher selects a fixed native contract only after recognizing the
	// action. Unknown input never constructs an executable catch-all contract.
	if err := request.validate(); err != nil {
		return Result{}, err
	}
	id := invocation.ID{Capability: "dockpipe.files", Action: request.Action, Version: "1"}
	definition := invocation.Definition{ID: id, Target: service.target}
	prepare := func(_ context.Context, input Request) (invocation.Prepared[Result], error) {
		if err := input.validate(); err != nil {
			return invocation.Prepared[Result]{}, err
		}
		owned := input
		owned.Content = append([]byte(nil), input.Content...)
		digest := ""
		if service.authorize != nil || service.target.Kind == invocation.Remote {
			canonical, err := json.Marshal(struct {
				Root    string  `json:"root"`
				Request Request `json:"request"`
			}{service.rootPath, owned})
			if err != nil {
				return invocation.Prepared[Result]{}, err
			}
			sum := sha256.Sum256(canonical)
			digest = hex.EncodeToString(sum[:])
		}
		return invocation.Prepared[Result]{Digest: digest, Revision: owned.Revision,
			Execute: func(ctx context.Context) (Result, error) { return service.execute(ctx, owned) }}, nil
	}
	contract, err := invocation.New(definition, prepare, service.authorize)
	if err != nil {
		return Result{}, err
	}
	call := invocation.Call{Contract: id, Target: service.target, Identity: service.identity}
	return contract.Invoke(ctx, call, request)
}

func (service *Service) execute(ctx context.Context, request Request) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	root, err := service.openRoot()
	if err != nil {
		return Result{}, err
	}
	switch request.Action {
	case "list":
		return list(ctx, root, request)
	case "read":
		content, _, err := read(ctx, root, request.Path)
		if err != nil {
			return Result{}, err
		}
		return Result{Schema: Schema, Path: request.Path, Content: content, Revision: revision(content)}, nil
	default:
		service.mutation.Lock()
		defer service.mutation.Unlock()
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		return mutate(ctx, root, request)
	}
}

func revision(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}
