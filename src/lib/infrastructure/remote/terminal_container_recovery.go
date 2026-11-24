package remote

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"dockpipe/src/lib/domain/invocation"
	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/process"
)

const containerTerminalReceiptSchema = "dockpipe.container-terminal-receipt/v1"

type containerTerminalReceipt struct {
	Schema        string                           `json:"schema"`
	Reference     contract.TerminalReference       `json:"reference"`
	Resource      infrastructure.DockerExecReceipt `json:"resource"`
	Scope         *process.ScopeReference          `json:"scope,omitempty"`
	ClientStopped bool                             `json:"client_stopped"`
	Uncertain     bool                             `json:"uncertain"`
	Reconciled    bool                             `json:"reconciled"`
}

type containerTerminalJournal struct {
	mutex sync.Mutex
	root  string
	value containerTerminalReceipt
}

func containerTerminalReceiptPath(root, id string) string {
	return filepath.Join(root, "container-terminals", id+".json")
}

func (journal *containerTerminalJournal) saveLocked() error {
	return saveContainerTerminalReceipt(journal.root, journal.value)
}

func saveContainerTerminalReceipt(root string, receipt containerTerminalReceipt) error {
	unlock, err := lockContainerTerminalStore(root)
	if err != nil {
		return err
	}
	defer unlock()
	if err := PrivateDirectory(filepath.Join(root, "container-terminals")); err != nil {
		return err
	}
	return WritePrivate(containerTerminalReceiptPath(root, receipt.Reference.ID), receipt)
}

func (journal *containerTerminalJournal) observe(resource infrastructure.DockerExecReceipt) error {
	journal.mutex.Lock()
	defer journal.mutex.Unlock()
	journal.value.Resource = resource
	return journal.saveLocked()
}

func (journal *containerTerminalJournal) observeScope(scope process.ScopeReference) error {
	journal.mutex.Lock()
	defer journal.mutex.Unlock()
	journal.value.Scope = &scope
	return journal.saveLocked()
}

func (journal *containerTerminalJournal) clientStopped() error {
	journal.mutex.Lock()
	defer journal.mutex.Unlock()
	journal.value.ClientStopped = true
	if err := journal.saveLocked(); err != nil {
		return err
	}
	return removeContainerTerminalReceipt(journal.root, journal.value.Reference)
}

func (journal *containerTerminalJournal) markUncertain() error {
	journal.mutex.Lock()
	defer journal.mutex.Unlock()
	journal.value.Uncertain = true
	return journal.saveLocked()
}

func loadContainerTerminalReceipt(root string, reference contract.TerminalReference) (containerTerminalReceipt, error) {
	var receipt containerTerminalReceipt
	if reference.Validate() != nil {
		return receipt, errors.New("invalid original container terminal reference")
	}
	unlock, err := lockContainerTerminalStore(root)
	if err != nil {
		return receipt, err
	}
	defer unlock()
	if err := readPrivateBounded(containerTerminalReceiptPath(root, reference.ID), &receipt, 8192); err != nil {
		return receipt, err
	}
	if receipt.Schema != containerTerminalReceiptSchema || receipt.Reference != reference || infrastructure.ValidateDockerExecReceipt(receipt.Resource) != nil {
		return receipt, errors.New("container terminal receipt identity changed")
	}
	return receipt, nil
}

type terminalRecoveryPayload struct {
	Reference         contract.TerminalReference `json:"reference"`
	ExpectedContainer string                     `json:"expected_container,omitempty"`
	Retire            bool                       `json:"retire"`
}

func validateTerminalRecoveryPayload(data json.RawMessage) error {
	var payload terminalRecoveryPayload
	if decodeFeature(data, &payload) != nil || payload.Reference.Validate() != nil ||
		payload.ExpectedContainer != "" && (len(payload.ExpectedContainer) != 64 || strings.Trim(payload.ExpectedContainer, "0123456789abcdef") != "") ||
		payload.Retire && payload.ExpectedContainer == "" {
		return errors.New("container terminal recovery requires an exact original reference")
	}
	return nil
}

