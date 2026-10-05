//go:build linux

package executor

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"dockpipe.vm/tools/internal/controller"
	"dockpipe.vm/tools/internal/manifest"
	"dockpipe.vm/tools/internal/protocol"
	"dockpipe.vm/tools/internal/provisioning"
)

type RunnerConfig struct {
	Contract  provisioning.Contract
	Manifest  manifest.Manifest
	Keys      provisioning.KeyMaterial
	Execution Contract
	Now       func() time.Time
	Dial      func(context.Context, string, string) (net.Conn, error)
}

type LinuxRunner struct {
	config      RunnerConfig
	mu          sync.Mutex
	command     *exec.Cmd
	exit        <-chan error
	process     controller.ProcessIdentity
	observation *observationSession
}

const guestVerificationFailureSchema = "dockpipe.vm.guest-verification-failure.v1"

type guestVerificationProgress struct {
	BootstrapVerified     bool
	CompletedCapabilities []string
}

type guestVerificationFailureEvidence struct {
	Schema                string   `json:"schema"`
	Operation             string   `json:"operation"`
	Reason                string   `json:"reason"`
	TimeoutSeconds        int      `json:"timeout_seconds"`
	BootstrapVerified     bool     `json:"bootstrap_verified"`
	CompletedCapabilities []string `json:"completed_capabilities"`
}

func NewLinuxRunner(config RunnerConfig) (*LinuxRunner, error) {
	if config.Now == nil {
		config.Now = time.Now
	}
	if config.Dial == nil {
		config.Dial = func(ctx context.Context, network, address string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, address)
		}
	}
	if err := config.Execution.Validate(); err != nil {
		return nil, err
	}
	if len(config.Keys.ControllerPrivate) != ed25519.PrivateKeySize || len(config.Keys.GuestPrivate) != ed25519.PrivateKeySize || len(config.Keys.ControllerPublic) != ed25519.PublicKeySize || len(config.Keys.GuestPublic) != ed25519.PublicKeySize || !bytes.Equal(config.Keys.ControllerPrivate[32:], config.Keys.ControllerPublic) || !bytes.Equal(config.Keys.GuestPrivate[32:], config.Keys.GuestPublic) || hashBytes(config.Keys.ControllerPublic) != config.Contract.Artifacts.ControllerPublicKeySHA256 || hashBytes(config.Keys.GuestPublic) != config.Contract.Artifacts.GuestPublicKeySHA256 {
		return nil, fmt.Errorf("production runner key material is not exact")
	}
	if config.Execution.RunID != config.Contract.RunID || config.Execution.CohortID != config.Contract.CohortID {
		return nil, fmt.Errorf("production runner identity mismatch")
	}
	return &LinuxRunner{config: config}, nil
}

func NewCleanupRunner() Runner { return &LinuxRunner{} }

func PrepareLiveRoots(c provisioning.Contract) (string, error) {
	instance := filepath.Join(c.Roots.Instances, c.RunID, c.CohortID)
	evidence := filepath.Join(c.Roots.Evidence, c.RunID, c.CohortID)
	config := filepath.Join(c.Roots.Config, "instances", c.RunID, c.CohortID)
	runtime := filepath.Join(c.Roots.Runtime, c.RunID, c.CohortID)
	for _, root := range []string{instance, evidence, config, runtime} {
		if err := os.MkdirAll(filepath.Dir(root), 0o700); err != nil {
			return "", err
		}
		if err := os.Mkdir(root, 0o700); err != nil {
			return "", fmt.Errorf("exclusively create live root %s: %w", root, err)
		}
		if err := syncDirectory(filepath.Dir(root)); err != nil {
			return "", err
		}
	}
	return filepath.Join(config, "identity"), nil
}

