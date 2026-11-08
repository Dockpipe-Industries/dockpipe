package remote

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"dockpipe/src/lib/domain/invocation"
	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure/osauth"
	"dockpipe/src/lib/infrastructure/remoteidentity"
	"dockpipe/src/lib/infrastructure/terminalsession"
)

const terminalPollInterval = 200 * time.Millisecond

type terminalWorkerRecord struct {
	mutex       sync.Mutex
	open        contract.TerminalOpen
	operator    string
	session     *terminalsession.Session
	cancel      context.CancelFunc
	leaseUntil  time.Time
	ackSequence uint64
	ackDigest   string
	inputBusy   bool
	outputAck   uint64
	state       string
	reason      string
	columns     int
	rows        int
}

type terminalWorkerService struct {
	root            string
	client          *Client
	config          contract.WorkerConfig
	instance        string
	epoch           string
	policy          string
	catalog         contract.FeatureCatalog
	manager         *terminalsession.Manager
	records         map[string]*terminalWorkerRecord
	environment     []string
	suspendClock    osauth.SuspendClock
	sleepGeneration uint64
}

// runTerminalWorker is independent of durable feature reconciliation and workflow
// jobs. It never creates a journal and never restores a session after restart.
func runTerminalWorker(ctx context.Context, root string, config contract.WorkerConfig, client *Client, instance string) error {
	if !config.Features || client == nil || client.Identity == nil || client.BrokerKey == "" || client.Identity.PublicKey != config.MachineKey || !validMachineNonce(instance) {
		return errors.New("terminal worker requires opt-in and paired identities")
	}
	unlock, err := Lock(filepath.Join(root, "terminal.lock"))
	if err != nil {
		return err
	}
	defer unlock()
	service := &terminalWorkerService{root: root, client: client, config: config, instance: instance,
		records: map[string]*terminalWorkerRecord{}, environment: os.Environ()}
	if terminalSupported() {
		platform, err := osauth.New()
		if err != nil {
			return err
		}
		clock, supported := platform.(osauth.SuspendClock)
		if !supported {
			return errors.New("terminal worker suspend detection unavailable")
		}
		generation, err := clock.SuspendGeneration()
		if err != nil {
			return err
		}
		service.suspendClock, service.sleepGeneration = clock, generation
	}
	defer service.shutdown()
	for ctx.Err() == nil {
		if err := service.checkSuspend(); err != nil {
			return err
		}
		permissions, err := loadFeaturePermissions(root)
		if err != nil {
			if err := service.cancelSessions(); err != nil {
				return err
			}
			if !pause(ctx, time.Second) {
				break
			}
			continue
		}
		policy := permissionFingerprint(permissions)
		if policy != service.policy {
			if err := service.shutdown(); err != nil {
				return err
			}
			if service.catalog.Schema != "" {
				// Try to report known cleanup under its original target before
				// changing generations. Failure leaves honest uncertainty.
				_, _ = service.exchange(ctx)
			}
			if err := service.configure(permissions, policy); err != nil {
				return err
			}
		}
		started := time.Now()
		response, err := service.exchange(ctx)
		if err != nil {
			// A connection/authentication failure stops every owned PTY now;
			// worker heartbeats never manufacture an operator lease.
			if err := service.cancelSessions(); err != nil {
				return err
			}
			if !pause(ctx, terminalPollInterval) {
				break
			}
			continue
		}
		if response.Schema != contract.TerminalVersion || !validMachineNonce(response.Epoch) || len(response.Directives) > contract.MaxTerminalSessions || len(response.Forget) > contract.MaxTerminalSessions {
			return errors.New("invalid terminal exchange response")
		}
		if response.Epoch != service.epoch {
			if err := service.shutdown(); err != nil {
				return err
			}
			service.records = map[string]*terminalWorkerRecord{}
			service.epoch = response.Epoch
			if err := service.configure(permissions, policy); err != nil {
				return err
			}
			continue
		}
		for _, reference := range response.Forget {
			if err := service.forget(ctx, reference); err != nil {
				return err
			}
		}
		for _, directive := range response.Directives {
			if err := service.apply(ctx, directive, started); err != nil {
				return err
			}
		}
		if !pause(ctx, terminalPollInterval) {
			break
		}
	}
	return ctx.Err()
}

func (service *terminalWorkerService) checkSuspend() error {
	if service.suspendClock == nil {
		return nil
	}
	generation, err := service.suspendClock.SuspendGeneration()
	if err != nil {
		return err
	}
	if generation != service.sleepGeneration {
		if err := service.cancelSessions(); err != nil {
			return err
		}
		service.sleepGeneration = generation
	}
	return nil
}

