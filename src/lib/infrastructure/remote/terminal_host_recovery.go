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
	"dockpipe/src/lib/infrastructure/process"
)

type hostTerminalReceipt struct {
	Schema          string                     `json:"schema"`
	Reference       contract.TerminalReference `json:"reference"`
	Root            string                     `json:"root"`
	Phase           string                     `json:"phase"`
	Scope           process.ScopeReference     `json:"scope"`
	Uncertain       bool                       `json:"uncertain"`
	CleanupVerified bool                       `json:"cleanup_verified"`
	Reconciled      bool                       `json:"reconciled"`
}

func (receipt hostTerminalReceipt) validate() error {
	if receipt.Schema != contract.HostTerminalReceiptVersion || receipt.Reference.Validate() != nil || !contract.ValidID(receipt.Root) ||
		receipt.Scope.Mode != "host-reboot" || receipt.Scope.Validate() != nil ||
		receipt.Phase != "planned" && receipt.Phase != "start-requested" && receipt.Phase != "running" {
		return errors.New("invalid host terminal receipt")
	}
	return nil
}

type hostTerminalJournal struct {
	mutex sync.Mutex
	root  string
	value hostTerminalReceipt
}

func hostTerminalReceiptPath(root, id string) string {
	return filepath.Join(root, "host-terminals", id+".json")
}

func prepareHostTerminalJournal(root string, reference contract.TerminalReference, grant string) (*hostTerminalJournal, error) {
	scope, err := process.CaptureHostBootScope()
	if err != nil {
		return nil, errors.New("host terminal boot identity is unavailable; no PTY was launched")
	}
	journal := &hostTerminalJournal{root: root, value: hostTerminalReceipt{
		Schema: contract.HostTerminalReceiptVersion, Reference: reference, Root: grant, Phase: "planned",
		Scope: scope,
	}}
	if err := saveHostTerminalReceipt(root, journal.value); err != nil {
		return nil, err
	}
	return journal, nil
}

func (journal *hostTerminalJournal) phase(phase string) error {
	journal.mutex.Lock()
	defer journal.mutex.Unlock()
	journal.value.Phase = phase
	return saveHostTerminalReceipt(journal.root, journal.value)
}

func (journal *hostTerminalJournal) finish(verified bool) error {
	journal.mutex.Lock()
	defer journal.mutex.Unlock()
	if journal.value.CleanupVerified == verified && journal.value.Uncertain == !verified {
		return nil
	}
	journal.value.Uncertain = !verified
	journal.value.CleanupVerified = verified
	return saveHostTerminalReceipt(journal.root, journal.value)
}

func saveHostTerminalReceipt(root string, receipt hostTerminalReceipt) error {
	if err := receipt.validate(); err != nil {
		return err
	}
	unlock, err := lockContainerTerminalStore(root)
	if err != nil {
		return err
	}
	defer unlock()
	if _, err := os.Lstat(hostTerminalReceiptPath(root, receipt.Reference.ID)); os.IsNotExist(err) {
		hosts, err := hostTerminalReceiptsLocked(root)
		if err != nil {
			return err
		}
		if len(hosts) >= 2*contract.MaxTerminalSessions {
			return errors.New("host terminal receipt storage budget reached")
		}
		containers, err := containerTerminalReceiptsLocked(root)
		if err != nil {
			return err
		}
		retained := map[string]bool{receipt.Reference.ID: true}
		for _, prior := range hosts {
			if !prior.Reconciled && !prior.CleanupVerified {
				retained[prior.Reference.ID] = true
			}
		}
		for _, prior := range containers {
			if !prior.Reconciled {
				retained[prior.Reference.ID] = true
			}
		}
		if len(retained) > contract.MaxTerminalSessions {
			return errors.New("retained host/container terminal receipts consume the session budget")
		}
	} else if err != nil {
		return err
	}
	if err := PrivateDirectory(filepath.Join(root, "host-terminals")); err != nil {
		return err
	}
	return WritePrivate(hostTerminalReceiptPath(root, receipt.Reference.ID), receipt)
}

