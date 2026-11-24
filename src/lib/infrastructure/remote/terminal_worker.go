package remote

import (
	"context"
	"errors"
	"os"
	"reflect"
	"runtime"
	"sync"
	"time"

	"dockpipe/src/lib/domain/invocation"
	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/localterminal"
	"dockpipe/src/lib/infrastructure/osauth"
	"dockpipe/src/lib/infrastructure/process"
	"dockpipe/src/lib/infrastructure/remoteidentity"
	"dockpipe/src/lib/infrastructure/terminalsession"
)

const terminalPollInterval = 200 * time.Millisecond
const terminalInteractiveInterval = 20 * time.Millisecond
const terminalInteractiveBurst = 250 * time.Millisecond

// Allow the native Docker closure check, relay reap and terminal session reap
// to finish before treating cleanup as uncertain. Cancellation still happens
// immediately and the wait does not renew the operator's lease.
const terminalCleanupWait = 5 * time.Second

type terminalWorkerRecord struct {
	mutex       sync.Mutex
	open        contract.TerminalOpen
	operator    string
	session     *terminalsession.Session
	manager     *terminalsession.Manager
	scope       *process.Scope
	journal     *containerTerminalJournal
	hostJournal *hostTerminalJournal
	recovered   bool
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
	root             string
	client           *Client
	config           contract.WorkerConfig
	instance         string
	epoch            string
	policy           string
	processRecovery  string
	catalog          contract.FeatureCatalog
	hostManagers     map[string]*terminalsession.Manager
	records          map[string]*terminalWorkerRecord
	environment      []string
	suspendClock     osauth.SuspendClock
	sleepGeneration  uint64
	wake             chan struct{}
	interactiveUntil time.Time
}