func (r *LinuxRunner) CreateOSClone(ctx context.Context, request OSCloneRequest) error {
	if request.Source != r.config.Contract.SourceImage.Path || request.SourceSHA256 != r.config.Contract.SourceImage.SHA256 {
		return fmt.Errorf("OS clone source differs from the authorized source image")
	}
	if err := provisioning.ValidateSourceImage(r.config.Contract.SourceImage, provisioning.OSImageInspector{}); err != nil {
		return err
	}
	if err := provisioning.ValidatePinnedBinary(request.Command.Binary, request.Command.BinarySHA256); err != nil {
		return err
	}
	if _, err := os.Lstat(request.Target); !os.IsNotExist(err) {
		return fmt.Errorf("private OS clone target is not fresh")
	}
	if err := runExact(ctx, request.Command, filepath.Dir(request.Target), r.evidencePath("qemu-img.log")); err != nil {
		return err
	}
	info, err := os.Lstat(request.Target)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("qemu-img did not create the exact private clone")
	}
	if err := os.Chmod(request.Target, 0o600); err != nil {
		return err
	}
	return syncFileAndDir(request.Target)
}

func (r *LinuxRunner) CreateSparseRawDisk(_ context.Context, request SparseRawDiskRequest) error {
	f, err := os.OpenFile(request.Target, os.O_RDWR|os.O_CREATE|os.O_EXCL, os.FileMode(request.Mode))
	if err != nil {
		return err
	}
	if err = f.Truncate(request.Bytes); err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(request.Target))
}

func (r *LinuxRunner) CreateNoCloudSeed(_ context.Context, request NoCloudSeedRequest) error {
	tree := filepath.Join(filepath.Dir(request.Target), "seed-tree")
	if err := os.Mkdir(tree, 0o700); err != nil {
		return fmt.Errorf("exclusively create NoCloud seed tree: %w", err)
	}
	for _, file := range request.Files {
		if err := durableExclusive(filepath.Join(tree, file.Name), file.Content, os.FileMode(file.Mode)); err != nil {
			return err
		}
	}
	if err := syncDirectory(tree); err != nil {
		return err
	}
	iso, err := buildNoCloudISO(request)
	if err != nil {
		return err
	}
	return durableExclusive(request.Target, iso, os.FileMode(request.Mode))
}