func hostTerminalReceipts(root string) ([]hostTerminalReceipt, error) {
	unlock, err := lockContainerTerminalStore(root)
	if err != nil {
		return nil, err
	}
	defer unlock()
	return hostTerminalReceiptsLocked(root)
}

func hostTerminalReceiptsLocked(root string) ([]hostTerminalReceipt, error) {
	path := filepath.Join(root, "host-terminals")
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("invalid host terminal receipt directory")
	}
	directory, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer directory.Close()
	entries, err := directory.ReadDir(2*contract.MaxTerminalSessions + 1)
	if err != nil && !errors.Is(err, io.EOF) || len(entries) > 2*contract.MaxTerminalSessions {
		return nil, errors.New("host terminal receipt capacity exceeded")
	}
	receipts := make([]hostTerminalReceipt, 0, len(entries))
	for _, entry := range entries {
		id := strings.TrimSuffix(entry.Name(), ".json")
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") || !validMachineNonce(id) {
			return nil, errors.New("invalid host terminal receipt entry")
		}
		var receipt hostTerminalReceipt
		if err := readPrivateBounded(hostTerminalReceiptPath(root, id), &receipt, 8192); err != nil {
			return nil, err
		}
		if receipt.Reference.ID != id || receipt.validate() != nil {
			return nil, errors.New("host terminal receipt identity changed")
		}
		receipts = append(receipts, receipt)
	}
	return receipts, nil
}

func loadHostTerminalReceipt(root string, reference contract.TerminalReference) (hostTerminalReceipt, error) {
	var receipt hostTerminalReceipt
	if reference.Validate() != nil {
		return receipt, errors.New("invalid original host terminal reference")
	}
	unlock, err := lockContainerTerminalStore(root)
	if err != nil {
		return receipt, err
	}
	defer unlock()
	if err := readPrivateBounded(hostTerminalReceiptPath(root, reference.ID), &receipt, 8192); err != nil {
		return receipt, err
	}
	if receipt.Reference != reference || receipt.validate() != nil {
		return receipt, errors.New("host terminal receipt identity changed")
	}
	return receipt, nil
}

func removeHostTerminalReceipt(root string, reference contract.TerminalReference) error {
	unlock, err := lockContainerTerminalStore(root)
	if err != nil {
		return err
	}
	defer unlock()
	var receipt hostTerminalReceipt
	if err := readPrivateBounded(hostTerminalReceiptPath(root, reference.ID), &receipt, 8192); err != nil {
		return err
	}
	if receipt.Reference != reference || receipt.validate() != nil {
		return errors.New("host terminal receipt changed before removal")
	}
	return os.Remove(hostTerminalReceiptPath(root, reference.ID))
}

func HostTerminalRecoveryCatalog(root string, catalog contract.FeatureCatalog) (contract.FeatureCatalog, error) {
	receipts, err := hostTerminalReceipts(root)
	if err != nil {
		return catalog, err
	}
	catalog.HostTerminals = nil
	for _, receipt := range receipts {
		if receipt.Reference.Target.Node != catalog.Target.Node || receipt.Reference.Target.Fingerprint != catalog.Target.Fingerprint {
			return catalog, errors.New("host terminal receipt belongs to another paired identity")
		}
		if !receipt.Uncertain && !receipt.Reconciled && receipt.Reference.Target.Generation == catalog.Target.Generation {
			continue
		}
		catalog.HostTerminals = append(catalog.HostTerminals, contract.HostTerminalReference{Reference: receipt.Reference, Root: receipt.Root, Phase: receipt.Phase})
	}
	return catalog, catalog.Validate()
}

type hostTerminalRecoveryPayload struct {
	Reference contract.TerminalReference `json:"reference"`
}

type hostRecoveryObserverKey struct{}