func (service *terminalWorkerService) configure(permissions FeaturePermissions, policy string) error {
	service.policy = policy
	target := contract.FeatureTarget{Node: service.config.Node, Fingerprint: remoteidentity.Fingerprint(service.config.MachineKey), Generation: Hash(service.instance + policy)}
	service.catalog = permissions.catalog(target)
	service.records = map[string]*terminalWorkerRecord{}
	service.manager = nil
	if permissions.TerminalProfile == nil || !terminalSupported() {
		return nil
	}
	var directory string
	for _, root := range permissions.Roots {
		if root.Terminal {
			directory = root.Path
		}
	}
	profile := terminalsession.Profile{ID: "owner-terminal", Executable: permissions.TerminalProfile.Executable,
		Args: permissions.TerminalProfile.Args, Env: service.environment, Dir: directory}
	authority := terminalsession.Authority{Target: terminalInvocationTarget(target), Identity: invocation.Identity{Issuer: "dockpipe.worker", Subject: target.Generation},
		Authorize: func(_ context.Context, binding invocation.Binding) error {
			if binding.Contract.Action != "release" {
				return errors.New("terminal session requires its own operator admission")
			}
			return nil
		}}
	manager, err := terminalsession.New(profile, authority)
	service.manager = manager
	return err
}

func terminalInvocationTarget(target contract.FeatureTarget) invocation.Target {
	return invocation.Target{Kind: invocation.Remote, ID: target.Node, Fingerprint: target.Fingerprint, Generation: target.Generation}
}