func (r *LinuxRunner) LaunchQEMU(ctx context.Context, request LaunchRequest) (result error) {
	if err := provisioning.ValidatePinnedBinary(request.Command.Binary, request.Command.BinarySHA256); err != nil {
		return err
	}
	if r.config.Execution.FirstBootObservation == nil {
		return fmt.Errorf("first-boot observation contract is required before QEMU launch")
	}
	for _, socket := range []string{request.QMP, request.AgentSocket} {
		if _, err := os.Lstat(socket); !os.IsNotExist(err) {
			return fmt.Errorf("QEMU socket path is not fresh: %s", socket)
		}
	}
	session, err := prepareObservationSession(*r.config.Execution.FirstBootObservation)
	if err != nil {
		return err
	}
	observationOwned := false
	defer func() {
		if !observationOwned {
			result = errors.Join(result, session.stopAndSync())
		}
	}()
	log, err := os.OpenFile(r.evidencePath("qemu.log"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(context.WithoutCancel(ctx), request.Command.Binary, request.Command.Args...)
	cmd.Env = []string{}
	cmd.Dir = filepath.Dir(request.Command.Binary)
	cmd.Stdout = log
	cmd.Stderr = log
	cmd.Stdin = nil
	if err := cmd.Start(); err != nil {
		log.Close()
		return err
	}
	exit := make(chan error, 1)
	go func() { exit <- cmd.Wait(); _ = log.Sync(); _ = log.Close() }()
	identity, err := processIdentity(cmd.Process.Pid, request.Command, filepath.Dir(r.config.Execution.OSClone.Target))
	if err != nil {
		return err
	}
	record, _ := json.Marshal(identity)
	if err := durableExclusive(request.ProcessRecord, record, 0o600); err != nil {
		return err
	}
	r.mu.Lock()
	r.command = cmd
	r.exit = exit
	r.process = identity
	r.mu.Unlock()
	type acceptResult struct {
		conn net.Conn
		err  error
	}
	accepted := make(chan acceptResult, 1)
	go func() {
		conn, acceptErr := session.listener.Accept()
		accepted <- acceptResult{conn: conn, err: acceptErr}
	}()
	consoleReady := false
	for {
		if readyPaths(request.QMP, request.AgentSocket) && consoleReady {
			if err := session.listener.Close(); err != nil {
				return err
			}
			session.listener = nil
			r.mu.Lock()
			if r.observation != nil {
				r.mu.Unlock()
				return fmt.Errorf("first-boot observation lifecycle is already active")
			}
			r.observation = session
			r.mu.Unlock()
			observationOwned = true
			return nil
		}
		select {
		case acceptedResult := <-accepted:
			if acceptedResult.err != nil {
				return fmt.Errorf("accept exact first-boot console transport: %w", acceptedResult.err)
			}
			session.conn = acceptedResult.conn
			consoleReady = true
		case err := <-exit:
			_ = session.listener.SetDeadline(time.Now())
			if !consoleReady {
				acceptedResult := <-accepted
				if acceptedResult.conn != nil {
					_ = acceptedResult.conn.Close()
				}
			}
			return fmt.Errorf("QEMU exited before its exact sockets were ready: %w", err)
		case <-ctx.Done():
			_ = session.listener.SetDeadline(time.Now())
			if !consoleReady {
				acceptedResult := <-accepted
				if acceptedResult.conn != nil {
					_ = acceptedResult.conn.Close()
				}
			}
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func (r *LinuxRunner) VerifyGuest(ctx context.Context, request GuestVerificationRequest) (result error) {
	progress := guestVerificationProgress{CompletedCapabilities: []string{}}
	if err := r.startObservationCapture(); err != nil {
		return errors.Join(err, r.finishObservation())
	}
	defer func() {
		if isGuestVerificationTimeout(result) {
			result = errors.Join(result, persistGuestVerificationTimeout(request, progress))
		}
		result = errors.Join(result, r.finishObservation())
	}()
	conn, err := r.config.Dial(ctx, "unix", request.Socket)
	if err != nil {
		return err
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	bootstrapJSON, err := protocol.ReadFramed(conn)
	if err != nil {
		return err
	}
	expected := protocol.Context{MachineUUID: r.config.Manifest.MachineUUID, DiskSerial: r.config.Manifest.DataDisk.Serial, RunID: r.config.Manifest.RunID, Scenario: r.config.Manifest.Scenario, DurabilityBoundary: r.config.Manifest.DurabilityBoundary}
	payload := protocol.IdentityBootstrapPayload{BootIDSource: r.config.Manifest.BootIDSource, ControllerPublicKeySHA256: r.config.Contract.Artifacts.ControllerPublicKeySHA256, GuestPublicKeySHA256: r.config.Contract.Artifacts.GuestPublicKeySHA256, ControllerBinarySHA256: r.config.Contract.Artifacts.ControllerBinarySHA256, GuestAgentBinarySHA256: r.config.Contract.Artifacts.GuestAgentBinarySHA256}
	bootstrap, err := protocol.VerifyIdentityBootstrap(bootstrapJSON, r.config.Keys.GuestPublic, r.config.Now(), expected, request.Bootstrap.BootstrapNonce, payload)
	if err != nil {
		return err
	}
	bootstrapEvidence, _ := json.Marshal(struct {
		Schema string          `json:"schema"`
		BootID string          `json:"boot_id"`
		Frame  json.RawMessage `json:"frame"`
	}{"dockpipe.vm.bootstrap-evidence.v1", bootstrap.Context.BootID, bootstrapJSON})
	if err := durableExclusive(request.Bootstrap.ExclusiveEvidencePath, bootstrapEvidence, 0o600); err != nil {
		return err
	}
	progress.BootstrapVerified = true
	results := make([]json.RawMessage, 0, len(request.Capabilities))
	nonces := map[string]bool{request.Bootstrap.BootstrapNonce: true}
	for i, capability := range request.Capabilities {
		nonce, err := freshNonce(nonces)
		if err != nil {
			return err
		}
		requestContext := bootstrap.Context
		requestContext.Sequence = request.FirstRequestSequence + uint64(i)
		requestContext.Nonce = nonce
		requestContext.Phase = "verification"
		body := any(struct{}{})
		if capability == "launch-hash-pinned/v1" {
			body = map[string]string{"controller_binary_sha256": r.config.Contract.Artifacts.ControllerBinarySHA256, "guest_agent_binary_sha256": r.config.Contract.Artifacts.GuestAgentBinarySHA256}
		}
		now := r.config.Now()
		signed, err := protocol.Sign(protocol.RequestKind, capability, requestContext, body, now, now.Add(time.Minute), r.config.Keys.ControllerPrivate)
		if err != nil {
			return err
		}
		if err := protocol.WriteFramed(conn, signed); err != nil {
			return err
		}
		responseJSON, err := protocol.ReadFramed(conn)
		if err != nil {
			return err
		}
		response, err := protocol.Verify(responseJSON, r.config.Keys.GuestPublic, r.config.Now())
		if err != nil {
			return err
		}
		if response.Kind != protocol.ResultKind || response.Capability != capability || response.Context != requestContext {
			return fmt.Errorf("guest result did not echo the complete authenticated request context")
		}
		if err := validateResult(response, r.config.Contract.Artifacts.ControllerBinarySHA256, r.config.Contract.Artifacts.GuestAgentBinarySHA256); err != nil {
			return err
		}
		results = append(results, responseJSON)
		progress.CompletedCapabilities = append(progress.CompletedCapabilities, capability)
	}
	evidence, _ := json.Marshal(struct {
		Schema  string            `json:"schema"`
		BootID  string            `json:"boot_id"`
		Results []json.RawMessage `json:"results"`
	}{"dockpipe.vm.verification-evidence.v1", bootstrap.Context.BootID, results})
	return durableExclusive(request.Evidence, evidence, 0o600)
}

func isGuestVerificationTimeout(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var timeout net.Error
	return errors.As(err, &timeout) && timeout.Timeout()
}

func persistGuestVerificationTimeout(request GuestVerificationRequest, progress guestVerificationProgress) error {
	evidence, err := json.Marshal(guestVerificationFailureEvidence{
		Schema: guestVerificationFailureSchema, Operation: "verify-guest", Reason: "timeout",
		TimeoutSeconds: request.TimeoutSeconds, BootstrapVerified: progress.BootstrapVerified,
		CompletedCapabilities: append([]string{}, progress.CompletedCapabilities...),
	})
	if err != nil {
		return fmt.Errorf("encode guest-verification timeout evidence: %w", err)
	}
	if err := durableExclusive(request.FailureEvidence, evidence, 0o600); err != nil {
		return fmt.Errorf("persist guest-verification timeout evidence: %w", err)
	}
	return nil
}

func (r *LinuxRunner) ControlledShutdown(ctx context.Context, request ShutdownRequest) error {
	if err := r.finishObservation(); err != nil {
		return err
	}
	if err := r.requireOwnedProcess(request.ProcessRecord); err != nil {
		return err
	}
	conn, err := r.config.Dial(ctx, "unix", request.QMP)
	if err != nil {
		return err
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	client := controller.QMPClient{Conn: conn}
	if err := client.Negotiate("capabilities-1"); err != nil {
		return err
	}
	if err := client.SystemPowerdown("powerdown-1"); err != nil {
		return err
	}
	r.mu.Lock()
	exit := r.exit
	r.mu.Unlock()
	if exit == nil {
		return fmt.Errorf("owned QEMU child is unavailable")
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-exit:
		if err != nil {
			return fmt.Errorf("QEMU did not exit cleanly after system_powerdown: %w", err)
		}
	}
	evidence, _ := json.Marshal(map[string]any{"schema": "dockpipe.vm.shutdown-evidence.v1", "command": request.Command, "pid": r.process.PID, "clean_exit": true})
	return durableExclusive(request.Evidence, evidence, 0o600)
}

func (r *LinuxRunner) PreserveFailure(_ context.Context, request PreservationRequest) error {
	result := r.finishObservation()
	for _, root := range request.Roots {
		if err := syncTree(root); err != nil && !os.IsNotExist(err) {
			result = errors.Join(result, err)
		}
	}
	return result
}

func (r *LinuxRunner) startObservationCapture() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.observation == nil {
		return fmt.Errorf("first-boot observation transport was not established by launch")
	}
	return r.observation.startCapture()
}

func (r *LinuxRunner) finishObservation() error {
	r.mu.Lock()
	session := r.observation
	r.observation = nil
	r.mu.Unlock()
	return session.stopAndSync()
}

func (r *LinuxRunner) Cleanup(_ context.Context, request CleanupRequest) error {
	if len(request.Resources) == 0 {
		return fmt.Errorf("exact cleanup resources are required")
	}
	processRecord := request.Resources[4]
	if active, err := recordedProcessActive(processRecord); err != nil {
		return err
	} else if active {
		return fmt.Errorf("exact cleanup refuses an active QEMU process")
	}
	for i, path := range request.Resources {
		if !filepath.IsAbs(path) {
			return fmt.Errorf("cleanup resource %d is not absolute", i)
		}
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("remove exact cleanup resource %s: %w", path, err)
		}
	}
	return nil
}

func runExact(ctx context.Context, command provisioning.PinnedCommand, dir, logPath string) error {
	log, err := os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, command.Binary, command.Args...)
	cmd.Env = []string{}
	cmd.Dir = dir
	cmd.Stdout = log
	cmd.Stderr = log
	cmd.Stdin = nil
	err = cmd.Run()
	syncErr := log.Sync()
	closeErr := log.Close()
	if err != nil {
		return err
	}
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}

func (r *LinuxRunner) evidencePath(name string) string {
	return filepath.Join(filepath.Dir(r.config.Execution.Guest.Evidence), name)
}
func (r *LinuxRunner) requireOwnedProcess(path string) error {
	var observed controller.ProcessIdentity
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(b, &observed); err != nil {
		return err
	}
	current, err := processIdentity(observed.PID, r.config.Execution.Launch.Command, observed.InstanceRoot)
	if err != nil {
		return err
	}
	if current != r.process || observed != r.process {
		return fmt.Errorf("QEMU process identity changed")
	}
	return nil
}

func processIdentity(pid int, command provisioning.PinnedCommand, instanceRoot string) (controller.ProcessIdentity, error) {
	stat, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return controller.ProcessIdentity{}, err
	}
	closeParen := bytes.LastIndexByte(stat, ')')
	if closeParen < 0 {
		return controller.ProcessIdentity{}, fmt.Errorf("invalid process stat")
	}
	fields := strings.Fields(string(stat[closeParen+1:]))
	if len(fields) < 20 {
		return controller.ProcessIdentity{}, fmt.Errorf("incomplete process stat")
	}
	start, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil {
		return controller.ProcessIdentity{}, err
	}
	material, _ := json.Marshal(struct {
		Binary string   `json:"binary"`
		Args   []string `json:"args"`
	}{command.Binary, command.Args})
	sum := sha256.Sum256(material)
	return controller.ProcessIdentity{PID: pid, UID: os.Geteuid(), StartTicks: start, ExecutableSHA: command.BinarySHA256, CommandSHA: hex.EncodeToString(sum[:]), InstanceRoot: instanceRoot}, nil
}

func recordedProcessActive(path string) (bool, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var p controller.ProcessIdentity
	if json.Unmarshal(b, &p) != nil || p.PID <= 1 || p.UID != os.Geteuid() || p.StartTicks == 0 || len(p.ExecutableSHA) != 64 || len(p.CommandSHA) != 64 || !filepath.IsAbs(p.InstanceRoot) {
		return false, fmt.Errorf("invalid process record")
	}
	stat, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", p.PID))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	closeParen := bytes.LastIndexByte(stat, ')')
	if closeParen < 0 {
		return false, fmt.Errorf("invalid live process stat")
	}
	fields := strings.Fields(string(stat[closeParen+1:]))
	if len(fields) < 20 {
		return false, fmt.Errorf("invalid live process stat")
	}
	start, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil {
		return false, fmt.Errorf("invalid live process start identity")
	}
	return start == p.StartTicks, nil
}

func validateResult(frame protocol.SignedFrame, controllerSHA256, guestSHA256 string) error {
	switch frame.Capability {
	case "identity/v1":
		var payload struct {
			MachineUUID string `json:"machine_uuid"`
			DiskSerial  string `json:"disk_serial"`
			BootID      string `json:"boot_id"`
		}
		if err := decodeExactPayload(frame.Payload, &payload); err != nil {
			return err
		}
		if payload.MachineUUID != frame.Context.MachineUUID || payload.DiskSerial != frame.Context.DiskSerial || payload.BootID != frame.Context.BootID {
			return fmt.Errorf("identity result mismatch")
		}
	case "health/v1":
		var payload struct {
			Healthy bool `json:"healthy"`
		}
		if err := decodeExactPayload(frame.Payload, &payload); err != nil {
			return err
		}
		if !payload.Healthy {
			return fmt.Errorf("health result mismatch")
		}
	case "launch-hash-pinned/v1":
		var payload struct {
			ControllerBinarySHA256 string `json:"controller_binary_sha256"`
			GuestAgentBinarySHA256 string `json:"guest_agent_binary_sha256"`
			Matched                bool   `json:"matched"`
		}
		if err := decodeExactPayload(frame.Payload, &payload); err != nil {
			return err
		}
		if !payload.Matched || payload.ControllerBinarySHA256 != controllerSHA256 || payload.GuestAgentBinarySHA256 != guestSHA256 {
			return fmt.Errorf("launch-pin result mismatch")
		}
	default:
		return fmt.Errorf("unexpected verification capability")
	}
	return nil
}

func decodeExactPayload(data []byte, out any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return fmt.Errorf("result payload contains trailing JSON")
	}
	return nil
}

func freshNonce(used map[string]bool) (string, error) {
	for {
		b := make([]byte, 32)
		if _, err := io.ReadFull(rand.Reader, b); err != nil {
			return "", err
		}
		n := hex.EncodeToString(b)
		if !used[n] {
			used[n] = true
			return n, nil
		}
	}
}
func readyPaths(paths ...string) bool {
	for _, path := range paths {
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSocket == 0 {
			return false
		}
	}
	return true
}
func durableExclusive(path string, data []byte, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(path))
}
func syncFileAndDir(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	f.Close()
	return syncDirectory(filepath.Dir(path))
}
func syncDirectory(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}
func syncTree(root string) error {
	directories := []string{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			f, e := os.Open(path)
			if e != nil {
				return e
			}
			e = f.Sync()
			f.Close()
			return e
		}
		if info.IsDir() {
			directories = append(directories, path)
		}
		return nil
	})
	if err != nil {
		return err
	}
	for i := len(directories) - 1; i >= 0; i-- {
		if err := syncDirectory(directories[i]); err != nil {
			return err
		}
	}
	return nil
}
func hashBytes(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }

var _ Runner = (*LinuxRunner)(nil)