func executeTerminalRecoveryFeature(ctx context.Context, permissions FeaturePermissions, request contract.FeatureRequest,
	identity invocation.Identity, authorize invocation.Authorize) (any, error) {
	var payload terminalRecoveryPayload
	if !permissions.DockerRead || validateTerminalRecoveryPayload(request.Payload) != nil || decodeFeature(request.Payload, &payload) != nil ||
		payload.Retire && !permissions.DockerRecovery || !payload.Retire && !permissions.DockerExec {
		return nil, errors.New("container terminal recovery requires its independent worker grant")
	}
	if payload.Reference.Target.Node != request.Target.Node || payload.Reference.Target.Fingerprint != request.Target.Fingerprint {
		return nil, errors.New("container terminal recovery changed the captured machine")
	}
	root, ok := ctx.Value(terminalStateRootKey{}).(string)
	if !ok || root == "" {
		return nil, errors.New("container terminal recovery worker state is unavailable")
	}
	id := invocation.ID{Capability: "dockpipe.containers", Action: "terminal-recover", Version: "1"}
	target := terminalInvocationTarget(request.Target)
	native, err := invocation.New(invocation.Definition{ID: id, Target: target, RequireAuthorization: true},
		func(context.Context, terminalRecoveryPayload) (invocation.Prepared[any], error) {
			return invocation.Prepared[any]{Digest: request.ID, Execute: func(ctx context.Context) (any, error) {
				receipt, err := loadContainerTerminalReceipt(root, payload.Reference)
				if err != nil {
					return nil, err
				}
				if payload.ExpectedContainer != "" && payload.ExpectedContainer != receipt.Resource.Container {
					return nil, errors.New("reviewed container terminal resource changed")
				}
				if receipt.Scope != nil {
					if err := process.StopScope(ctx, *receipt.Scope); err != nil {
						return nil, err
					}
				}
				receipt.ClientStopped = true
				if receipt.Resource.Phase == "planned" || receipt.Resource.Phase == "create-requested" {
					// A durable start-requested phase is required before an attach
					// request can be sent. Empty native scope fences old callers.
					receipt.Reconciled = true
				} else if receipt.Resource.Phase == "running" || receipt.Resource.Phase == "closed" {
					if infrastructure.InspectContainerExecClosure(ctx, receipt.Resource) == nil {
						receipt.Reconciled = true
					}
				}
				if !receipt.Reconciled {
					resource := receipt.Resource.ContainerLifecycleReceipt
					resource.Completed = false
					if err := infrastructure.RecoverContainerLifecycle(ctx, resource, payload.Retire); err != nil {
						return nil, err
					}
					receipt.Reconciled = true
				}
				if err := saveContainerTerminalReceipt(root, receipt); err != nil {
					return nil, err
				}
				return map[string]any{"schema": containerTerminalReceiptSchema, "reference": payload.Reference, "container": receipt.Resource.Container, "reconciled": true}, nil
			}}, nil
		}, authorize)
	if err != nil {
		return nil, err
	}
	return native.Invoke(ctx, invocation.Call{Contract: id, Target: target, Identity: identity}, payload)
}

type terminalStateRootKey struct{}

// WithTerminalStateRoot is trusted worker composition, never a caller-selected
// host path. It routes recovery to the same private worker store as launches.
func WithTerminalStateRoot(ctx context.Context, root string) context.Context {
	return context.WithValue(ctx, terminalStateRootKey{}, root)
}

func removeContainerTerminalReceipt(root string, reference contract.TerminalReference) error {
	unlock, err := lockContainerTerminalStore(root)
	if err != nil {
		return err
	}
	defer unlock()
	return os.Remove(containerTerminalReceiptPath(root, reference.ID))
}

func containerTerminalReceipts(root string) ([]containerTerminalReceipt, error) {
	unlock, err := lockContainerTerminalStore(root)
	if err != nil {
		return nil, err
	}
	defer unlock()
	return containerTerminalReceiptsLocked(root)
}

func containerTerminalReceiptsLocked(root string) ([]containerTerminalReceipt, error) {
	directory, err := os.Open(filepath.Join(root, "container-terminals"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer directory.Close()
	entries, err := directory.ReadDir(2*contract.MaxTerminalSessions + 1)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}

	receipts := make([]containerTerminalReceipt, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			return nil, errors.New("invalid container terminal receipt store")
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		if len(id) != 64 || strings.Trim(id, "0123456789abcdef") != "" {
			return nil, errors.New("invalid container terminal receipt identity")
		}
		var receipt containerTerminalReceipt
		if err := readPrivateBounded(filepath.Join(root, "container-terminals", entry.Name()), &receipt, 8192); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, err
		}
		if receipt.Schema != containerTerminalReceiptSchema || receipt.Reference.ID != id || receipt.Reference.Validate() != nil || infrastructure.ValidateDockerExecReceipt(receipt.Resource) != nil {
			return nil, errors.New("invalid container terminal receipt")
		}
		if receipt.Resource.Completed && receipt.ClientStopped {
			if err := os.Remove(containerTerminalReceiptPath(root, receipt.Reference.ID)); err != nil {
				return nil, err
			}
			continue
		}
		receipts = append(receipts, receipt)
	}
	tail, tailErr := directory.ReadDir(1)
	if len(tail) != 0 || tailErr != nil && !errors.Is(tailErr, io.EOF) || len(receipts) > 2*contract.MaxTerminalSessions {
		return nil, errors.New("container terminal receipt capacity reached")
	}
	return receipts, nil
}

// ContainerTerminalRecoveryCatalog publishes only bounded original references.
// The feature worker invokes this against its private store before publication.
func ContainerTerminalRecoveryCatalog(root string, catalog contract.FeatureCatalog) (contract.FeatureCatalog, error) {
	receipts, err := containerTerminalReceipts(root)
	if err != nil {
		return catalog, err
	}
	catalog.ContainerTerminals = nil
	for _, receipt := range receipts {
		if receipt.Reconciled {
			if receipt.Reference.Target.Generation != catalog.Target.Generation {
				if err := removeContainerTerminalReceipt(root, receipt.Reference); err != nil && !errors.Is(err, os.ErrNotExist) {
					return catalog, err
				}
			}
			continue
		}
		if receipt.Reference.Target.Node != catalog.Target.Node || receipt.Reference.Target.Fingerprint != catalog.Target.Fingerprint {
			return catalog, errors.New("container terminal receipt belongs to another paired identity")
		}
		if !receipt.Uncertain && receipt.Reference.Target.Generation == catalog.Target.Generation {
			continue
		}
		catalog.ContainerTerminals = append(catalog.ContainerTerminals, contract.ContainerTerminalReference{
			Reference: receipt.Reference, Container: receipt.Resource.Container, Phase: receipt.Resource.Phase})
	}
	return catalog, catalog.Validate()
}
