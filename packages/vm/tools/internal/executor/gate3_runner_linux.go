//go:build linux

package executor

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"dockpipe.vm/tools/internal/controller"
	"dockpipe.vm/tools/internal/manifest"
	"dockpipe.vm/tools/internal/protocol"
	"dockpipe.vm/tools/internal/provisioning"
	"dockpipe.vm/tools/internal/recovery"
	"golang.org/x/sys/unix"
)

const gate3ConsoleLimit = int64(4 * 1024 * 1024)

type Gate3RunnerConfig struct {
	Plan          Gate3Plan
	Execution     Contract
	Contract      provisioning.Contract
	Manifest      manifest.Manifest
	Keys          provisioning.KeyMaterial
	Authorization Gate3Authorization
	Token         string
	Now           func() time.Time
}

type gate3TrialState struct {
	Nonce            string
	CheckpointBootID string
}

type Gate3LinuxRunner struct {
	config   Gate3RunnerConfig
	mu       sync.Mutex
	command  *exec.Cmd
	exit     <-chan error
	process  controller.ProcessIdentity
	agent    net.Conn
	boot     protocol.SignedFrame
	sequence uint64
	nonces   map[string]bool
	trials   map[string]gate3TrialState
	console  *gate3ConsoleSession
}

type gate3ConsoleSession struct {
	listener net.Listener
	conn     net.Conn
	file     *os.File
	done     chan error
}

func NewGate3LinuxRunner(config Gate3RunnerConfig) (*Gate3LinuxRunner, error) {
	if config.Now == nil {
		config.Now = time.Now
	}
	if err := config.Plan.Validate(config.Execution); err != nil {
		return nil, err
	}
	if err := config.Authorization.Validate(config.Plan, config.Execution, config.Now()); err != nil {
		return nil, err
	}
	if hashBytes([]byte(config.Token)) != config.Authorization.TokenSHA256 {
		return nil, fmt.Errorf("Gate 3 destructive token does not match the authorization")
	}
	if len(config.Keys.ControllerPrivate) != ed25519.PrivateKeySize || len(config.Keys.ControllerPublic) != ed25519.PublicKeySize || len(config.Keys.GuestPrivate) != ed25519.PrivateKeySize || len(config.Keys.GuestPublic) != ed25519.PublicKeySize || !bytes.Equal(config.Keys.ControllerPrivate[32:], config.Keys.ControllerPublic) || !bytes.Equal(config.Keys.GuestPrivate[32:], config.Keys.GuestPublic) || hashBytes(config.Keys.ControllerPublic) != config.Contract.Artifacts.ControllerPublicKeySHA256 || hashBytes(config.Keys.GuestPublic) != config.Contract.Artifacts.GuestPublicKeySHA256 {
		return nil, fmt.Errorf("Gate 3 controller and guest key material is invalid")
	}
	if _, err := os.Lstat(config.Plan.EvidenceRoot); !os.IsNotExist(err) {
		return nil, fmt.Errorf("Gate 3 evidence root is not fresh")
	}
	return &Gate3LinuxRunner{config: config, trials: map[string]gate3TrialState{}}, nil
}