func validateHostTerminalRecoveryPayload(data json.RawMessage) error {
	var payload hostTerminalRecoveryPayload
	if decodeFeature(data, &payload) != nil || payload.Reference.Validate() != nil {
		return errors.New("host terminal recovery requires its exact original reference")
	}
	return nil
}

func executeHostTerminalRecovery(ctx context.Context, permissions FeaturePermissions, request contract.FeatureRequest, identity invocation.Identity, authorize invocation.Authorize) (any, error) {
	var payload hostTerminalRecoveryPayload
	if validateHostTerminalRecoveryPayload(request.Payload) != nil || decodeFeature(request.Payload, &payload) != nil ||
		!terminalRoot(permissions.catalog(request.Target), request.Root) || payload.Reference.Target.Node != request.Target.Node || payload.Reference.Target.Fingerprint != request.Target.Fingerprint {
		return nil, errors.New("host terminal recovery changed its paired identity or root grant")
	}
	root, ok := ctx.Value(terminalStateRootKey{}).(string)
	if !ok || root == "" {
		return nil, errors.New("host terminal worker state is unavailable")
	}
	id := invocation.ID{Capability: "dockpipe.terminal", Action: "host-recover", Version: "1"}
	target := terminalInvocationTarget(request.Target)
	native, err := invocation.New(invocation.Definition{ID: id, Target: target, RequireAuthorization: true},
		func(context.Context, hostTerminalRecoveryPayload) (invocation.Prepared[any], error) {
			return invocation.Prepared[any]{Digest: request.ID, Execute: func(ctx context.Context) (any, error) {
				receipt, err := loadHostTerminalReceipt(root, payload.Reference)
				if err != nil {
					return nil, errors.New("the original host terminal boot receipt is unavailable; legacy sessions cannot be retroactively verified")
				}
				if receipt.Root != request.Root {
					return nil, errors.New("host terminal recovery changed the original root")
				}
				// A graceful worker stop can verify cleanup before the broker sees it.
				if !receipt.CleanupVerified || receipt.Uncertain {
					if err := process.StopScope(ctx, receipt.Scope); err != nil {
						return nil, err
					}
				}

				receipt.Reconciled = true
				if err := saveHostTerminalReceipt(root, receipt); err != nil {
					return nil, err
				}
				proof := contract.HostTerminalRecovery{Schema: contract.HostTerminalReceiptVersion, Reference: payload.Reference, Root: receipt.Root, Reconciled: true}
				if observe, ok := ctx.Value(hostRecoveryObserverKey{}).(func(contract.HostTerminalRecovery) error); ok {
					if err := observe(proof); err != nil {
						return nil, err
					}
				}
				return proof, nil
			}}, nil
		}, authorize)
	if err != nil {
		return nil, err
	}
	return native.Invoke(ctx, invocation.Call{Contract: id, Target: target, Identity: identity}, payload)
}

func (broker *Broker) acceptHostTerminalCompletion(operation *contract.FeatureOperation, completion contract.FeatureCompletion) error {
	if completion.HostRecovery == nil {
		if operation.Request.Action == "terminal.host-recover" && completion.Result.Status == "success" {
			return errors.New("host terminal recovery completion is missing its typed proof")
		}
		return nil
	}
	proof := *completion.HostRecovery
	if completion.Result.Status != "success" || proof.Validate() != nil ||
		operation.HostRecovery != nil && *operation.HostRecovery != proof {
		return errors.New("host terminal recovery completion changed its proof")
	}
	if !completion.ReceiptOnly {
		var payload contract.HostTerminalRecovery
		if decodeFeature(completion.Result.Payload, &payload) != nil || payload != proof {
			return errors.New("host terminal recovery result does not match its metadata proof")
		}
	}
	if err := broker.completeHostTerminalRecovery(operation.Request, proof); err != nil {
		return err
	}
	operation.HostRecovery = &proof
	return nil
}