func (service *terminalWorkerService) cancelSessions() error {
	for _, record := range service.records {
		if record.cancel != nil {
			record.cancel()
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for _, record := range service.records {
		if record.session != nil {
			result, err := record.session.Wait(ctx)
			if err != nil || result.CleanupError != nil {
				return errors.New("terminal cleanup could not be verified")
			}
		}
	}
	return nil
}

func (service *terminalWorkerService) shutdown() error {
	if err := service.cancelSessions(); err != nil {
		return err
	}
	if service.manager != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		return service.manager.Shutdown(ctx)
	}
	return nil
}

func (service *terminalWorkerService) exchange(ctx context.Context) (contract.TerminalExchangeResult, error) {
	reports := make([]contract.TerminalReport, 0, len(service.records))
	for _, record := range service.records {
		record.mutex.Lock()
		report := contract.TerminalReport{Reference: record.open.Reference(), State: record.state, AckSequence: record.ackSequence, AckDigest: record.ackDigest,
			OutputStart: record.outputAck, Reason: record.reason}
		record.mutex.Unlock()
		if record.session != nil {
			if output, err := record.session.Read(ctx, report.OutputStart, contract.MaxTerminalChunk); err == nil {
				report.Data, report.Truncated = output.Data, output.Truncated
				report.OutputStart = output.Cursor - uint64(len(output.Data))
			}
			select {
			case <-record.session.Done():
				result, _ := record.session.Wait(ctx)
				report.State, report.Reason = "closed", "exited"
				if result.CleanupError != nil {
					report.State, report.Reason = "unknown", "cleanup-unverified"
				}
				if result.Cause != nil && report.State == "closed" {
					report.Reason = "closed"
				}
				if report.State == "closed" && errors.Is(result.Cause, terminalsession.ErrInputUncertain) {
					report.Reason = "input-uncertain"
				}
				if report.State == "closed" && errors.Is(result.Cause, terminalsession.ErrLeaseExpired) {
					report.Reason = "authority-lost"
				}
				record.mutex.Lock()
				if record.inputBusy {
					report.State, report.Reason = record.state, record.reason
				} else {
					if report.State == "closed" && record.reason == "input-uncertain" {
						report.Reason = record.reason
					}
					report.AckSequence, report.AckDigest = record.ackSequence, record.ackDigest
					record.state, record.reason = report.State, report.Reason
				}
				record.mutex.Unlock()
			default:
			}
		}
		reports = append(reports, report)
	}
	request := contract.TerminalExchange{Schema: contract.TerminalVersion, Epoch: service.epoch, Catalog: service.catalog, Reports: reports}
	var response contract.TerminalExchangeResult
	callContext, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	err := service.client.Call(callContext, "/v1/terminal-worker", request, &response)
	return response, err
}

func (service *terminalWorkerService) forget(ctx context.Context, reference contract.TerminalReference) error {
	if reference.Validate() != nil || reference.Epoch != service.epoch || reference.Target != service.catalog.Target {
		return errors.New("terminal forget changed identity")
	}
	record := service.records[reference.ID]
	if record == nil {
		return nil
	}
	if record.cancel != nil {
		record.cancel()
	}
	if record.session != nil {
		wait, cancel := context.WithTimeout(ctx, 2*time.Second)
		result, err := record.session.Wait(wait)
		cancel()
		if err != nil || result.CleanupError != nil {
			return errors.New("terminal cleanup remains uncertain")
		}
		if err := service.manager.Release(ctx, record.session.ID()); err != nil {
			return err
		}
	}
	delete(service.records, reference.ID)
	return nil
}

func (service *terminalWorkerService) apply(ctx context.Context, directive contract.TerminalDirective, started time.Time) error {
	request := directive.Open
	if request.Validate() != nil || request.Epoch != service.epoch || request.Target != service.catalog.Target || !validMachineNonce(directive.Operator) ||
		!terminalRoot(service.catalog, request.Root) || !contract.ValidTerminalSize(directive.Columns, directive.Rows) {
		return errors.New("terminal directive changed identity or permission")
	}
	if directive.Forget {
		return service.forget(ctx, request.Reference())
	}
	record := service.records[request.ID]
	if record != nil && (record.open != request || record.operator != directive.Operator) {
		return errors.New("terminal owner changed")
	}
	if record == nil {
		if len(service.records) >= contract.MaxTerminalSessions {
			return errors.New("terminal worker capacity reached")
		}
		record = &terminalWorkerRecord{open: request, operator: directive.Operator, state: "closed", reason: "closed", columns: request.Columns, rows: request.Rows}
		service.records[request.ID] = record
		if directive.Close {
			return nil
		}
		deadline, err := featureLeaseDeadline(started, directive.LeaseMillis)
		// Absolute open deadlines belong to the broker's clock. The worker
		// admits only the signed remaining duration measured from its own
		// monotonic request start, never a cross-machine wall-clock comparison.
		if err != nil || service.manager == nil {
			return errors.New("terminal launch lease expired")
		}
		record.leaseUntil = deadline
		lifetime, cancel := context.WithCancel(ctx)
		record.cancel = cancel
		identity := invocation.Identity{Issuer: "dockpipe.os-session", Subject: directive.Operator}
		policy, target, root := service.policy, request.Target, service.root
		clock, sleepGeneration := service.suspendClock, service.sleepGeneration
		authorize := func(ctx context.Context, binding invocation.Binding) error {
			if binding.Target != terminalInvocationTarget(target) || binding.Identity != identity || binding.Contract.Capability != "dockpipe.terminal" || binding.Contract.Version != "1" {
				return errors.New("terminal admission changed")
			}
			record.mutex.Lock()
			live := time.Now().Before(record.leaseUntil)
			record.mutex.Unlock()
			if !live {
				return errors.New("terminal authority expired")
			}
			if clock != nil {
				generation, err := clock.SuspendGeneration()
				if err != nil || generation != sleepGeneration {
					return errors.New("terminal worker resumed from suspend")
				}
			}
			permissions, err := loadFeaturePermissions(root)
			if err != nil || permissionFingerprint(permissions) != policy {
				return errors.New("terminal owner permissions changed")
			}
			return ctx.Err()
		}
		session, err := service.manager.OpenAs(lifetime, terminalsession.Size{Columns: request.Columns, Rows: request.Rows}, time.Until(deadline), identity, authorize)
		if err != nil {
			cancel()
			record.reason = "launch-failed"
			return nil
		}
		record.session, record.state, record.reason = session, "running", ""
	}
	if record.session == nil {
		return nil
	}
	record.mutex.Lock()
	if directive.OutputAck < record.outputAck {
		record.mutex.Unlock()
		return errors.New("terminal output acknowledgement moved backwards")
	}
	record.outputAck = directive.OutputAck
	closed := record.state != "running"
	record.mutex.Unlock()
	if directive.Close {
		record.cancel()
		return nil
	}
	if closed {
		return nil
	}
	deadline, err := featureLeaseDeadline(started, directive.LeaseMillis)
	if err != nil {
		record.cancel()
		return nil
	}
	record.mutex.Lock()
	record.leaseUntil = deadline
	record.mutex.Unlock()
	if err := record.session.RenewLease(ctx, time.Until(deadline)); err != nil {
		record.cancel()
		return nil
	}
	if record.columns != directive.Columns || record.rows != directive.Rows {
		if err := record.session.Resize(ctx, terminalsession.Size{Columns: directive.Columns, Rows: directive.Rows}); err != nil {
			record.cancel()
			return nil
		}
		record.columns, record.rows = directive.Columns, directive.Rows
	}
	if directive.Input != nil {
		input := *directive.Input
		if input.Validate() != nil || input.Reference != request.Reference() {
			return errors.New("terminal input changed identity")
		}
		record.mutex.Lock()
		if record.inputBusy {
			record.mutex.Unlock()
			return nil
		}
		record.inputBusy = true
		record.mutex.Unlock()
		go func() {
			defer clear(input.Data)
			err := record.session.Input(ctx, input.Sequence, input.Data)
			record.mutex.Lock()
			defer record.mutex.Unlock()
			record.inputBusy = false
			if err != nil {
				record.reason = "input-uncertain"
				record.cancel()
				return
			}
			record.ackSequence, record.ackDigest = input.Sequence, contract.TerminalInputDigest(input.Data)
		}()
	}
	return nil
}