// runTerminalWorker is independent of durable feature reconciliation and workflow
// jobs. It never creates a journal and never restores a session after restart.
func runTerminalWorker(ctx context.Context, root string, config contract.WorkerConfig, client *Client, instance string) error {
	if !config.Features || client == nil || client.Identity == nil || client.BrokerKey == "" || client.Identity.PublicKey != config.MachineKey || !validMachineNonce(instance) {
		return errors.New("terminal worker requires opt-in and paired identities")
	}
	unlock, err := lockTerminalWorker(root)
	if err != nil {
		return err
	}
	defer unlock()
	service := &terminalWorkerService{root: root, client: client, config: config, instance: instance,
		records: map[string]*terminalWorkerRecord{}, environment: os.Environ(), wake: make(chan struct{}, 1)}
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
		if !service.waitForWork(ctx) {
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
	service.processRecovery = permissions.ProcessRecovery
	if service.processRecovery == "" {
		service.processRecovery = process.RecoveryMode()
	}
	target := contract.FeatureTarget{Node: service.config.Node, Fingerprint: remoteidentity.Fingerprint(service.config.MachineKey), Generation: Hash(service.instance + policy)}
	service.catalog = permissions.catalog(target)
	service.records = map[string]*terminalWorkerRecord{}
	service.hostManagers = nil
	if permissions.TerminalProfile == nil || !terminalSupported() {
		return nil
	}

	authority := terminalsession.Authority{Target: terminalInvocationTarget(target), Identity: invocation.Identity{Issuer: "dockpipe.worker", Subject: target.Generation},
		Authorize: func(_ context.Context, binding invocation.Binding) error {
			if binding.Contract.Action != "release" {
				return errors.New("terminal session requires its own operator admission")
			}
			return nil
		}}
	service.hostManagers = make(map[string]*terminalsession.Manager)
	for _, root := range permissions.Roots {
		if !root.Terminal {
			continue
		}
		profile := terminalsession.Profile{ID: "owner-terminal", Executable: permissions.TerminalProfile.Executable,
			Args: permissions.TerminalProfile.Args, Env: service.environment, Dir: root.Path}
		manager, err := newHostTerminalManager(profile, authority)
		if err != nil {
			return err
		}
		service.hostManagers[root.ID] = manager
	}
	return nil
}

func newHostTerminalManager(profile terminalsession.Profile, authority terminalsession.Authority) (*terminalsession.Manager, error) {
	if runtime.GOOS == "windows" {
		profile.Route = "native"
		profile.EnvironmentMode = "complete"
		return terminalsession.NewOwnedProvider(profile, authority, localterminal.InstalledProvider)
	}
	return terminalsession.New(profile, authority)
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
	ctx, cancel := context.WithTimeout(context.Background(), terminalCleanupWait)
	defer cancel()
	for _, record := range service.records {
		if record.session != nil {
			result, err := record.session.Wait(ctx)
			if err != nil || result.CleanupError != nil && !record.recovered {
				if record.journal != nil {
					_ = record.journal.markUncertain()
				}
				if record.hostJournal != nil {
					_ = record.hostJournal.finish(false)
				}
				return errors.New("terminal cleanup could not be verified")
			}
			if record.hostJournal != nil && !record.recovered {
				if err := record.hostJournal.finish(true); err != nil {
					return err
				}
			}
			if record.scope != nil && !record.recovered {
				if service.processRecovery != "host-reboot" {
					if err := record.scope.Stop(ctx); err != nil {
						return err
					}
				}
				if err := record.journal.clientStopped(); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (service *terminalWorkerService) shutdown() error {
	if err := service.cancelSessions(); err != nil {
		return err
	}
	for _, record := range service.records {
		if record.open.Container != "" && record.manager != nil && !record.recovered {
			cleanup, stop := context.WithTimeout(context.Background(), 2*time.Second)
			err := record.manager.Shutdown(cleanup)
			stop()
			if err != nil {
				return err
			}
		}
	}
	if len(service.hostManagers) > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		var shutdownErrors []error
		for _, manager := range service.hostManagers {
			if err := manager.Shutdown(ctx); err != nil {
				shutdownErrors = append(shutdownErrors, err)
			}
		}
		return errors.Join(shutdownErrors...)
	}
	return nil
}

func (service *terminalWorkerService) exchange(ctx context.Context) (contract.TerminalExchangeResult, error) {
	// Drain only notifications already represented by the snapshot below. An
	// event arriving after this point stays queued for the next exchange.
	select {
	case <-service.wake:
	default:
	}
	for _, record := range service.records {
		if record.session != nil {
			select {
			case <-record.session.OutputReady():
			default:
			}
		}
	}
	reports := make([]contract.TerminalReport, 0, len(service.records))
	for _, record := range service.records {
		if record.hostJournal != nil {
			if receipt, err := loadHostTerminalReceipt(service.root, record.open.Reference()); err == nil && receipt.Reconciled {
				record.state, record.reason, record.recovered = "closed", "closed", true
			}
		}
		if record.journal != nil {
			receipt, err := loadContainerTerminalReceipt(service.root, record.open.Reference())
			if err == nil && receipt.Reconciled {
				record.state, record.reason, record.recovered = "closed", "closed", true
			}
		}
		record.mutex.Lock()
		report := contract.TerminalReport{Reference: record.open.Reference(), State: record.state, AckSequence: record.ackSequence, AckDigest: record.ackDigest,
			OutputStart: record.outputAck, Reason: record.reason}
		record.mutex.Unlock()
		if record.session != nil && !record.recovered {
			if output, err := record.session.Read(ctx, report.OutputStart, contract.MaxTerminalChunk); err == nil {
				report.Data, report.Truncated = output.Data, output.Truncated
				report.OutputStart = output.Cursor - uint64(len(output.Data))
			}
			select {
			case <-record.session.Done():
				result, _ := record.session.Wait(ctx)
				report.State, report.Reason = "closed", "exited"
				if record.hostJournal != nil {
					if err := record.hostJournal.finish(result.CleanupError == nil); err != nil {
						result.CleanupError = err
					}
				}
				if result.CleanupError == nil && record.scope != nil {
					if service.processRecovery == "host-reboot" {
						result.CleanupError = record.journal.clientStopped()
					} else {
						cleanup, stop := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
						result.CleanupError = record.scope.Stop(cleanup)
						stop()
						if result.CleanupError == nil {
							result.CleanupError = record.journal.clientStopped()
						}
					}
				}
				if result.CleanupError != nil {
					if record.journal != nil {
						if err := record.journal.markUncertain(); err != nil {
							return contract.TerminalExchangeResult{}, err
						}
					}
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
	if record.journal != nil {
		if receipt, err := loadContainerTerminalReceipt(service.root, reference); err == nil && receipt.Reconciled {
			record.recovered = true
		}
	}
	if record.hostJournal != nil {
		if receipt, err := loadHostTerminalReceipt(service.root, reference); err == nil && receipt.Reconciled {
			record.recovered = true
		}
	}
	if record.cancel != nil {
		record.cancel()
	}
	if record.session != nil {
		wait, cancel := context.WithTimeout(ctx, terminalCleanupWait)
		result, err := record.session.Wait(wait)
		cancel()
		if err != nil || result.CleanupError != nil && !record.recovered {
			return errors.New("terminal cleanup remains uncertain")
		}
		if !record.recovered {
			if record.open.Container == "" {
				err = record.manager.Release(ctx, record.session.ID())
			} else {
				// A container manager owns only this ExecID. The worker can
				// discard its verified closure after the operator lease ends.
				err = record.manager.Shutdown(ctx)
			}
			if err != nil {
				return err
			}
		}
	}
	if record.journal != nil {
		if err := removeContainerTerminalReceipt(service.root, reference); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if record.hostJournal != nil {
		if err := removeHostTerminalReceipt(service.root, reference); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	delete(service.records, reference.ID)
	return nil
}

func (service *terminalWorkerService) apply(ctx context.Context, directive contract.TerminalDirective, started time.Time) error {
	request := directive.Open
	if request.Validate() != nil || request.Epoch != service.epoch || request.Target != service.catalog.Target || !validMachineNonce(directive.Operator) ||
		!terminalSelectionAllowed(service.catalog, request) || !contract.ValidTerminalSize(directive.Columns, directive.Rows) {
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
		if err != nil {
			return errors.New("terminal launch lease expired")
		}
		hostManager := service.hostManagers[request.Root]
		if request.Container == "" && hostManager == nil {
			return errors.New("terminal root provider unavailable")
		}
		record.leaseUntil = deadline
		lifetime, cancel := context.WithCancel(ctx)
		record.cancel = cancel
		identity := invocation.Identity{Issuer: "dockpipe.os-session", Subject: directive.Operator}
		policy, target, root := service.policy, request.Target, service.root
		clock, sleepGeneration := service.suspendClock, service.sleepGeneration
		authorize := func(ctx context.Context, binding invocation.Binding) error {
			if binding.Target != terminalInvocationTarget(target) || binding.Identity != identity || binding.Contract.Version != "1" ||
				binding.Contract.Capability != "dockpipe.terminal" && !(request.Container != "" && binding.Contract.Capability == "dockpipe.containers" && binding.Contract.Action == "exec") {
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
			if request.Container == "" && binding.Contract.Action == "open" {
				if err := service.containerCapacity(request.ID); err != nil {
					return err
				}
			}
			permissions, err := loadFeaturePermissions(root)
			if err != nil || permissionFingerprint(permissions) != policy {
				return errors.New("terminal owner permissions changed")
			}
			return ctx.Err()
		}
		var session *terminalsession.Session
		if request.Container == "" {
			record.manager = hostManager
			if err = service.containerCapacity(request.ID); err == nil {
				record.hostJournal, err = prepareHostTerminalJournal(service.root, request.Reference(), request.Root)
			}
			if err == nil {
				err = record.hostJournal.phase("start-requested")
			}
			if err == nil {
				session, err = hostManager.OpenAs(lifetime, terminalsession.Size{Columns: request.Columns, Rows: request.Rows}, time.Until(deadline), identity, authorize)
			}
			if err == nil {
				err = record.hostJournal.phase("running")
			}
		} else {
			if err = service.containerCapacity(request.ID); err == nil {
				record.journal = &containerTerminalJournal{root: service.root, value: containerTerminalReceipt{
					Schema: containerTerminalReceiptSchema, Reference: request.Reference()}}
				lifetime, record.scope = process.WithScopeMode(lifetime, request.ID, service.processRecovery, record.journal.observeScope)
				var terminal infrastructure.DockerTerminal
				terminal, err = infrastructure.OpenContainerTerminal(lifetime,
					infrastructure.ContainerExecRequest{Container: request.Container, Shell: request.Shell},
					service.environment, terminalInvocationTarget(target), identity, authorize,
					terminalsession.Size{Columns: request.Columns, Rows: request.Rows}, time.Until(deadline), record.journal.observe)
				record.manager, session = terminal.Manager, terminal.Session
			}
		}
		if err != nil {
			cancel()
			record.reason = "launch-failed"
			if record.hostJournal != nil {
				uncertain := session != nil || errors.Is(err, terminalsession.ErrCleanupIncomplete)
				if uncertain {
					record.session, record.state, record.reason = session, "unknown", "cleanup-unverified"
				}
				if err := record.hostJournal.finish(!uncertain); err != nil {
					return err
				}
			}
			if record.journal != nil {
				if receipt, loadErr := loadContainerTerminalReceipt(service.root, request.Reference()); loadErr == nil {
					if receipt.Resource.Phase == "start-requested" || receipt.Resource.Phase == "running" {
						record.state, record.reason = "unknown", "cleanup-unverified"
						if err := record.journal.markUncertain(); err != nil {
							return err
						}
					} else if record.scope != nil {
						cleanup, stop := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
						if record.scope.Stop(cleanup) == nil {
							_ = removeContainerTerminalReceipt(service.root, request.Reference())
						}
						stop()
					}
				}
			}
			return nil
		}
		record.session, record.state, record.reason = session, "running", ""
		service.notifyWork()
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
		service.interactiveUntil = time.Now().Add(terminalInteractiveBurst)
		record.mutex.Unlock()
		go func() {
			defer clear(input.Data)
			err := record.session.Input(ctx, input.Sequence, input.Data)
			record.mutex.Lock()
			record.inputBusy = false
			if err != nil {
				record.reason = "input-uncertain"
				record.cancel()
			} else {
				record.ackSequence, record.ackDigest = input.Sequence, contract.TerminalInputDigest(input.Data)
			}
			record.mutex.Unlock()
			service.notifyWork()
		}()
	}
	return nil
}

func (service *terminalWorkerService) notifyWork() {
	select {
	case service.wake <- struct{}{}:
	default:
	}
}

func (service *terminalWorkerService) waitForWork(ctx context.Context) bool {
	interval := terminalPollInterval
	if time.Now().Before(service.interactiveUntil) {
		interval = terminalInteractiveInterval
	}
	timer := time.NewTimer(interval)
	defer timer.Stop()
	cases := []reflect.SelectCase{
		{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ctx.Done())},
		{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(timer.C)},
		{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(service.wake)},
	}
	// The existing eight-session contract bounds this wait to 19 channels.
	// No polling goroutine or retained output copy exists per session.
	for _, record := range service.records {
		record.mutex.Lock()
		running := record.state == "running"
		record.mutex.Unlock()
		if record.session != nil && running {
			cases = append(cases,
				reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(record.session.OutputReady())},
				reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(record.session.Done())})
		}
	}
	selected, _, _ := reflect.Select(cases)
	return selected != 0
}

func (service *terminalWorkerService) containerCapacity(current string) error {
	receipts, err := containerTerminalReceipts(service.root)
	if err != nil {
		return err
	}
	hosts, err := hostTerminalReceipts(service.root)
	if err != nil {
		return err
	}
	retained := len(service.records)
	for _, receipt := range receipts {
		if !receipt.Reconciled && receipt.Reference.ID != current && service.records[receipt.Reference.ID] == nil {
			retained++
		}
	}
	for _, receipt := range hosts {
		if !receipt.Reconciled && !receipt.CleanupVerified && receipt.Reference.ID != current && service.records[receipt.Reference.ID] == nil {
			retained++
		}
	}
	if retained > contract.MaxTerminalSessions {
		return errors.New("retained container terminal cleanup consumes the session budget")
	}
	return nil
}