func (r *Gate3LinuxRunner) Boot(ctx context.Context, bootNumber int) (result error) {
	if bootNumber < 1 || bootNumber > len(Gate3Trials())+1 {
		return fmt.Errorf("Gate 3 boot number is outside the closed cohort")
	}
	if bootNumber == 1 {
		if err := os.Mkdir(r.config.Plan.EvidenceRoot, 0o700); err != nil {
			return fmt.Errorf("exclusively create Gate 3 evidence root: %w", err)
		}
		if err := syncDirectory(filepath.Dir(r.config.Plan.EvidenceRoot)); err != nil {
			return err
		}
	}
	for _, socket := range []string{r.config.Plan.QMP, r.config.Plan.AgentSocket, r.config.Plan.ConsoleSocket} {
		if err := removeExactStaleSocket(socket); err != nil {
			return err
		}
	}
	console, err := startGate3Console(r.config.Plan.ConsoleSocket, filepath.Join(r.config.Plan.EvidenceRoot, fmt.Sprintf("boot-%02d-console.log", bootNumber)))
	if err != nil {
		return err
	}
	defer func() {
		if result != nil {
			result = errors.Join(result, console.finish())
		}
	}()
	logPath := filepath.Join(r.config.Plan.EvidenceRoot, fmt.Sprintf("boot-%02d-qemu.log", bootNumber))
	log, err := os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	command := exec.CommandContext(context.WithoutCancel(ctx), r.config.Plan.Launch.Binary, r.config.Plan.Launch.Args...)
	command.Env = []string{}
	command.Dir = filepath.Dir(r.config.Plan.Launch.Binary)
	command.Stdout, command.Stderr = log, log
	if err := command.Start(); err != nil {
		log.Close()
		return err
	}
	exit := make(chan error, 1)
	go func() { exit <- command.Wait(); _ = log.Sync(); _ = log.Close() }()
	identity, err := processIdentity(command.Process.Pid, r.config.Plan.Launch, filepath.Dir(r.config.Execution.OSClone.Target))
	if err != nil {
		return err
	}
	processJSON, _ := json.Marshal(identity)
	if err := durableExclusive(filepath.Join(r.config.Plan.EvidenceRoot, fmt.Sprintf("boot-%02d-process.json", bootNumber)), processJSON, 0o600); err != nil {
		return err
	}
	if err := console.accept(ctx); err != nil {
		return err
	}
	for !readyPaths(r.config.Plan.QMP, r.config.Plan.AgentSocket) {
		select {
		case err := <-exit:
			return fmt.Errorf("QEMU exited before Gate 3 sockets were ready: %w", err)
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
	agent, err := (&net.Dialer{}).DialContext(ctx, "unix", r.config.Plan.AgentSocket)
	if err != nil {
		return err
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = agent.SetDeadline(deadline)
	}
	bootstrapJSON, err := protocol.ReadFramed(agent)
	if err != nil {
		agent.Close()
		return err
	}
	expected := protocol.Context{MachineUUID: r.config.Manifest.MachineUUID, DiskSerial: r.config.Manifest.DataDisk.Serial, RunID: r.config.Manifest.RunID, Scenario: r.config.Manifest.Scenario, DurabilityBoundary: r.config.Manifest.DurabilityBoundary}
	payload := protocol.IdentityBootstrapPayload{BootIDSource: r.config.Manifest.BootIDSource, ControllerPublicKeySHA256: r.config.Contract.Artifacts.ControllerPublicKeySHA256, GuestPublicKeySHA256: r.config.Contract.Artifacts.GuestPublicKeySHA256, ControllerBinarySHA256: r.config.Contract.Artifacts.ControllerBinarySHA256, GuestAgentBinarySHA256: r.config.Contract.Artifacts.GuestAgentBinarySHA256}
	bootstrap, err := protocol.VerifyIdentityBootstrap(bootstrapJSON, r.config.Keys.GuestPublic, r.config.Now(), expected, r.config.Contract.BootstrapNonce, payload)
	if err != nil {
		agent.Close()
		return err
	}
	if err := durableExclusive(filepath.Join(r.config.Plan.EvidenceRoot, fmt.Sprintf("boot-%02d-bootstrap.json", bootNumber)), bootstrapJSON, 0o600); err != nil {
		agent.Close()
		return err
	}
	r.mu.Lock()
	r.command, r.exit, r.process, r.agent, r.boot, r.sequence, r.console = command, exit, identity, agent, bootstrap, protocol.FirstRequestSequence, console
	r.nonces = map[string]bool{r.config.Contract.BootstrapNonce: true}
	r.mu.Unlock()
	for _, capability := range []string{"identity/v1", "health/v1", "launch-hash-pinned/v1"} {
		body := any(struct{}{})
		if capability == "launch-hash-pinned/v1" {
			body = map[string]string{"controller_binary_sha256": r.config.Contract.Artifacts.ControllerBinarySHA256, "guest_agent_binary_sha256": r.config.Contract.Artifacts.GuestAgentBinarySHA256}
		}
		response, err := r.exchange(ctx, capability, body, "gate3-verification", "")
		if err != nil {
			return err
		}
		frame, _ := protocol.Verify(response, r.config.Keys.GuestPublic, r.config.Now())
		if err := validateResult(frame, r.config.Contract.Artifacts.ControllerBinarySHA256, r.config.Contract.Artifacts.GuestAgentBinarySHA256); err != nil {
			return err
		}
	}
	return nil
}

func (r *Gate3LinuxRunner) Checkpoint(ctx context.Context, trial Gate3Trial) error {
	nonce, err := freshNonce(r.nonces)
	if err != nil {
		return err
	}
	body := map[string]any{"cohort_id": r.config.Plan.CohortID, "trial_id": trial.TrialID, "attempt": trial.Attempt, "boundary": trial.Boundary, "ticket_nonce": nonce, "harness_sha256": r.config.Plan.HarnessSHA256}
	responseDelivery := filepath.Join(r.config.Plan.EvidenceRoot, trial.TrialID+"-checkpoint-response-delivered.json")
	response, err := r.exchange(ctx, "checkpoint/v1", body, "gate3-checkpoint", responseDelivery)
	if err != nil {
		return err
	}
	var result struct {
		TicketSHA256     string          `json:"ticket_sha256"`
		CheckpointBootID string          `json:"checkpoint_boot_id"`
		HarnessSHA256    string          `json:"harness_sha256"`
		Evidence         json.RawMessage `json:"evidence"`
	}
	if err := decodeGate3Payload(response, r.config.Keys.GuestPublic, r.config.Now(), &result); err != nil || result.CheckpointBootID != r.boot.Context.BootID || result.HarnessSHA256 != r.config.Plan.HarnessSHA256 || !isSHA256(result.TicketSHA256) || len(result.Evidence) == 0 {
		return fmt.Errorf("Gate 3 checkpoint result changed: %v", err)
	}
	if err := validateGate3HarnessEvidence(result.Evidence, r.config.Plan.CohortID, trial, false); err != nil {
		return err
	}
	identity := recovery.Identity{MachineUUID: r.config.Plan.MachineUUID, DiskSerial: r.config.Plan.DiskSerial, BootID: r.boot.Context.BootID, RunID: r.config.Plan.RunID, CohortID: r.config.Plan.CohortID, TrialID: trial.TrialID, Scenario: r.config.Plan.Scenario, DurabilityBoundary: trial.Boundary, Nonce: nonce, HarnessSHA256: r.config.Plan.HarnessSHA256}
	ticketJSON, _ := json.Marshal(recovery.Ticket{Identity: identity, Status: "pending"})
	if hashBytes(ticketJSON) != result.TicketSHA256 {
		return fmt.Errorf("Gate 3 checkpoint ticket hash mismatch")
	}
	if err := durableExclusive(filepath.Join(r.config.Plan.EvidenceRoot, trial.TrialID+"-checkpoint.json"), response, 0o600); err != nil {
		return err
	}
	r.trials[trial.TrialID] = gate3TrialState{Nonce: nonce, CheckpointBootID: r.boot.Context.BootID}
	return nil
}

func (r *Gate3LinuxRunner) Recover(ctx context.Context, trial Gate3Trial) error {
	state, ok := r.trials[trial.TrialID]
	if !ok || state.CheckpointBootID == r.boot.Context.BootID {
		return fmt.Errorf("Gate 3 recovery has no distinct checkpoint boot")
	}
	body := map[string]any{"cohort_id": r.config.Plan.CohortID, "trial_id": trial.TrialID, "attempt": trial.Attempt, "boundary": trial.Boundary, "ticket_nonce": state.Nonce, "checkpoint_boot_id": state.CheckpointBootID, "harness_sha256": r.config.Plan.HarnessSHA256}
	response, err := r.exchange(ctx, "recovery/v1", body, "gate3-recovery", "")
	if err != nil {
		return err
	}
	var result struct {
		TicketSHA256     string          `json:"ticket_sha256"`
		CheckpointBootID string          `json:"checkpoint_boot_id"`
		RecoveryBootID   string          `json:"recovery_boot_id"`
		HarnessSHA256    string          `json:"harness_sha256"`
		Outcome          string          `json:"outcome"`
		EvidenceSHA256   string          `json:"evidence_sha256"`
		Evidence         json.RawMessage `json:"evidence"`
	}
	if err := decodeGate3Payload(response, r.config.Keys.GuestPublic, r.config.Now(), &result); err != nil || result.CheckpointBootID != state.CheckpointBootID || result.RecoveryBootID != r.boot.Context.BootID || result.HarnessSHA256 != r.config.Plan.HarnessSHA256 || hashBytes(result.Evidence) != result.EvidenceSHA256 {
		return fmt.Errorf("Gate 3 recovery result changed: %v", err)
	}
	wantOutcome := "old"
	if trial.Boundary == "after-commit-before-reload" || trial.Boundary == "after-validation-before-ack" {
		wantOutcome = "new"
	}
	if result.Outcome != wantOutcome {
		return fmt.Errorf("Gate 3 recovery outcome = %q, want %q", result.Outcome, wantOutcome)
	}
	if err := validateGate3HarnessEvidence(result.Evidence, r.config.Plan.CohortID, trial, true); err != nil {
		return err
	}
	identity := recovery.Identity{MachineUUID: r.config.Plan.MachineUUID, DiskSerial: r.config.Plan.DiskSerial, BootID: state.CheckpointBootID, RunID: r.config.Plan.RunID, CohortID: r.config.Plan.CohortID, TrialID: trial.TrialID, Scenario: r.config.Plan.Scenario, DurabilityBoundary: trial.Boundary, Nonce: state.Nonce, HarnessSHA256: r.config.Plan.HarnessSHA256}
	ticketJSON, _ := json.Marshal(recovery.Ticket{Identity: identity, Status: "pending"})
	if hashBytes(ticketJSON) != result.TicketSHA256 {
		return fmt.Errorf("Gate 3 recovery ticket hash mismatch")
	}
	return durableExclusive(filepath.Join(r.config.Plan.EvidenceRoot, trial.TrialID+"-recovery.json"), response, 0o600)
}

func (r *Gate3LinuxRunner) HardPower(ctx context.Context, trial Gate3Trial) error {
	r.mu.Lock()
	agent, exit, expected := r.agent, r.exit, r.process
	r.agent = nil
	r.mu.Unlock()
	if agent != nil {
		_ = agent.Close()
	}
	observed, err := processIdentity(expected.PID, r.config.Plan.Launch, expected.InstanceRoot)
	if err != nil {
		return err
	}
	plan, err := controller.PlanHardPower(controller.DestructiveAuthorization{Qualification: true, Disposable: true, MachineUUID: r.config.Plan.MachineUUID, DiskSerial: r.config.Plan.DiskSerial, RunID: r.config.Plan.RunID, CheckpointAuthenticated: true, ExpectedProcess: expected, ObservedProcess: observed, ExpectedTokenSHA256: r.config.Authorization.TokenSHA256, PresentedToken: r.config.Token})
	if err != nil || plan.Execute || plan.Mechanism != "pidfd_send_signal" || plan.Signal != "SIGKILL" {
		return fmt.Errorf("Gate 3 hard-power plan rejected: %v", err)
	}
	pidfd, err := unix.PidfdOpen(expected.PID, 0)
	if err != nil {
		return fmt.Errorf("pidfd_open exact QEMU: %w", err)
	}
	defer unix.Close(pidfd)
	current, err := processIdentity(expected.PID, r.config.Plan.Launch, expected.InstanceRoot)
	if err != nil || current != expected {
		return fmt.Errorf("QEMU identity changed after pidfd_open: %v", err)
	}
	if err := unix.PidfdSendSignal(pidfd, unix.SIGKILL, nil, 0); err != nil {
		return fmt.Errorf("pidfd_send_signal exact QEMU: %w", err)
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-exit:
	}
	if err := r.finishConsole(); err != nil {
		return err
	}
	evidence, _ := json.Marshal(map[string]any{"schema": "dockpipe.vm.gate3-hard-power.v1", "trial_id": trial.TrialID, "pid": expected.PID, "start_ticks": expected.StartTicks, "mechanism": "pidfd_send_signal", "signal": "SIGKILL", "exit_observed": true})
	return durableExclusive(filepath.Join(r.config.Plan.EvidenceRoot, trial.TrialID+"-hard-power.json"), evidence, 0o600)
}

func (r *Gate3LinuxRunner) ControlledShutdown(ctx context.Context) error {
	if r.agent != nil {
		_ = r.agent.Close()
		r.agent = nil
	}
	observed, err := processIdentity(r.process.PID, r.config.Plan.Launch, r.process.InstanceRoot)
	if err != nil || observed != r.process {
		return fmt.Errorf("Gate 3 final QEMU identity changed: %v", err)
	}
	conn, err := (&net.Dialer{}).DialContext(ctx, "unix", r.config.Plan.QMP)
	if err != nil {
		return err
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	client := controller.QMPClient{Conn: conn}
	if err := client.Negotiate("gate3-capabilities"); err != nil {
		return err
	}
	if err := client.SystemPowerdown("gate3-powerdown"); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-r.exit:
		if err != nil {
			return fmt.Errorf("Gate 3 QEMU did not exit cleanly: %w", err)
		}
	}
	if err := r.finishConsole(); err != nil {
		return err
	}
	evidence, _ := json.Marshal(map[string]any{"schema": "dockpipe.vm.gate3-shutdown.v1", "pid": r.process.PID, "command": "system_powerdown", "clean_exit": true})
	return durableExclusive(filepath.Join(r.config.Plan.EvidenceRoot, "shutdown.json"), evidence, 0o600)
}

func (r *Gate3LinuxRunner) Preserve(context.Context) error {
	result := r.finishConsole()
	for _, root := range r.config.Execution.Preservation.Roots {
		if err := syncTree(root); err != nil && !os.IsNotExist(err) {
			result = errors.Join(result, err)
		}
	}
	return result
}

func (r *Gate3LinuxRunner) exchange(ctx context.Context, capability string, body any, phase, responseDeliveryPath string) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.agent == nil {
		return nil, fmt.Errorf("Gate 3 agent connection is unavailable")
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = r.agent.SetDeadline(deadline)
	}
	nonce, err := freshNonce(r.nonces)
	if err != nil {
		return nil, err
	}
	requestContext := r.boot.Context
	requestContext.Sequence = r.sequence
	requestContext.Nonce = nonce
	requestContext.Phase = phase
	r.sequence++
	now := r.config.Now()
	signed, err := protocol.Sign(protocol.RequestKind, capability, requestContext, body, now, now.Add(time.Minute), r.config.Keys.ControllerPrivate)
	if err != nil {
		return nil, err
	}
	if err := protocol.WriteFramed(r.agent, signed); err != nil {
		return nil, err
	}
	response, err := protocol.ReadFramed(r.agent)
	if err != nil {
		return nil, err
	}
	frame, err := protocol.Verify(response, r.config.Keys.GuestPublic, r.config.Now())
	if err != nil || frame.Kind != protocol.ResultKind || frame.Capability != capability || frame.Context != requestContext {
		return nil, fmt.Errorf("Gate 3 guest result context changed: %v", err)
	}
	if responseDeliveryPath != "" {
		if err := durableExclusive(responseDeliveryPath, response, 0o600); err != nil {
			return nil, fmt.Errorf("persist Gate 3 signed response delivery: %w", err)
		}
	}
	return response, nil
}

func decodeGate3Payload(response []byte, guestPublic ed25519.PublicKey, now time.Time, target any) error {
	frame, err := protocol.Verify(response, guestPublic, now)
	if err != nil {
		return err
	}
	return decodeExactPayload(frame.Payload, target)
}

func validateGate3HarnessEvidence(data []byte, cohortID string, trial Gate3Trial, recovery bool) error {
	var evidence struct {
		Schema               string `json:"schema"`
		CohortID             string `json:"cohort_id"`
		TrialID              string `json:"trial_id"`
		Boundary             string `json:"boundary"`
		Attempt              int    `json:"attempt"`
		Root                 string `json:"root"`
		RootIdentity         string `json:"root_identity"`
		Database             string `json:"database"`
		ExpectedRevision     int64  `json:"expected_revision"`
		ObservedRevision     int64  `json:"observed_revision,omitempty"`
		ObservedDigest       string `json:"observed_digest,omitempty"`
		PreMetadataSHA256    string `json:"pre_metadata_sha256"`
		PostMetadataSHA256   string `json:"post_metadata_sha256,omitempty"`
		CompileOptionsSHA256 string `json:"compile_options_sha256"`
		SQLiteVersion        string `json:"sqlite_version"`
		SQLiteSourceID       string `json:"sqlite_source_id"`
		VFS                  string `json:"vfs"`
		QuickCheck           string `json:"quick_check,omitempty"`
		Retries              int    `json:"retries"`
		Replays              int    `json:"replays"`
		Repairs              int    `json:"repairs"`
		Fallbacks            int    `json:"fallbacks"`
	}
	canonical, err := protocol.Canonicalize(data)
	if err != nil || !bytes.Equal(canonical, data) {
		return fmt.Errorf("Gate 3 harness evidence is not canonical: %v", err)
	}
	if err := decodeExactPayload(data, &evidence); err != nil {
		return fmt.Errorf("decode Gate 3 harness evidence: %w", err)
	}
	wantRevision := int64(1)
	if trial.Boundary == "after-commit-before-reload" || trial.Boundary == "after-validation-before-ack" {
		wantRevision = 2
	}
	wantSchema := "dockpipe.sqlite-vm-checkpoint-evidence.v1"
	if recovery {
		wantSchema = "dockpipe.sqlite-vm-recovery-evidence.v1"
	}
	wantRoot := filepath.Join("/var/lib/dockpipe-qualification/cohorts", cohortID, trial.Boundary, fmt.Sprintf("attempt-%d", trial.Attempt))
	if evidence.Schema != wantSchema || evidence.CohortID != cohortID || evidence.TrialID != trial.TrialID || evidence.Boundary != trial.Boundary || evidence.Attempt != trial.Attempt || evidence.Root != wantRoot || evidence.RootIdentity == "" || evidence.Database != filepath.Join(wantRoot, "sqlite", "main", "aggregate.sqlite") || evidence.ExpectedRevision != wantRevision || !isSHA256(evidence.PreMetadataSHA256) || !isSHA256(evidence.CompileOptionsSHA256) || evidence.SQLiteVersion != "3.53.3" || evidence.SQLiteSourceID != "2026-06-26 20:14:12 d4c0e51e4aeb96955b99185ab9cde75c339e2c29c3f3f12428d364a10d782c62" || evidence.VFS != "unix" || evidence.Retries != 0 || evidence.Replays != 0 || evidence.Repairs != 0 || evidence.Fallbacks != 0 {
		return fmt.Errorf("Gate 3 harness evidence identity or invariant changed")
	}
	if recovery && (evidence.ObservedRevision != wantRevision || !isSHA256(evidence.ObservedDigest) || !isSHA256(evidence.PostMetadataSHA256) || evidence.QuickCheck != "ok") {
		return fmt.Errorf("Gate 3 recovery evidence is incomplete")
	}
	if !recovery && (evidence.ObservedRevision != 0 || evidence.ObservedDigest != "" || evidence.PostMetadataSHA256 != "" || evidence.QuickCheck != "") {
		return fmt.Errorf("Gate 3 checkpoint evidence contains a recovery result")
	}
	return nil
}

func (r *Gate3LinuxRunner) finishConsole() error {
	r.mu.Lock()
	console := r.console
	r.console = nil
	r.mu.Unlock()
	if console == nil {
		return nil
	}
	return console.finish()
}

func startGate3Console(socketPath, evidencePath string) (*gate3ConsoleSession, error) {
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(socketPath, 0o600); err != nil {
		listener.Close()
		return nil, err
	}
	file, err := os.OpenFile(evidencePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		listener.Close()
		return nil, err
	}
	return &gate3ConsoleSession{listener: listener, file: file, done: make(chan error, 1)}, nil
}

func (s *gate3ConsoleSession) accept(ctx context.Context) error {
	type accepted struct {
		conn net.Conn
		err  error
	}
	result := make(chan accepted, 1)
	go func() { conn, err := s.listener.Accept(); result <- accepted{conn, err} }()
	select {
	case <-ctx.Done():
		_ = s.listener.Close()
		return ctx.Err()
	case got := <-result:
		_ = s.listener.Close()
		s.listener = nil
		if got.err != nil {
			return got.err
		}
		s.conn = got.conn
		go func() {
			written, err := io.Copy(s.file, io.LimitReader(got.conn, gate3ConsoleLimit+1))
			if written > gate3ConsoleLimit {
				err = fmt.Errorf("Gate 3 console exceeded 4 MiB")
			}
			s.done <- err
		}()
		return nil
	}
}

func (s *gate3ConsoleSession) finish() error {
	if s == nil {
		return nil
	}
	if s.listener != nil {
		_ = s.listener.Close()
	}
	if s.conn != nil {
		_ = s.conn.Close()
	}
	var result error
	if s.done != nil && s.conn != nil {
		result = <-s.done
		if errors.Is(result, net.ErrClosed) {
			result = nil
		}
	}
	if err := s.file.Sync(); result == nil {
		result = err
	}
	if err := s.file.Close(); result == nil {
		result = err
	}
	if err := syncDirectory(filepath.Dir(s.file.Name())); result == nil {
		result = err
	}
	return result
}

func removeExactStaleSocket(path string) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil || info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("Gate 3 stale path is not the exact socket %s", path)
	}
	return os.Remove(path)
}

var _ Gate3Runner = (*Gate3LinuxRunner)(nil)
